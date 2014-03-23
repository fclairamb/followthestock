package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
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
		}
		time.Sleep(time.Minute)
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
	stocks map[string]StockFollower
}

func httpGet(url string) (*http.Response, error) {
	log.Println("Fetching ", url, "...")
	r, e := http.Get(url)
	log.Println("Fetched ", url)
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

	body, err := fetchBoursoramaPageFomSymbol(s.GetBoursoramaSymbol())

	if err != nil {
		return nil, errors.New(fmt.Sprintf("No \"%s\" on %s market !", short, market))
	}

	{ // Name
		// http://play.golang.org/p/TpmUsGxvtQ
		re, err := regexp.Compile("(?s)\\<h1\\>.*title=\"([^\\\"]+)\".*\\</h1\\>")
		if err != nil {
			log.Fatal(err)
		}
		result := re.FindStringSubmatch(body)
		if len(result) < 2 {
			return nil, errors.New(fmt.Sprintf("Could not find name in the %s page !", s.GetBoursoramaSymbol()))
		}
		s.Name = result[1]
	}

	return s, nil
}

func (s *Stock) GetBoursoramaSymbol() (symbol string) {
	if s.Market == "US" {
		symbol = s.Short
	} else if s.Market == "FR" {
		symbol = "1rP" + s.Short
	} else {
		log.Fatal("Unknown market: " + s.Market)
	}
	return
}

func (s *Stock) GetValue() (value float32, err error) {
	var body string
	{ // We get the page's content
		resp, err := httpGet(fmt.Sprintf("http://www.boursorama.com/cours.phtml?symbole=%s", s.GetBoursoramaSymbol()))
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
		{
			v, _ := strconv.ParseFloat(strings.Replace(result[1], " ", "", -1), 32)
			value = float32(v)
		}
	}

	return value, nil
}

func NewStocksMgmt() *StocksMgmt {
	sm := &StocksMgmt{stocks: make(map[string]StockFollower)}

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

	if s == nil {
		e = errors.New("Could not find " + short)
	} else {
		e = nil
	}

	return
}

func (sm *StocksMgmt) LoadStocks() (err error) {
	stocks := db.GetAllStocks()

	for _, s := range *stocks {
		log.Println("Loading", s.String(), "...")
		stock := s // Not doing so make us share the same pointer
		sf := NewStockFollower(&stock)
		sf.Start()
	}

	return nil
}

func (sm *StocksMgmt) Start() {
	sm.LoadStocks()
}

func (sm *StocksMgmt) Close() {

}
