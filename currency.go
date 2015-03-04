package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const CURRENCY_EXPIRATION = int64(time.Minute) * 15

var reRate = regexp.MustCompile("<span class=\"cotation\">([0-9\\ \\.]+)[^<>]*[A-Z]{2,3}</span>")

func fetchCurrencyRate(from, to string) (float32, error) {
	resp, err := httpGet(fmt.Sprintf("http://www.boursorama.com/taux-de-change-x-%s-%s", from, to))
	defer resp.Body.Close()
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != 200 {
		return 0, errors.New(fmt.Sprintf("Wrong status code %d", resp.StatusCode))
	}

	finalUrl := resp.Request.URL.String()

	if strings.Contains(finalUrl, "recherche") {
		return 0, errors.New(fmt.Sprintf("Not found !"))
	}

	var body string
	{ // We get the body
		raw, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		body = string(raw)
	}

	{ // And the value
		result := reRate.FindStringSubmatch(body)
		if len(result) >= 2 {
			v, err := strconv.ParseFloat(strings.Replace(result[1], " ", "", -1), 32)
			if err != nil {
				return 0, err
			}
			return float32(v), nil
		} else {
			return 0, errors.New(fmt.Sprintf("Could not fetch rate for %s/%s", from, to))
		}
	}
}

func CurrencyRate(from, to string) float32 {
	cur := db.GetCurrencyConversion(from, to)

	if cur == nil {
		return 0
	}

	now := time.Now().UTC().UnixNano()

	if now-cur.LastUpdate > CURRENCY_EXPIRATION {
		var err error
		if cur.Rate, err = fetchCurrencyRate(from, to); err == nil {
			cur.LastUpdate = now
			db.SaveCurrencyConversion(cur)
		} else {
			return 0
		}
	}

	return cur.Rate
}
