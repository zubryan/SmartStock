package main

import (
	"bufio"
	"strings"
	// "encoding/json"
	"flag"
	"fmt"
	"github.com/influxdb/influxdb/client"
	"github.com/larspensjo/config"
	"os"
)

var (
	configFile = flag.String("configfile", "config.ini", "General configuration file")
)

type Mktdata struct {
	Ticker_exchange         string   //  "ticker.exchange",
	DataDate                string   // "dataDate",
	OpenPrice               string   // "openPrice",
	ClosePrice              string   // "closePrice",
	HighestPrice            float64  // "highestPrice",
	LowestPrice             float64  // "lowestPrice",
	Price_change            float64  // "price_change",
	Price_change_persentage float64  // "price_change_percentage",
	Volume                  int64    // "volume",
	Ammount                 int64    //“ammount"
	Macd                    Macd     // "macd"
	tradableQty             int64    //    "tradableQty": "tradable stock qty",
	currency                string   //    "currency": "calculation currency(计价货币)",
	criterias               []string //    "criterias": "criterias needed for alerts，eg. [c1,c2,c4]",
	shortName               string   //    "shortName": "instrument name",
	isActive                bool     //    "isActive": "whether to calculate"
}

type Macd struct {
	Dif  float64 // "DIF": "EMA12-EMA26",
	Dea  float64 // "DEA": "EMA(DIF,9)",
	Macd float64 // "MACD": "(DIF-DEA)*2"
}

var (
	DBCONF    = make(map[string]string)
	GROUPMOD  = 100
	Stockfile string
	Mktdatas  []Mktdata
)

func prepRefData(mds []Mktdata, ch chan int) {

	c, err := client.NewClient(&client.ClientConfig{
		Host:     DBCONF["host"],
		Username: DBCONF["username"],
		Password: DBCONF["password"],
		Database: DBCONF["database"],
	})
	if err != nil {
		panic(err)
	}
	for i, _ := range mds {
		// fmt.Print(mds[i].Ticker_exchange)
		result, err := c.Query("select * from mktdata.*" + mds[i].Ticker_exchange)
		if err == nil {
			//fmt.Println(err) ignore error
			for _, series := range result {
				fmt.Println(series.Name)
			}
		}
	}
	ch <- 1
}

func getMktData(mds []Mktdata, ch chan int) {
	for i, _ := range mds {
		mds[i].Ammount = 1
	}
	ch <- 1
}

func main() {

	fmt.Println("Preparing...")
	initialize()
	fmt.Println("Loading Refdata...")
	loadrefdata()
	fmt.Println("Calculating Data...")
	process()
	fmt.Println("Done")
}

func process() {

}

func initialize() {
	cfg, err := config.ReadDefault(*configFile)
	if err != nil {
		panic(err)
	}

	GROUPMOD, _ = cfg.Int("GENERAL", "groupmod")

	DBCONF["host"], _ = cfg.String("DB", "host")
	DBCONF["username"], _ = cfg.String("DB", "username")
	DBCONF["password"], _ = cfg.String("DB", "password")
	DBCONF["database"], _ = cfg.String("DB", "database")

	Stockfile, _ = cfg.String("FILE", "stocklist")

	fillstocklist()
}

func fillstocklist() {

	fin, err := os.Open(Stockfile)
	defer fin.Close()
	if err != nil {
		panic(err)
	}
	r := bufio.NewReader(fin)
	s, err := r.ReadString('\n')
	for err == nil {
		var md Mktdata
		md.Ticker_exchange = strings.Trim(s, "\n\r")
		Mktdatas = append(Mktdatas, md)
		s, err = r.ReadString('\n')
	}
}

func loadrefdata() {
	stockLen := len(Mktdatas)
	chs := make(map[int]chan int)
	for i := 0; i*GROUPMOD < stockLen; i += 1 {
		var slen int = (i + 1) * GROUPMOD
		if slen > stockLen {
			slen = stockLen
		}
		chs[i] = make(chan int)
		go prepRefData(Mktdatas[i*GROUPMOD:slen], chs[i])
		// j := i
	}
	for i, ch := range chs {
		fmt.Printf("waiting gort %d\n", i)
		<-ch
	}
}
