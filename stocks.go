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

func NewStockFollower(s *Stock) *StockFollower {
	return &StockFollower{Stock: s}
}

func (sf *StockFollower) run() {
	for {
		v, err := sf.Stock.GetValue()
		if err != nil {
			log.Println("Stock", sf.Stock, "error", err)
		} else {
			log.Println("Stock", sf.Stock, "=", v)
			sf.considerValue(v)
		}
		time.Sleep(time.Minute)
	}
}

func (sf *StockFollower) considerValue(value float32) {
	db.SaveStockValue(sf.Stock, value)
	for _, al := range *db.GetAlerts(sf.Stock) {
		if al.LastValue == 0 {
			al.LastValue = value
			db.SaveAlert(&al)
			continue
		}

		diff := value - al.LastValue
		per := diff / al.LastValue * 100
		varPer := float32(math.Abs(float64(per)))
		log.Println("Alert", al.Id, sf.Stock, ":", varPer, "%")
		if varPer >= al.Percent {
			log.Println("Alert", al.Id, "Trigger !")
			contact := db.GetContactFromId(al.Contact)
			if contact == nil {
				log.Println("Contact missing, deleting alert !")
				db.DeleteAlert(&al)
				continue
			}
			al.LastValue = value
			timeDiff := time.Now().UTC().UnixNano() - al.LastTriggered
			al.LastTriggered = time.Now().UTC().UnixNano()
			message := fmt.Sprintf("%s : %f (%d)", sf.Stock.String(), varPer, timeDiff/time.Minute.Nanoseconds())
			db.SaveAlert(&al)
			sc := &SendChat{Remote: contact.Email, Text: message}
			log.Println("Sending ", sc)
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
		reName, err := regexp.Compile("(?s)<h1>.*title=\"([^\\\"]+)\".*</h1>")
		if err != nil {
			log.Fatal(err)
		}
		result := reName.FindStringSubmatch(body)
		if len(result) > 1 {
			s.Name = result[1]
		}
	}

	if len(s.Name) == 0 { // Second attempt for other quotations
		reName, err := regexp.Compile("(?s)<h1>.*<a.*>(.*)</a>.*</h1>")
		if err != nil {
			log.Fatal(err)
		}
		result := reName.FindStringSubmatch(body)
		if len(result) > 1 {
			s.Name = strings.Trim(result[1], " \n\r")
		}
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
		defer resp.Body.Close()
		if err != nil {
			return -1, err
		}
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
		re, err := regexp.Compile("<span class=\"cotation\">([0-9\\ \\.]+)[^\\<\\>]*(EUR|USD)</span>")
		if err != nil {
			log.Fatal(err)
		}
		result := re.FindStringSubmatch(body)
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

func (sm *StocksMgmt) GetStock(short string) (s *Stock, e error) {
	short = strings.ToUpper(short)
	market := "FR"
	precise := false
	tokens := strings.SplitN(short, ":", 2)
	if len(tokens) == 2 {
		precise = true
		market = tokens[0]
		short = tokens[1]
	}

	s = db.GetStock(market, short)

	if s == nil {
		s, e = tryNewStock(market, short)
		if e == nil {
			s.Save()
		}
	}

	if !precise && s == nil {
		market = "US"
		s = db.GetStock(market, short)

		if s == nil {
			s, e = tryNewStock(market, short)
			if e == nil {
				s.Save()
			}
		} else {
			log.Printf("Stock: %v", s)
		}
	}

	if !precise && s == nil {
		market = "W"
		s = db.GetStock(market, short)

		if s == nil {
			s, e = tryNewStock(market, short)
			if e == nil {
				s.Save()
			}
		} else {
			log.Printf("Stock: %v", s)
		}
	}

	if s != nil {
		e = nil
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

	if _, ok := sm.stocks[s.String()]; !ok {
		sm.LoadStock(s)
	}

	return a, e
}

func (sm *StocksMgmt) Start() {
	sm.Lock()
	sm.LoadStocks()
	sm.Unlock()
}

func (sm *StocksMgmt) Close() {

}
