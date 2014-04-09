package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
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
	reName1, reName2, reCotation *regexp.Regexp
	sleepTime                    time.Duration
)

func init() {
	var err error
	reName1, err = regexp.Compile("(?s)<h1>.*title=\"([^\\\"]+)\".*</h1>")
	if err != nil {
		log.Fatal(err)
	}
	reName2, err = regexp.Compile("(?s)<h1>.*<a.*>(.*)</a>.*</h1>")
	if err != nil {
		log.Fatal(err)
	}
	reCotation, err = regexp.Compile("<span class=\"cotation\">([0-9\\ \\.]+)[^<>]*[A-Z]{2,3}</span>")
	if err != nil {
		log.Fatal(err)
	}

	sleepTime = time.Minute
}

func NewStockFollower(s *Stock) *StockFollower {
	return &StockFollower{Stock: s}
}

func (sf *StockFollower) run() {
	t := time.Now().UTC() //.UnixNano()
	for {
		v, err := sf.Stock.GetValue()
		if err != nil {
			log.Println("Stock", sf.Stock, "error", err)
		} else {
			log.Println("Stock", sf.Stock, "=", v)
			sf.considerValue(v)
		}
		if par.exact {
			t = t.Add(sleepTime) //.Nanoseconds()
			sl := t.Sub(time.Now().UTC())
			time.Sleep(sl)
		} else {
			time.Sleep(sleepTime)
		}
	}
}

func (sf *StockFollower) considerValue(value float32) {
	if value == 0 {
		log.Println("WARNING: We have to ignore zero value for stock %v.", sf.Stock)
		return
	}

	db.SaveStockValue(sf.Stock, value)
	for _, al := range *db.GetAlertsForStock(sf.Stock) {
		if al.LastValue == 0 {
			value = value * 0.5
			al.LastValue = value
			al.LastTriggered = time.Now().UTC().UnixNano()
			db.SaveAlert(&al)

			contact := db.GetContactFromId(al.Contact)
			if contact == nil {
				log.Println("Alert", al.Id, "- Contact missing, deleting alert !")
				db.DeleteAlert(&al)
			}

			continue
		}

		diff := value - al.LastValue
		per := diff / al.LastValue * 100
		varPer := float32(math.Abs(float64(per)))
		log.Println("Alert", al.Id, "-", sf.Stock, ":", varPer, "%")
		if varPer >= al.Percent {
			contact := db.GetContactFromId(al.Contact)
			if contact == nil {
				log.Println("Alert", al.Id, "- Contact missing, deleting alert !")
				db.DeleteAlert(&al)
				continue
			}
			if time.Now().UTC().UnixNano() < contact.PauseUntil {
				log.Println("Alert", al.Id, "- Contact is in pause")
				continue
			}

			log.Println("Alert", al.Id, "- Trigger !")
			al.LastValue = value
			timeDiff := time.Duration(time.Now().UTC().UnixNano() - al.LastTriggered)
			timeDiff -= timeDiff % time.Second
			al.LastTriggered = time.Now().UTC().UnixNano()
			var plus string
			if per > 0 {
				plus = "+"
			} else {
				plus = ""
			}
			message := fmt.Sprintf("%s : %.3f (%s%.2f%%) in %v", sf.Stock.String(), value, plus, per, timeDiff)
			db.SaveAlert(&al)

			// We might be able to give some valuation data
			if csv := db.GetContactStockValue(al.Contact, al.Stock); csv.Exists() {
				cost := float32(csv.Nb) * csv.Value
				value := float32(csv.Nb) * value
				diff := value - cost
				per := diff / cost * 100
				if per > 0 {
					plus = "+"
				} else {
					plus = ""
				}

				message += fmt.Sprintf(" / %.3f - %.3f = %s%.3f (%s%.2f%%)", value, cost, plus, diff, plus, per)
			}

			xm.Send <- &SendChat{Remote: contact.Email, Text: message}
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
	log.Println("Fetching", url, "...")
	r, e := http.Get(url)
	//log.Println("Fetched ", url)
	return r, e
}

func fetchBoursoramaPageFomSymbol(symbol string) (body string, err error) {
	resp, err := httpGet(fmt.Sprintf("http://www.boursorama.com/cours.phtml?symbole=%s", symbol))
	defer resp.Body.Close()
	if err != nil {
		return "", err
	}
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
	log.Println(fmt.Sprintf("tryNewStock( \"%s\", \"%s\" );", market, short))
	s := &Stock{Market: market, Short: short}

	body, err := fetchBoursoramaPageFomSymbol(s.getBoursoramaSymbol())

	if err != nil {
		return nil, errors.New(fmt.Sprintf("No \"%s\" on %s market !", short, market))
	}

	{ // First attempt for standard quotations
		result := reName1.FindStringSubmatch(body)
		if len(result) > 1 {
			s.Name = result[1]
		}
	}

	if len(s.Name) == 0 { // Second attempt for other quotations
		result := reName2.FindStringSubmatch(body)
		if len(result) > 1 {
			s.Name = strings.Trim(result[1], " \n\r")
		}
	}

	if len(s.Name) == 0 { // If we still couldn't get a name
		return s, errors.New("Could not get the name")
	}

	return s, nil
}

func (s *Stock) getBoursoramaSymbol() (symbol string) {
	if s.Market == "US" {
		symbol = s.Short
	} else if s.Market == "FR" {
		symbol = "1rP" + s.Short
	} else if s.Market == "W" {
		symbol = "2rP" + s.Short
	} else {
		log.Fatal("Unknown market: " + s.Market)
	}
	return
}

func (s *Stock) GetValue() (value float32, err error) {
	var body string
	{ // We get the page's content
		resp, err := httpGet(fmt.Sprintf("http://www.boursorama.com/cours.phtml?symbole=%s", s.getBoursoramaSymbol()))
		if err != nil {
			return -1, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return -1, errors.New(fmt.Sprintf("Wrong status code %d", resp.StatusCode))
		}

		finalUrl := resp.Request.URL.String()

		if strings.Contains(finalUrl, "recherche") {
			return -1, errors.New(fmt.Sprintf("Not found !"))
		}

		{
			raw, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return -1, err
			}
			body = string(raw)
		}
	}

	{ // Value
		result := reCotation.FindStringSubmatch(body)
		if len(result) >= 2 {
			v, _ := strconv.ParseFloat(strings.Replace(result[1], " ", "", -1), 32)
			value = float32(v)
		} else {
			log.Println("Could not fetch cotation for", s.String())
		}
	}

	return value, nil
}

func NewStocksMgmt() *StocksMgmt {
	sm := &StocksMgmt{stocks: make(map[string]*StockFollower)}

	return sm
}

var marketsToTest = [...]string{"FR", "US", "W"}

func (sm *StocksMgmt) getOrCreateStock(market, short string) (s *Stock, e error) {
	s = db.GetStock(market, short)
	if s == nil { // If we couldn't get it
		s, e = tryNewStock(market, short) // We try to get it
		if s != nil {
			s.Value, e = s.GetValue() // And we get the value
			db.SaveStock(s)
		}
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
		log.Println("Loading", s.String(), "...")
		stock := s // Not doing so make us share the same pointer
		sm.LoadStock(&stock)
	}

	return nil
}

func (sm *StocksMgmt) SubscribeAlert(s *Stock, c *Contact, per float32) (alert *Alert, err error) {
	a, e := db.SubscribeAlert(s, c, per)

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
	log.Println("StocksMgmt.Stop()")
}
