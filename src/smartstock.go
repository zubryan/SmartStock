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
var STOCKFILE = make(map[string]string)
var GOGROUP = 100

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
			Host:     DBCONF["host"],
			Username: DBCONF["username"],
			Password: DBCONF["password"],
			Database: DBCONF["database"],
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

	APICONF["url"], _ = cfg.String("API", "url")
	APICONF["market"], _ = cfg.String("API", "market")
	APICONF["version"], _ = cfg.String("API", "version")
	APICONF["auth"], _ = cfg.String("API", "auth")

	DBCONF["host"], _ = cfg.String("DB", "host")
	DBCONF["username"], _ = cfg.String("DB", "username")
	DBCONF["password"], _ = cfg.String("DB", "password")
	DBCONF["database"], _ = cfg.String("DB", "database")

	STOCKFILE["FILE"], _ = cfg.String("FILE", "stocklist")

	fin, err := os.Open(STOCKFILE["FILE"])
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

	stockLen := len(stockSecIds)

	modR := int(math.Mod(float64(stockLen), float64(GOGROUP)))
	var groupNum int
	if modR > 0 {
		groupNum = (stockLen / GOGROUP) + 1
	} else {
		groupNum = (stockLen / GOGROUP)
	}

	var securityId string
	m := 0
	n := 0
	chs := make([]chan int, groupNum)
	for i := 0; i < stockLen; i++ {
		m++
		securityId = stockSecIds[i][0:len(stockSecIds[i])-1] + "," + securityId
		if m%GOGROUP == 0 {
			chs[n] = make(chan int)
			go importData(securityId, chs[n])
			securityId = ""
			n++
		}
	}

	securityId = ""
	if modR > 0 {
		for p := 0; p < modR; p++ {
			securityId = stockSecIds[stockLen-modR+p][0:len(stockSecIds[stockLen-modR+p])-1] + "," + securityId
		}
		chs[n] = make(chan int)
		go importData(securityId, chs[n])
	}

	for _, ch := range chs {
		<-ch
	}
}
