package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	// "encoding/json"
	// "errors"
	. "github.com/dimdin/decimal"
	. "smartstock/framework"
	// "strings"
	// "time"
)

type MktEqudRef struct {
	SecID          string  // "secID": "002296.XSHE",
	TradeDate      string  // "tradeDate": "2014-10-31",
	SecShortName   string  // "secShortName": "辉煌科技",
	NegMarketValue float64 // "preClosePrice": 20.55,
	TransCurrCD    string
}

var MktEqudRefFields = [3]string{"secShortName", "negMarketValue", "transCurrCD"}

type MktEqudRefslice struct {
	RetCode int
	RetMsg  string
	Data    []MktEqudRef
}

type Macd struct {
	time            int64  // datetime.UnixNano()   / 1e6,
	ticker_exchange string // ticker_exchange,      // "ticker.exchange",
	dataDate        string // Mktdata[j].TradeDate, // "dataDate",
	EMAS            Dec    //EMA12
	EMAL            Dec    //EMA26
	Dif             Dec    // "DIF": "EMAS-EMAL",
	Dea             Dec    // "DEA": "EMA(DIF,9)",
	Macd            Dec    // "MACD": "(DIF-DEA)*2"
}

type Criteria struct {
	name  string
	desc  string
	isHit func(Metrics) bool
}
type Refdata struct {
	ticker_exchange string
	lasttime        int64
	shortName       string
	tradableQty     Dec
	currency        string
	criterias       []Criteria
	closePriceSeq   []Dec
	cpsum4          Dec
	cpsum9          Dec
	cpsum19         Dec
	volSeq          []Dec
	prevEMAL        Dec
	prevEMAS        Dec
	prevDEA         Dec
	lastTradeDate   string
	isActive        bool
	isQualified     bool // some stock has less data than needed
}
type Alert struct {
	ticker_exchange string // "ticker.exchange",
	dataDate        string // "dataDate",
	dataTime        string // "dataTime",
	criteriaHit     string // "criteriaHit"
}
type Metrics struct {
	dataTime string // "dataTime",
	X1_1     Dec    // "X1-1", Volume Ratio 5d
	X1_2     Dec    // "X1-2", Volume Ratio 10d
	X2       Dec    // "X2", PrcChgPct
	Y1       bool   // "Y1", MA5>=MA10>=MA20
	Y2       bool   // "Y2", MA5<=MA10<=MA20
	X3       Dec    // "X3", abs(MACD)
	X4       Dec    // "X4", tradableQty * Prc
}

var DefaultCriterias []Criteria = []Criteria{
	Criteria{
		name:  "Criteria 1",
		desc:  "xxxx",
		isHit: isHitCriteria_1},
	Criteria{
		name:  "Criteria 2",
		desc:  "xxxx",
		isHit: isHitCriteria_2},
}

func isHitCriteria_1(m Metrics) bool {
	return false
}
func isHitCriteria_2(m Metrics) bool {
	return false
}

var Ref []Refdata

