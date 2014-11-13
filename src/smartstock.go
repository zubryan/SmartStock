package main

import (
	"client"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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

func main() {
	var stock Stockslice
	req, err := http.NewRequest("GET", "https://gw.wmcloud.com/data/market/1.0.0/getTickRTSnapshot.json?field=dataDate,dataTime,shortNM,currencyCD,prevClosePrice,openPrice,volume,value,deal,highPrice,lowPrice,lastPrice", nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", "Bearer f637816e9303cea3c12981eee33f76ffd090ce9ba1599bac4853fba65a226243")

	httpClient := &http.Client{}

	resp, _ := httpClient.Do(req)

	if resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		err := json.Unmarshal(body, &stock)
		if err != nil {
			panic(err)
		}

		var dataSh []Stock
		for _, v := range stock.Data {
			if v.ExchangeCD == "XSHG" {
				dataSh = append(dataSh, v)
			}
		}
		fmt.Println(len(dataSh))
	}

	c, err := client.NewClient(&client.ClientConfig{
		Host:     "192.168.129.136:8086",
		Username: "root",
		Password: "root",
		Database: "smartstock",
	})
	if err != nil {
		panic(err)
	}

	result, err := c.Query("select * from cpu_idle")
	if err != nil {
		panic(err)
	}

	fmt.Println(result[0].GetPoints())
}
