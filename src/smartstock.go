package main

import (
	"encoding/json"
	. "github.com/dimdin/decimal"
	"github.com/influxdb/influxdb/client"
	"io/ioutil"
	"net/http"
	. "smartstock/framework"
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
	SetProcess(Goproc{process, "Get 1 TickRTSnapshot"})
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

		c := GetNewDbClient()
		for j := 0; j < len(tickRTSnapshotSlice.Data); j++ {
			var lastPrice, priceChange, priceChangePct, preClosePrice Dec
			name := "mktdata." + tickRTSnapshotSlice.Data[j].Ticker + "." + tickRTSnapshotSlice.Data[j].ExchangeCD

			if tickRTSnapshotSlice.Data[j].LastPrice == 0 {
				lastPrice.SetFloat64(tickRTSnapshotSlice.Data[j].PrevClosePrice)
			} else {
				lastPrice.SetFloat64(tickRTSnapshotSlice.Data[j].LastPrice)
			}
			preClosePrice.SetFloat64(tickRTSnapshotSlice.Data[j].PrevClosePrice)

			priceChange.Sub(&lastPrice, &preClosePrice)
			priceChangePct.Div(&priceChange, &preClosePrice, DECIMAL_PCT+2)
			priceChangePct.Mul(&priceChangePct, New(100))
			priceChangePct.Round(DECIMAL_PCT)

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
					{
						tickRTSnapshotSlice.Data[j].Ticker + "." + tickRTSnapshotSlice.Data[j].ExchangeCD,
						tickRTSnapshotSlice.Data[j].DataDate,
						tickRTSnapshotSlice.Data[j].DataTime,
						lastPrice,
						tickRTSnapshotSlice.Data[j].Volume,
						tickRTSnapshotSlice.Data[j].Value,
						priceChange,
						priceChangePct,
					},
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
