package main

import (
	"encoding/json"
	. "fmt"
	"github.com/influxdb/influxdb/client"
	"io/ioutil"
	"net/http"
	. "smartstock/framework"
)

const (
	DECIMALS = 100000
)

type TickRTSnapshot struct {
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

type TickRTSnapshotSlice struct {
	Data []TickRTSnapshot
}

var TickRTSnapshotFields = [12]string{"dataDate", "dataTime", "shortNM", "currencyCD", "prevClosePrice", "openPrice", "volume", "value", "deal", "highPrice", "lowPrice", "lastPrice"}

func init() {
	Println(STOCKCOUNT)
	SetProcess(Goproc{process, "doTickRTSnapshot"})
}

func process(mds []Stock, ch chan int) {
	var tickRTSnapshotSlice TickRTSnapshotSlice
	var (
		securityIds string
		fields      string
	)

	for _, id := range mds {
		securityIds += id.Ticker_exchange + ","
	}

	for _, field := range TickRTSnapshotFields {
		fields += field + ","
	}

	var url = APICONF["url"] + "/" + APICONF["market"] + "/" + APICONF["version"] + "/getTickRTSnapshot.json?securityID=" + securityIds + "&field=" + fields
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+APICONF["auth"])
	httpClient := &http.Client{}
	resp, _ := httpClient.Do(req)

	if resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &tickRTSnapshotSlice)

		c, _ := client.NewClient(&client.ClientConfig{
			Host:     DBCONF["host"],
			Username: DBCONF["username"],
			Password: DBCONF["password"],
			Database: DBCONF["database"],
		})

		for j := 0; j < len(tickRTSnapshotSlice.Data); j++ {
			name := "mktdata." + tickRTSnapshotSlice.Data[j].Ticker + "." + tickRTSnapshotSlice.Data[j].ExchangeCD
			var lastPrice int64
			if tickRTSnapshotSlice.Data[j].LastPrice == 0 {
				lastPrice = ToInt(tickRTSnapshotSlice.Data[j].PrevClosePrice)
			} else {
				lastPrice = ToInt(tickRTSnapshotSlice.Data[j].LastPrice)
			}
			priceChange := lastPrice - ToInt(tickRTSnapshotSlice.Data[j].PrevClosePrice)
			priceChangePt := Div(priceChange, ToInt(tickRTSnapshotSlice.Data[j].PrevClosePrice)) * 100 * DECIMALS

			series := &client.Series{
				Name: name,
				Columns: []string{
					"ticker.exchange",
					"dataDate", "dataTime",
					"lastPrice",
					"volume",
					"ammount",
					"price_change",
					"price_change_percentage",
				},
				Points: [][]interface{}{
					{tickRTSnapshotSlice.Data[j].Ticker + "." + tickRTSnapshotSlice.Data[j].ExchangeCD, tickRTSnapshotSlice.Data[j].DataDate, tickRTSnapshotSlice.Data[j].DataTime, lastPrice, tickRTSnapshotSlice.Data[j].Volume, tickRTSnapshotSlice.Data[j].Value, priceChange, priceChangePt},
				},
			}
			c.WriteSeries([]*client.Series{series})
		}
	}
	ch <- 1
}

func main() {
	Main()
}
