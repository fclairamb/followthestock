package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type StockFollower struct {
	Stock *Stock
}

var (
	reCotation *regexp.Regexp
	reName     []*regexp.Regexp
	sleepTime  time.Duration = time.Minute
)

const TEMPDIR = "/tmp/followthestock/"

func init() {
	reCotation = regexp.MustCompile("<span class=\"cotation\">([0-9\\ \\.]+)[^A-Z<>]*([A-Z]{2,3})</span>")

	reName = []*regexp.Regexp{
		regexp.MustCompile("(?s)<[^>]* itemprop=\"name\" title=\"([^\\\"]+)\"[^>]*>"),
		regexp.MustCompile("(?s)<h1>.*<a.*>(.*)</a>.*</h1>"),
	}
}

func NewStockFollower(s *Stock) *StockFollower {
	return &StockFollower{Stock: s}
}

func (sf *StockFollower) run() {
	t := time.Now().UTC() //.UnixNano()
	for {
		v, _, err := sf.Stock.GetValue()
		if err != nil {
			log.Warning("Stock %s: %v", sf.Stock.String(), err)
		} else {
			log.Info("Stock %s = %f %s", sf.Stock, v, sf.Stock.Currency)
			sf.considerValue(v)
		}
		if config.General.ExactTiming {
			t = t.Add(sleepTime) //.Nanoseconds()
			sl := t.Sub(time.Now().UTC())
			time.Sleep(sl)
		} else {
			time.Sleep(sleepTime)
		}
	}
}

func (sf *StockFollower) considerValue(value float32) {

	now := time.Now().UTC().UnixNano()

	if value == 0 {
		log.Warning("We have to ignore zero value for stock %v.", sf.Stock)
		return
	}

	db.SaveStockValue(sf.Stock, value, now)
	for _, al := range *db.GetAlertsForStock(sf.Stock) {
		if al.LastValue == 0 {
			value = value * 0.5
			al.LastValue = value
			al.LastTriggered = now
			db.SaveAlert(&al)

			contact := db.GetContactFromId(al.Contact)
			if contact == nil {
				log.Info("Alert %d - Contact missing, deleting alert !", al.Id)
				db.DeleteAlert(&al)
			}

			continue
		}

		diff := value - al.LastValue
		per := diff / al.LastValue * 100
		varPer := float32(math.Abs(float64(per)))
		log.Info("Alert %s", al.String())

		var triggered bool
		switch al.PercentDirection {
		case ALERT_DIRECTION_BOTH:
			triggered = (varPer >= al.Percent)
		case ALERT_DIRECTION_UP:
			triggered = (per > al.Percent)
		case ALERT_DIRECTION_DOWN:
			triggered = (per < al.Percent)
		}

		if triggered {
			contact := db.GetContactFromId(al.Contact)
			if contact == nil {
				log.Info("Alert %d - Contact missing, deleting alert !", al.Id)
				db.DeleteAlert(&al)
				continue
			}
			if now < contact.PauseUntil {
				log.Info("Alert %d - Contact is in pause", al.Id)
				continue
			}

			log.Info("Alert %d - Trigger !", al.Id)
			al.LastValue = value
			timeDiff := time.Duration(now - al.LastTriggered)
			timeDiff -= timeDiff % time.Second
			al.LastTriggered = now
			al.LastDate = now
			message := fmt.Sprintf("%s : %.3f (%+.2f%%) in %v", sf.Stock.String(), value, per, timeDiff)

			if contact.ShowUrl {
				message += " / " + sf.Stock.Url()
			}

			db.SaveAlert(&al)

			// We might be able to give some valuation data
			if csv := db.GetContactStockValue(al.Contact, al.Stock); csv.Exists() {
				cost := float32(csv.Nb) * csv.Value
				value := float32(csv.Nb) * value
				diff := value - cost
				per := diff / cost * 100
				message += fmt.Sprintf(" / %.3f - %.3f = %+.3f (%+.2f%%)", value, cost, diff, per)
			}

			xm.Send <- &SendChat{Remote: contact.Email, Text: message}
		} else {
			// If we have a duration, we might have to push the LastDate in the future
			if al.Duration != 0 && now-al.LastDate > al.Duration {
				startOfTimeWindow := now - al.Duration
				if value, err := db.GetStockValue(sf.Stock, startOfTimeWindow); err == nil {
					al.LastDate = startOfTimeWindow
					al.LastValue = value.Value
					db.SaveAlert(&al)
					log.Info("Alert %s: lastDate = %v, lastValue = %v", al.String(), time.Unix(0, al.LastDate), al.LastValue)
				} else {
					log.Error("Cannot find rows for alert %v: %v", sf.Stock, err)
				}
			}
		}
	}
}