func init() {
	Ref = make([]Refdata, STOCKCOUNT)
	SetProcess(Goproc{loadRefData, "Loading RefData..."})
	SetProcess(Goproc{calcRealTimeMktData, "Continuously Monitor the Markets ..."})
	//	DBdropShards([]string{"default"})
}
func getRefdataDB(sec Stock) (Refdata, error) {
	var ref Refdata
	ref.isQualified = false
	ref.lasttime = 0
	c := GetNewDbClient()
	query := fmt.Sprintf("select dataDate,closePrice,volume "+
		"from mktdata_daily_corrected.%s limit 19 order desc", sec.Ticker_exchange)
	series, err := c.Query(query)
	if err != nil {
		Logger.Println(err)
	}
	columns := series[0].GetColumns()
	points := series[0].GetPoints()
	if len(points) != 19 {
		return ref, nil
	}

	var idxDataDate, idxClosePrice, idxVolume int
	for i, _ := range columns {
		switch columns[i] {
		case "dataDate":
			idxDataDate = i
		case "closePrice":
			idxClosePrice = i
		case "volume":
			idxVolume = i
		default:
		}
	}
	ref.ticker_exchange = sec.Ticker_exchange
	// ref.shortName = "undefined"
	// ref.tradableQty
	// ref.currency
	ref.criterias = DefaultCriterias
	ref.closePriceSeq = make([]Dec, 19)
	for i, p := range points {
		f, ok := p[idxClosePrice].(float64)
		if !ok {
			Logger.Println("invalid prc")
			return ref, errors.New("invalid prc")
		}
		ref.closePriceSeq[i].SetFloat64(f)
	}

	ref.cpsum4 = *New(0)
	ref.cpsum9 = *New(0)
	ref.cpsum19 = *New(0)
	sum := *New(0)
	for i, val := range ref.closePriceSeq {
		sum.Add(&sum, &val)
		if i == 3 {
			ref.cpsum4 = sum
		}
		if i == 8 {
			ref.cpsum9 = sum
		}
		if i == 18 {
			ref.cpsum19 = sum
		}
	}
	for i, p := range points {
		f, ok := p[idxVolume].(float64)
		if !ok {
			Logger.Println("invalid volume")
			return ref, errors.New("invalid volume")
		}
		ref.volSeq[i].SetFloat64(f)
	}

	s, ok := points[0][idxDataDate].(string)
	if !ok {
		Logger.Println("invalid date")
		return ref, errors.New("invalid date")
	}

	ref.lastTradeDate = s
	ref.isActive = false

	query = fmt.Sprintf("select EMAL,EMAS,DEA "+
		"from indicators.macd.%s limit 1", sec.Ticker_exchange)
	series, err = c.Query(query)
	if err != nil {
		Logger.Println(err)
	}
	columns = series[0].GetColumns()
	points = series[0].GetPoints()
	if len(points) == 0 {
		Logger.Panicln("Data Error! No MACD")
	}

	var idxEMAL, idxEMAS, idxDEA int
	for i, _ := range columns {
		switch columns[i] {
		case "EMAL":
			idxEMAL = i
		case "EMAS":
			idxEMAS = i
		case "DEA":
			idxDEA = i
		default:
		}
	}

	f, ok := points[0][idxEMAL].(float64)
	if !ok {
		Logger.Println("invalid prevEMAL")
		return ref, errors.New("invalid prevEMAL")
	}
	ref.prevEMAL.SetFloat64(f)

	f, ok = points[0][idxEMAS].(float64)
	if !ok {
		Logger.Println("invalid prevEMAL")
		return ref, errors.New("invalid prevEMAL")
	}
	ref.prevEMAS.SetFloat64(f)

	f, ok = points[0][idxDEA].(float64)
	if !ok {
		Logger.Println("invalid prevEMAL")
		return ref, errors.New("invalid prevEMAL")
	}
	ref.prevDEA.SetFloat64(f)
	ref.isQualified = true
	return ref, nil
}

func getRefdataDataAPI(sec Stock, date string) (MktEqudRefslice, error) {
	var refdata MktEqudRefslice
	retry := APIMAXRETRY
	ok := false
	for !ok && retry > 0 {
		body, err := CallDataAPI(
			"market",
			"1.0.0",
			"getMktEqud.json",
			[]string{
				"secID=" + sec.Ticker_exchange,
				"field=" + strings.Join(MktEqudRefFields[:], ","),
				"&beginDate=" + date,
				"&endDate=" + date,
			})
		//Logger.Print(string(body))
		if err != nil {
			Logger.Panic(err)
		}
		json.Unmarshal(body, &refdata)

		switch refdata.RetCode {
		case -1:
			Logger.Printf("Fetch OK but no Data %s : %d - %s \n", sec, refdata.RetCode, refdata.RetMsg)
			fallthrough
		case 1:
			SetStockStatus(sec.Idx, STATUS_RUNNING, "Call DataAPI OK")
			ok = true
		default:
			SetStockStatus(sec.Idx, STATUS_RETRYING, "Call DataAPI Failed Retry ...")
			retry--
			time.Sleep(100 * time.Millisecond)
			Logger.Printf("%s\n", string(body))
			Logger.Printf("Fetch Failed %s : %d - %s | RetryRemain = %d ..\n", sec, refdata.RetCode, refdata.RetMsg, retry)
		}
	}
	if retry == 0 {
		return refdata, errors.New("Failed calling DataAPI...")
	}
	return refdata, nil
}

