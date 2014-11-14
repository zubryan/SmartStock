package main

import (
	"bufio"
	"encoding/json"
	"flag"
	//"fmt"
	"github.com/influxdb/influxdb/client"
	"github.com/larspensjo/config"
	"io/ioutil"
	"math"
	"net/http"
	"os"
)

var (
	configFile = flag.String("configfile", "config.ini", "General configuration file")
)

type Stock struct {
	Timestamp      int64
	Ticker         string
	ExchangeCD     string
	DataDate       string
	DataTime       string
	ShortNM        string
	CurrencyCD     string
	PrevClosePrice float64
	OpenPrice      float64
	Volume         int64
	Value          float64
	Deal           int32
	HighPrice      float64
	LowPrice       float64
	LastPrice      float64
}

type Stockslice struct {
	Data []Stock
}

var APICONF = make(map[string]string)
var DBCONF = make(map[string]string)

var CONSTMOD = 2

func importData(securityId string, ch chan int) {
	var stock Stockslice
	var url = APICONF["url"] + "/" + APICONF["market"] + "/" + APICONF["version"] + "/getTickRTSnapshot.json?securityID=" + securityId + "&field=dataDate,dataTime,shortNM,currencyCD,prevClosePrice,openPrice,volume,value,deal,highPrice,lowPrice,lastPrice"
	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("Authorization", "Bearer "+APICONF["auth"])

	httpClient := &http.Client{}

	resp, _ := httpClient.Do(req)

	if resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &stock)

		c, _ := client.NewClient(&client.ClientConfig{
			Host:     "192.168.129.136:8086",
			Username: "root",
			Password: "root",
			Database: "smartstock",
		})

		for j := 0; j < len(stock.Data); j++ {
			name := "mktdata." + stock.Data[j].Ticker + "." + stock.Data[j].ExchangeCD
			series := &client.Series{
				Name:    name,
				Columns: []string{"ticker.exchange", "dataDate", "dataTime", "lastPrice", "volume"},
				Points: [][]interface{}{
					{stock.Data[j].Ticker + "." + stock.Data[j].ExchangeCD, stock.Data[j].DataDate, stock.Data[j].DataTime, stock.Data[j].LastPrice, stock.Data[j].Volume},
				},
			}
			c.WriteSeries([]*client.Series{series})
		}
	}
	ch <- 1
}

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

	if cfg.HasSection("DB") {
		section, err := cfg.SectionOptions("DB")
		if err == nil {
			for _, v := range section {
				options, err := cfg.String("DB", v)
				if err == nil {
					DBCONF[v] = options
				}
			}
		}
	}

	fin, err := os.Open("../data/stocklist")
	defer fin.Close()
	if err != nil {
		panic(err)
	}

	var stockSecIds []string
	r := bufio.NewReader(fin)
	s, err := r.ReadString('\n')
	for err == nil {
		stockSecIds = append(stockSecIds, s)
		s, err = r.ReadString('\n')
	}

	var stockSecId []string = stockSecIds[:9]
	stockLen := len(stockSecId)

	modR := int(math.Mod(float64(stockLen), float64(CONSTMOD)))
	var groupNum int
	if modR > 0 {
		groupNum = (stockLen / CONSTMOD) + 1
	} else {
		groupNum = (stockLen / CONSTMOD)
	}

	var securityId string
	m := 0
	n := 0
	chs := make([]chan int, groupNum)
	for i := 0; i < len(stockSecId); i++ {
		m++
		securityId = stockSecId[i][0:len(stockSecId[i])-1] + "," + securityId
		if m%CONSTMOD == 0 {
			chs[n] = make(chan int)
			go importData(securityId, chs[n])
			securityId = ""
			n++
		}
	}

	securityId = ""
	if modR > 0 {
		for p := 0; p < modR; p++ {
			securityId = stockSecId[stockLen-modR+p][0:len(stockSecId[stockLen-modR+p])-1] + "," + securityId
		}
		chs[n] = make(chan int)
		go importData(securityId, chs[n])
	}

	for _, ch := range chs {
		<-ch
	}
}