func (sf *StockFollower) Start() {
	go sf.run()
}

func (sf *StockFollower) Stop() {

}

func (sf *StockFollower) String() string {
	return sf.Stock.String()
}

type StocksMgmt struct {
	sync.RWMutex
	stocks map[string]*StockFollower
}

func httpGet(url string) (*http.Response, error) {
	log.Debug("Fetching \"%s\"...", url)
	r, e := http.Get(url)
	//log.Println("Fetched ", url)
	return r, e
}

func (this *Stock) PageContent() (body string, err error) {
	resp, err := httpGet(this.Url())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("Wrong status code %d", resp.StatusCode))
	}

	finalUrl := resp.Request.URL.String()

	if strings.Contains(finalUrl, "recherche") {
		return "", errors.New(fmt.Sprintf("Not found !"))
	}

	{ // We get the body
		raw, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		body = string(raw)
	}

	return
}

func tryNewStock(market, short string) (*Stock, error) {
	log.Debug("tryNewStock( \"%s\", \"%s\" );", market, short)
	s := &Stock{Market: market, Short: short}

	body, err := s.PageContent()

	if err != nil {
		return nil, errors.New(fmt.Sprintf("No \"%s\" on %s market !", short, market))
	}

	for _, re := range reName { // Second attempt for other quotations
		result := re.FindStringSubmatch(body)
		if len(result) > 1 {
			s.Name = strings.Trim(result[1], " \n\r")
			break
		} else {
			log.Error("Regex failed: %s", re)
		}
	}

	if len(s.Name) == 0 { // If we still couldn't get a name
		// We will save the raw data for future testing
		os.MkdirAll(TEMPDIR, 0755)
		fileName := fmt.Sprintf("%s/%s_%s.html", TEMPDIR, s.Market, s.Short)
		if err := ioutil.WriteFile(fileName, []byte(body), 0644); err != nil {
			log.Error("Could not write %s: %s", fileName, err)
		}
		return s, errors.New("Could not get the name")
	}

	return s, nil
}

var marketsToTest = [...]string{"FR", "AM", "US", "US2", "W", "BE"}

func (s *Stock) boursoramaSymbol() (symbol string) {
	switch s.Market {
	case "US": // NASDAQ & NYSE
		symbol = s.Short
	case "US2": // XETRA ?
		symbol = "1z" + s.Short
	case "FR": // EURONEXT Paris
		symbol = "1rP" + s.Short
	case "AM": // EURONEXT Amsterdam
		symbol = "1rA" + s.Short
	case "W": // Warrants
		symbol = "2rP" + s.Short
	case "W2":
		symbol = "3rP" + s.Short
	case "BE": // EURONEXT Bruxelles
		symbol = "FF11-" + s.Short
	default:
		log.Fatal("Unknown market: " + s.Market)
	}
	return
}

func (this *Stock) Url() string {
	return fmt.Sprintf("http://www.boursorama.com/cours.phtml?symbole=%s", this.boursoramaSymbol())
}

func (s *Stock) fetchPage() (string, error) {
	resp, err := httpGet(fmt.Sprintf("http://www.boursorama.com/cours.phtml?symbole=%s", s.boursoramaSymbol()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("Wrong status code %d", resp.StatusCode))
	}

	finalUrl := resp.Request.URL.String()

	if strings.Contains(finalUrl, "recherche") {
		return "", errors.New(fmt.Sprintf("Not found !"))
	}

	{
		raw, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		body := string(raw)
		return body, nil
	}
}

