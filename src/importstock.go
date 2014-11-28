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

	STOCKFILE["FILE"], _ = cfg.String("FILE", "stocklist")

	var stock Stockslice
	req, err := http.NewRequest("GET", APICONF["url"]+"/"+APICONF["master"]+"/"+APICONF["version"]+"/getSecID.json?ticker=ticker&field=secID,listStatusCD,exchangeCD", nil)
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

		for _, value := range stock.Data {
			// ashare stock only
			if value.ExchangeCD == "XSHG" {
				if value.Ticker[0] != '6' {
					continue
				}
			} else if value.ExchangeCD == "XSHE" {
				if value.Ticker[0] != '0' && value.Ticker[0] != '3' {
					continue
				}
			} else {
				continue
			}
			// exclude unlisted stock
			if value.ListStatusCD == "L" {
				fout.WriteString(value.SecID + "\n")
			}
		}
	}
}
