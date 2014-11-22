package main

import (
	"encoding/json"
	. "fmt"
	"github.com/influxdb/influxdb/client"
	"io/ioutil"
	"math"
	"net/http"
	. "smartstock/framework"
	// "strings"
	"time"
)

const (
	DECIMALS = 100000
)

type MktEqud struct {
	SecID            string  // "secID": "002296.XSHE",
	TradeDate        string  // "tradeDate": "2014-10-31",
	SecShortName     string  // "secShortName": "辉煌科技",
	PreClosePrice    float64 // "preClosePrice": 20.55,
	ActPreClosePrice float64 // "actPreClosePrice": 20.55,
	OpenPrice        float64 // "openPrice": 20.55,
	HighestPrice     float64 // "highestPrice": 20.57,
	LowestPrice      float64 // "lowestPrice": 18.94,
	ClosePrice       float64 // "closePrice": 19.19,
	TurnoverVol      float64 // "turnoverVol": 34861787,
	TurnoverValue    float64 // "turnoverValue": 680173098.18,
	// "dealAmount": 27118,
	// "turnoverRate": 0.19,
	// "negMarketValue": 3521188356.05,
	MarketValuee float64 // "marketValue": 7228036699.8
}
type MktEqudslice struct {
	Data []MktEqud
}

// fields of MktEqud must fit
var MktEqudFields = [10]string{"secShortName", "preClosePrice", "actPreClosePrice", "openPrice", "highestPrice", "lowestPrice", "closePrice", "turnoverVol", "turnoverValue", "marketValue"}
var BeginDate = "20141001"

func init() {
	Println(STOCKCOUNT)
	// StockDatas = make([]Mktdata, STOCKCOUNT)
	SetProcess(Goproc{process, "dosth"})
	SetProcess(Goproc{process, "dosth"})
}

func ToInt(v float64) int64 {
	// not support negative
	return int64(math.Trunc(v*1e5 + 0.5))
}
func Div(v int64, w int64) int64 {
	if w == 0 {
		return 0
	}
	return v / w
}

func histdata(sec string) MktEqudslice {
	var histock MktEqudslice
	success := false
	var url = APICONF["url"] + "/" + APICONF["market"] + "/" + APICONF["version"] + "/getMktEqud.json?secID=" + sec + "&field="
	for _, field := range MktEqudFields {
		url += field + ","
	}
	url += "&beginDate=" + BeginDate
	req, _ := http.NewRequest("GET", url, nil)
	// fmt.Printf("Fetch %s on %s\n", sec, date)
	req.Header.Add("Authorization", "Bearer "+APICONF["auth"])
	retry := 0
	httpClient := &http.Client{}
	for !success && retry < 10 {
		resp, err := httpClient.Do(req)
		if err != nil {
			panic(err)
		}
		if resp.StatusCode == 200 {
			body, _ := ioutil.ReadAll(resp.Body)
			json.Unmarshal(body, &histock)
			success = true
		} else {
			Println("[ERROR] fail calling %s", url)
			time.Sleep(time.Second)
			success = false // don't retry for now
			retry++
		}
	}
	retry = 0
	Printf("Fetch OK %s : %d record(s) got\n", sec, len(histock.Data))
	return histock
}

func process(mds []Stock, ch chan int) {

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
		name := "mktdata_daily." + mds[i].Ticker_exchange
		name_corrected := "mktdata_daily_corrected." + mds[i].Ticker_exchange
		mktdataDaily := histdata(mds[i].Ticker_exchange)
		pointsofthedays := make([][]interface{}, len(mktdataDaily.Data))
		pointsofthedays_corrected := make([][]interface{}, len(mktdataDaily.Data))
		if len(mktdataDaily.Data) > 0 {
			for j := range mktdataDaily.Data {
				pointsofthedays[j] = []interface{}{
					mds[i].Ticker_exchange,                                                                                                                          // "ticker.exchange",
					mktdataDaily.Data[j].TradeDate,                                                                                                                  // "dataDate",
					ToInt(mktdataDaily.Data[j].OpenPrice),                                                                                                           // "openPrice",
					ToInt(mktdataDaily.Data[j].ClosePrice),                                                                                                          // "closePrice",
					ToInt(mktdataDaily.Data[j].PreClosePrice),                                                                                                       //"preClosePrice",
					ToInt(mktdataDaily.Data[j].HighestPrice),                                                                                                        // "highestPrice",
					ToInt(mktdataDaily.Data[j].LowestPrice),                                                                                                         // "lowestPrice",
					ToInt(mktdataDaily.Data[j].ClosePrice) - ToInt(mktdataDaily.Data[j].PreClosePrice),                                                              // "price_change",
					Div((ToInt(mktdataDaily.Data[j].ClosePrice)-ToInt(mktdataDaily.Data[j].PreClosePrice))*100*DECIMALS, ToInt(mktdataDaily.Data[j].PreClosePrice)), // "price_change_percentage",
					ToInt(mktdataDaily.Data[j].TurnoverVol),                                                                                                         // "volume",
					ToInt(mktdataDaily.Data[j].TurnoverValue),                                                                                                       // "ammount"
				}
			}
		}
		replaceSeries(c, name, pointsofthedays)

		for j := range pointsofthedays {
			pointsofthedays_corrected[j] = pointsofthedays[j]
			if ToInt(mktdataDaily.Data[j].PreClosePrice) != ToInt(mktdataDaily.Data[j].ActPreClosePrice) {
				for k := range pointsofthedays_corrected[:j] {

					pointsofthedays_corrected[k][3] = pointsofthedays_corrected[k][3].(int64) * ToInt(mktdataDaily.Data[j].PreClosePrice) / ToInt(mktdataDaily.Data[j].ActPreClosePrice)
					pointsofthedays_corrected[k][4] = pointsofthedays_corrected[k][4].(int64) * ToInt(mktdataDaily.Data[j].PreClosePrice) / ToInt(mktdataDaily.Data[j].ActPreClosePrice)
					pointsofthedays_corrected[k][5] = pointsofthedays_corrected[k][5].(int64) * ToInt(mktdataDaily.Data[j].PreClosePrice) / ToInt(mktdataDaily.Data[j].ActPreClosePrice)
					pointsofthedays_corrected[k][6] = pointsofthedays_corrected[k][6].(int64) * ToInt(mktdataDaily.Data[j].PreClosePrice) / ToInt(mktdataDaily.Data[j].ActPreClosePrice)
					pointsofthedays_corrected[k][7] = pointsofthedays_corrected[k][7].(int64) * ToInt(mktdataDaily.Data[j].PreClosePrice) / ToInt(mktdataDaily.Data[j].ActPreClosePrice)
					pointsofthedays_corrected[k][8] = pointsofthedays_corrected[k][8].(int64) * ToInt(mktdataDaily.Data[j].PreClosePrice) / ToInt(mktdataDaily.Data[j].ActPreClosePrice)

				}
			}
		}
		replaceSeries(c, name_corrected, pointsofthedays_corrected)

	}
	ch <- 1
}
func replaceSeries(c *client.Client, name string, points [][]interface{}) {
	c.Query("drop series " + name)
	putSeries(c, name, points)
}

func putSeries(c *client.Client, name string, points [][]interface{}) {
	series := &client.Series{
		Name: name,
		Columns: []string{
			"ticker.exchange",
			"dataDate",
			"openPrice",
			"closePrice",
			"preClosePrice",
			"highestPrice",
			"lowestPrice",
			"price_change",
			"price_change_percentage",
			"volume",
			"ammount"},
		Points: points,
	}
	c.WriteSeries([]*client.Series{series})
}

func main() {
	Main()
}
