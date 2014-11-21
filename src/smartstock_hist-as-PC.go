package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/influxdb/influxdb/client"
	"github.com/larspensjo/config"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
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

type TradingDate struct {
	ExchangeCD    string // "exchangeCD": "XSHE",
	CalendarDate  string // "calendarDate": "2014-09-01",
	IsOpen        int    // "isOpen": 1,
	PrevTradeDate string // "prevTradeDate": "2014-08-29"
}

type TradingDatelice struct {
	Data []TradingDate
}

var APICONF = make(map[string]string)
var DBCONF = make(map[string]string)
var STOCKFILE = make(map[string]string)
var GROUPMOD = 100
var Dates []string

func importData(securityId string, ch chan int) {
	var stock Stockslice
	c, _ := client.NewClient(&client.ClientConfig{
		Host:     DBCONF["host"],
		Username: DBCONF["username"],
		Password: DBCONF["password"],
		Database: DBCONF["database"],
	})

	for _, sec := range strings.Split(securityId, ",") {
		sec = strings.Trim(sec, "\r\n")
		for _, d := range Dates {

			name := "mktdata." + sec
			stock = histdata(sec, d)
			if len(stock.Data) > 0 {
				pointsoftheday := make([][]interface{}, len(stock.Data))
				for j := 0; j < len(stock.Data); j++ {
					if stock.Data[j].LastPrice == 0 {
						stock.Data[j].LastPrice = stock.Data[j].PrevClosePrice
					}
					priceChangePt := (stock.Data[j].LastPrice - stock.Data[j].PrevClosePrice)
					priceChange := priceChangePt / stock.Data[j].PrevClosePrice * 100
					pointsoftheday[j] = []interface{}{stock.Data[j].Ticker + "." + stock.Data[j].ExchangeCD, stock.Data[j].DataDate, stock.Data[j].DataTime, stock.Data[j].LastPrice, stock.Data[j].Volume, stock.Data[j].Value, priceChange, priceChangePt}
					// fmt.Print(pointsoftheday)
				}

				series := &client.Series{
					Name:    name,
					Columns: []string{"ticker.exchange", "dataDate", "dataTime", "lastPrice", "volume", "ammount", "price_change", "price_change_percentage"},
					Points:  pointsoftheday,
				}
				c.WriteSeries([]*client.Series{series})
			}
		}
	}
	ch <- 1
}

func tradedate() []string {
	var dates []string
	var date TradingDatelice
	var url = "https://gw.wmcloud.com/data/master/1.0.0/getTradeCal.json?field=&exchangeCD=XSHG,XSHE&beginDate=20140901&endDate=20141121&callback="
	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("Authorization", "Bearer "+APICONF["auth"])

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	defer resp.Body.Close()
	if err != nil {
		panic(err)
	}
	if resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &date)
		for _, d := range date.Data {
			if d.IsOpen == 1 {
				dates = append(dates, strings.Join(strings.Split(d.CalendarDate, "-"), ""))
			}
		}
	}
	return dates
}

func histdata(sec string, date string) Stockslice {
	var histock Stockslice
	success := false
	var url = APICONF["url"] + "/" + APICONF["market"] + "/" + APICONF["version"] + "/getTicksHistOneDay.json?securityID=" + sec + "&field=dataDate,dataTime,shortNM,currencyCD,prevClosePrice,openPrice,volume,value,deal,highPrice,lowPrice,lastPrice&date=" + date
	req, _ := http.NewRequest("GET", url, nil)
	// fmt.Printf("Fetch %s on %s\n", sec, date)
	req.Header.Add("Authorization", "Bearer "+APICONF["auth"])

	httpClient := &http.Client{}
	for !success {
		resp, err := httpClient.Do(req)
		if err != nil {
			panic(err)
		}
		if resp.StatusCode == 200 {
			body, _ := ioutil.ReadAll(resp.Body)
			json.Unmarshal(body, &histock)
			success = true
		} else {
			fmt.Printf("[ERROR] Fail calling %s on %d\n", sec, date)
			time.Sleep(1000)
			success = true // don't retry for now
		}
	}
	fmt.Printf("Fetch OK %s on %s : %d record(s) got\n", sec, date, len(histock.Data))
	return histock
}

func main() {
	cfg, err := config.ReadDefault(*configFile)
	if err != nil {
		panic(err)
	}

	GROUPMOD, _ = cfg.Int("GENERAL", "groupmod")

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
	Dates = tradedate()

	time.Sleep(1000)
	panic("")
	//histdata("000429.XSHE", "20140901")

	var stockSecIds []string
	r := bufio.NewReader(fin)
	s, err := r.ReadString('\n')
	for err == nil {
		stockSecIds = append(stockSecIds, s)
		s, err = r.ReadString('\n')
	}

	stockLen := len(stockSecIds)

	modR := int(math.Mod(float64(stockLen), float64(GROUPMOD)))
	var groupNum int
	if modR > 0 {
		groupNum = (stockLen / GROUPMOD) + 1
	} else {
		groupNum = (stockLen / GROUPMOD)
	}

	var securityId string
	m := 0
	n := 0
	chs := make([]chan int, groupNum)
	for i := 0; i < stockLen; i++ {
		m++
		securityId = stockSecIds[i][0:len(stockSecIds[i])-1] + "," + securityId
		if m%GROUPMOD == 0 {
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
