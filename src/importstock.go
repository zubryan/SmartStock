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
	SecID  string
	Ticker string
}

type Stockslice struct {
	Data []Stock
}

var APICONF = make(map[string]string)

func main() {
	cfg, err := config.ReadDefault(*configFile)
	if err != nil {
		panic(err)
	}

	if cfg.HasSection("API") {
		section, err := cfg.SectionOptions("API")
		if err == nil {
			for _, v := range section {
				options, err := cfg.String("API", v)
				if err == nil {
					APICONF[v] = options
				}
			}
		}
	}

	var stock Stockslice
	req, err := http.NewRequest("GET", APICONF["url"]+"/"+APICONF["master"]+"/"+APICONF["version"]+"/getSecID.json?ticker=ticker&field=secID", nil)
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

		fout, err := os.Create("../data/stocklist")
		defer fout.Close()
		if err != nil {
			panic(err)
		}

		for _, value := range stock.Data {
			fout.WriteString(value.SecID + "\n")
		}
	}
}