func (s *Stock) GetValue() (value float32, currency string, err error) {
	body, err := s.fetchPage()

	save := false

	result := reCotation.FindStringSubmatch(body)
	if len(result) >= 2 {
		v, _ := strconv.ParseFloat(strings.Replace(result[1], " ", "", -1), 32)
		value = float32(v)
		currency = result[2]
		if s.FailedFetches != 0 {
			s.FailedFetches = 0
			log.Debug("Updating %v's failed fetches", s)
			save = true
		}
		if s.Currency == "" && currency != "" {
			s.Currency = currency
			log.Info("Updating %v's currency", s)
			save = true
		}
	} else {
		s.FailedFetches += 1
		log.Warning("Could not fetch cotation %s for the %dth time.", s, s.FailedFetches)
		if s.FailedFetches > 1000 {
			log.Info("Deleting stock %#v ...", s)
			db.DeleteStock(s)
		} else {
			save = true
		}

	}

	if s.Name == "" || s.Currency == "" { // We get the name if we couldn't get it earlier
		log.Warning("Missing name or currency, trying to update data about %v", s)
		if s2, err := tryNewStock(s.Market, s.Short); err == nil {
			if s.Name == "" && s2.Name != "" {
				log.Info("Updating %v's name", s)
				s.Name = s2.Name
				save = true
			}
		}
	}

	if save {
		log.Info("Updating stock %#v ...", s)
		db.SaveStock(s)
	}

	return
}

func NewStocksMgmt() *StocksMgmt {
	sm := &StocksMgmt{stocks: make(map[string]*StockFollower)}

	return sm
}

func (sm *StocksMgmt) getOrCreateStock(market, short string) (s *Stock, e error) {
	s = db.GetStock(market, short)
	if s == nil { // If we couldn't get it
		s, e = tryNewStock(market, short) // We try to get it
		if s != nil {
			s.Value, s.Currency, e = s.GetValue() // And we get the value
			db.SaveStock(s)
		}
	} else if s.Currency == "" {
		_, s.Currency, _ = s.GetValue()
		db.SaveStock(s)
	}
	return
}

func (sm *StocksMgmt) GetStock(short string) (s *Stock, e error) {
	short = strings.ToUpper(short)
	tokens := strings.SplitN(short, ":", 2)

	if len(tokens) == 2 { // Specific market stock
		market := tokens[0]
		short = tokens[1]
		s, e = sm.getOrCreateStock(market, short)
	} else { // Unspecified market stock
		for _, market := range marketsToTest { // We test all stocks
			s, e = sm.getOrCreateStock(market, short)
			if s != nil {
				break
			}
		}
	}

	return
}

func (sm *StocksMgmt) LoadStock(s *Stock) {
	sf := NewStockFollower(s)
	sf.Start()
	sm.stocks[s.String()] = sf
}

func (sm *StocksMgmt) LoadStocks() (err error) {
	stocks := db.GetAllStocks()

	for _, s := range *stocks {
		log.Info("Loading %s...", s.String())
		stock := s // Not doing so make us share the same pointer
		sm.LoadStock(&stock)
	}

	return nil
}

func (sm *StocksMgmt) SubscribeAlert(s *Stock, c *Contact, per float32, direction int, duration int64) (alert *Alert, err error) {
	a, e := db.SubscribeAlert(s, c, per, direction, duration)

	sm.Lock()
	if _, ok := sm.stocks[s.String()]; !ok {
		sm.LoadStock(s)
	}
	sm.Unlock()

	return a, e
}

func (sm *StocksMgmt) UnsubscribeAlert(s *Stock, c *Contact) (err error) {
	_, err = db.UnsubscribeAlert(s, c)

	return
}

func (sm *StocksMgmt) Start() {
	sm.Lock()
	sm.LoadStocks()
	sm.Unlock()
}

func (sm *StocksMgmt) Stop() {
	log.Debug("StocksMgmt.Stop()")
}
