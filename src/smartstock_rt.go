package main

import (
	"encoding/json"
	"fmt"
	. "github.com/dimdin/decimal"
	"github.com/influxdb/influxdb/client"
	"strings"

	. "smartstock/framework"
	"time"
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
	RetCode int
	RetMsg  string
	Data    []TickRTSnapshot
}

var TickRTSnapshotFields = [12]string{"dataDate", "dataTime", "shortNM", "currencyCD", "prevClosePrice", "openPrice", "volume", "value", "deal", "highPrice", "lowPrice", "lastPrice"}
var lasttimes []string
var recCount []int

const (
	timetoSleep = time.Second
)

func init() {
	if !DEBUGMODE {
		SetGoInf()
	}
	lasttimes = make([]string, STOCKCOUNT)
	recCount = make([]int, STOCKCOUNT)
	SetProcess(Goproc{process, "Get TickRTSnapshot"})
	DBdropShards([]string{"mktdata"})
}

func process(mds []Stock, ch chan int) {
	var tickRTSnapshotSlice TickRTSnapshotSlice
	var securityIds string

	idxMap := make(map[string]int)
	for i := range mds {
		idxMap[mds[i].Ticker_exchange] = mds[i].Idx
		securityIds += mds[i].Ticker_exchange + ","
	}

	for {
		retry := APIMAXRETRY
		var body []byte
		var err error
		ok := false
		for i := range mds {
			StartProcess(mds[i].Idx)
		}
		for !ok && retry > 0 {
			body, err = CallDataAPI(
				"market",
				"1.0.0",
				"getTickRTSnapshot.json",
				[]string{
					"securityID=" + securityIds,
					"field=" + strings.Join(TickRTSnapshotFields[:], ","),
				})
			if err != nil {
				Logger.Print(string(body))
				for i := range mds {
					SetStockStatus(mds[i].Idx, STATUS_ERROR, "Standby")
				}
				time.Sleep(timetoSleep)
				continue
			}
			json.Unmarshal(body, &tickRTSnapshotSlice)

			switch tickRTSnapshotSlice.RetCode {
			case -1:
				Logger.Print(string(body))
				Logger.Printf("Fetch OK but no Data %s : %d - %s \n", securityIds, tickRTSnapshotSlice.RetCode, tickRTSnapshotSlice.RetMsg)
				fallthrough
			case 1:
				for i := range mds {
					SetStockStatus(mds[i].Idx, STATUS_RUNNING, "Call DataAPI OK")
				}
				ok = true
			default:
				Logger.Print(string(body))
				for i := range mds {
					SetStockStatus(mds[i].Idx, STATUS_RETRYING, "Call DataAPI Failed (Maybe busy) Retry ...")
				}
				retry--
				time.Sleep(100 * time.Millisecond)
				Logger.Printf("%s\n", string(body))
				Logger.Printf("Fetch Failed %s : %d - %s | RetryRemain = %d ..\n", securityIds, tickRTSnapshotSlice.RetCode, tickRTSnapshotSlice.RetMsg, retry)
			}
		}

		c := GetNewDbClient()
		for j := 0; j < len(tickRTSnapshotSlice.Data); j++ {
			var lastPrice, priceChange, priceChangePct, preClosePrice Dec
			ticker_exchange := tickRTSnapshotSlice.Data[j].Ticker + "." + tickRTSnapshotSlice.Data[j].ExchangeCD

			idx, ok := idxMap[ticker_exchange]
			if lasttimes[idx] == tickRTSnapshotSlice.Data[j].DataDate+tickRTSnapshotSlice.Data[j].DataTime {

				// prevent duplicate data
				SetStockStatus(idx, STATUS_DONE, fmt.Sprintf("%d Record(s)\nLast Updated: %s", recCount[idx], lasttimes[idx]))
				continue
			}
			if !ok {
				Logger.Println("invalid ticker received:", ticker_exchange)
				continue //impossible?
			}
			name := "mktdata." + ticker_exchange
			if tickRTSnapshotSlice.Data[j].LastPrice == 0 {
				lastPrice.SetFloat64(tickRTSnapshotSlice.Data[j].PrevClosePrice)
			} else {
				lastPrice.SetFloat64(tickRTSnapshotSlice.Data[j].LastPrice)
			}
			preClosePrice.SetFloat64(tickRTSnapshotSlice.Data[j].PrevClosePrice)

			priceChange.Sub(&lastPrice, &preClosePrice)
			priceChangePct = CalcPercentage(priceChange, preClosePrice, DECIMAL_PCT+2)

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
						ticker_exchange,
						tickRTSnapshotSlice.Data[j].DataDate,
						tickRTSnapshotSlice.Data[j].DataTime,
						lastPrice.Float64(),
						tickRTSnapshotSlice.Data[j].Volume,
						tickRTSnapshotSlice.Data[j].Value,
						priceChange.Float64(),
						priceChangePct.Float64(),
					},
				},
			}
			Logger.Println(series)
			err = c.WriteSeries([]*client.Series{series})
			if err != nil {
				Logger.Println(err)
				SetStockStatus(idx, STATUS_ERROR, "ERROR writing to db...\n"+err.Error())
			} else {
				recCount[idx]++
				SetStockStatus(idx, STATUS_DONE, fmt.Sprintf("%d Record(s)\nLast Updated: %s", recCount[idx], lasttimes[idx]))
				// prevent duplicate data
				lasttimes[idx] = tickRTSnapshotSlice.Data[j].DataDate + tickRTSnapshotSlice.Data[j].DataTime
			}
		}

		for i := range mds {
			if mds[i].Status != STATUS_DONE {
				SetStockStatus(mds[i].Idx, STATUS_RETRYING, "No data, Retry...")
			}
		}
		time.Sleep(timetoSleep)
		if DEBUGMODE {
			Logger.Printf("%d / %d records got.  \n", len(tickRTSnapshotSlice.Data), len(mds))
			break
		}
	}
	ch <- 1
}

func main() {
	Main()
}