func loadRefData(mds []Stock, ch chan int) {
	// c := GetNewDbClient()
	var err error
	var res MktEqudRefslice
	for i := range mds {
		StartProcess(mds[i].Idx)
		var pRef *Refdata = &Ref[mds[i].Idx]
		*pRef, err = getRefdataDB(mds[i])
		if err != nil {
			Logger.Panic(err)
		}
		res, err = getRefdataDataAPI(mds[i], (*pRef).lastTradeDate)
		if err != nil {
			Logger.Panic(err)
		}
		if len(res.Data) == 0 {
			Logger.Panic("No data from API")
		}
		(*pRef).currency = res.Data[0].TransCurrCD
		(*pRef).shortName = res.Data[0].SecShortName
		var NegMv, preClose Dec
		NegMv.SetFloat64(res.Data[0].NegMarketValue)
		preClose = (*pRef).closePriceSeq[0]
		(*pRef).tradableQty = *new(Dec).Div(&NegMv, &preClose, 0)
		SetStockStatus(mds[i].Idx, STATUS_DONE, "GetRef OK.")
	}
	ch <- 1
}

func calcRealTimeMktData(mds []Stock, ch chan int) {
	// c := GetNewDbClient()
	var m Metrics
	for {
		for i := range mds {
			var idx = mds[i].Idx
			var pRef = &Ref[idx]
			StartProcess(idx)

			if (*pRef).isQualified {

				m = CalcMetrics(idx)
			}
			if (*pRef).isActive {
				var havealert bool
				havealert = HaveAlerts(idx, m)
				if havealert {
					(*pRef).isQualified = false
					SetStockStatus(idx, STATUS_READY, "Alert Raised.")
				} else {
					SetStockStatus(idx, STATUS_READY, "Standby")
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	ch <- 1
}
func CalcMetrics(idx int) bool {
	var pRef = &Ref[idx]
	c := GetNewDbClient()
	query := fmt.Sprintf("select dataDate,dataTime,lastPrice,price_change_percentage,volume "+
		"from mktdata.%s where time > %d", (*pRef).ticker_exchange, (*pRef).lasttime)
	series, err := c.Query(query)
	if err != nil {
		Logger.Println(err)
	}
	columns := series[0].GetColumns()
	points := series[0].GetPoints()
	var idxtime, idxdataDate, idxdataTime, idxlastPrice, idxPriceChgPct, idxVol int
	for i, _ := range columns {
		switch columns[i] {
		case "time":
			idxtime = i
		case "dataDate":
			idxdataDate = i
		case "dataTime":
			idxdataTime = i
		case "lastPrice":
			idxlastPrice = i
		case "price_change_percentage":
			idxPriceChgPct = i
		case "volume":
			idxVol = i
		default:
		}
	}
	// X1_1     Dec    // "X1-1", Volume Ratio 5d
	// X1_2     Dec    // "X1-2", Volume Ratio 10d
	// X2       Dec    // "X2", PrcChgPct
	// Y1       bool   // "Y1", MA5>=MA10>=MA20
	// Y2       bool   // "Y2", MA5<=MA10<=MA20
	// X3       Dec    // "X3", abs(MACD)
	// X4       Dec    // "X4", tradableQty * Prc
	for _, p := range points {
		var m Metrics
		var volume float64
		var lstprice float64
		m.dataTime,_ = p[idxdataTime].(string)
		volume , _ := p[idxVol].(float64)
		lstprice , _ := p[idxlastPrice].(float64)
		volDec := new(Dec).SetFloat64(volume)
		prcDec := new(Dec).SetFloat64(lstprice)
		m.X1_1 = new(Dec).Add(&volDec,&(*pRef).)
	}
	// (*pRef).lasttime =

	return m
}
func HaveAlerts(idx int, m Metrics) bool {
	var pRef = &Ref[idx]
	for i := range (*pRef).criterias {
		if (*pRef).criterias[i].isHit(m) {

		}
	}

	return false
}

func Alert2Pnts(alerts []Alert) [][]interface{} {
	var points [][]interface{}
	points = make([][]interface{}, 1)
	return points
}

func main() {
	Main()
}
