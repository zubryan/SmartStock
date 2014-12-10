package main

import (
	"encoding/json"
	"flag"
	"github.com/larspensjo/config"
	"io/ioutil"
	"net/http"
	"os"
)

var (
	configFile = flag.String("configfile", "config.ini", "General configuration file")
)

type Stock struct {
	SecID        string
	Ticker       string
	ExchangeCD   string
	ListStatusCD string
}

type Stockslice struct {
	Data []Stock
}

var APICONF = make(map[string]string)
var STOCKFILE = make(map[string]string)

func main() {
	cfg, err := config.ReadDefault(*configFile)
	if err != nil {
		panic(err)
	}

	APICONF["url"], _ = cfg.String("API", "url")
	APICONF["master"], _ = cfg.String("API", "master")
	APICONF["version"], _ = cfg.String("API", "version")
	APICONF["auth"], _ = cfg.String("API", "auth")
	loadHK, _ := cfg.Bool("FILE", "loadHK")

	STOCKFILE["FILE"], _ = cfg.String("FILE", "stocklist")

	var stock Stockslice

	reqpath := APICONF["url"] + "/" + APICONF["master"]
	if len(APICONF["version"]) > 0 {
		reqpath += "/" + APICONF["version"]
	}
	reqpath += "/getSecID.json?ticker=ticker&field=secID,listStatusCD,exchangeCD"
	req, err := http.NewRequest("GET", reqpath, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", "Bearer "+APICONF["auth"])

	httpClient := &http.Client{}

	resp, _ := httpClient.Do(req)

	if resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		err := json.Unmarshal(body, &stock)
		if err != nil {
			panic(err)
		}

		fout, err := os.Create(STOCKFILE["FILE"])
		defer fout.Close()
		if err != nil {
			panic(err)
		}

		//randomly mess the stocklist to optimize historical data processing
		// filter the XHKG2 ...
		stockmap := make(map[string]Stock, len(stock.Data))
		for _, value := range stock.Data {
			if value.ExchangeCD == "XSHG" {
				if value.Ticker[0] != '6' {
					continue
				}
			} else if value.ExchangeCD == "XSHE" {
				if value.Ticker[0] != '0' && value.Ticker[0] != '3' {
					continue
				}
			} else {
				if !loadHK {
					continue
				}
			}
			if value.ListStatusCD == "L" {
				stockmap[value.Ticker+"."+value.ExchangeCD] = value
			}
		}

		for k, _ := range stockmap {
			fout.WriteString(k + "\n")
		}
	}
}
