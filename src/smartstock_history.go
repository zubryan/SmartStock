package main

import (
	"encoding/json"
	"errors"
	. "github.com/dimdin/decimal"
	. "smartstock/framework"
	"strings"
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
	RetCode int
	RetMsg  string
	Data    []MktEqud
}

type MktdataDaily struct {
	time            int64  // datetime.UnixNano()   / 1e6,
	ticker_exchange string // ticker_exchange,      // "ticker.exchange",
	dataDate        string // Mktdata[j].TradeDate, // "dataDate",
	openPrice       Dec    // openPrice,            // "openPrice",
	closePrice      Dec    // closePrice,           // "closePrice",
	preClosePrice   Dec    // preClosePrice,        // "preClosePrice",
	highestPrice    Dec    // highestPrice,         // "highestPrice",
	lowestPrice     Dec    // lowestPrice,          // "lowestPrice",
	priceChange     Dec    // priceChange,          // "price_change",
	priceChangePct  Dec    // priceChangePct,       // "price_change_percentage",
	volume          Dec    // turnoverVol,          // "volume",
	ammount         Dec    // turnoverValue,        // "ammount"
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
type IndicatorProc struct {
	Name string
	Desc string
	Calc func([][]interface{}, [][]interface{}) error
}

// fields of MktEqud must fit
var MktEqudFields = [10]string{"secShortName", "preClosePrice", "actPreClosePrice", "openPrice", "highestPrice", "lowestPrice", "closePrice", "turnoverVol", "turnoverValue", "marketValue"}
var BeginDate = "19900101"
var (
	columns_mktdata_daily = [...]string{
		"time",
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
		"ammount",
	}
	columns_macd = [...]string{
		"time",
		"ticker.exchange", //"as is",
		"dataDate",        // "Date of this snapshot from mktdata",
		"DIF",             // "EMA12-EMA26",
		"DEA",             // "EMA(DIF,9)",
		"MACD",            // "(DIF-DEA)*2"
	}
	debuggonumber = 2
)

func init() {
	// StockDatas = make([]Mktdata, STOCKCOUNT)
	SetProcess(Goproc{loadHistdata, "loadHistdata"})
	initDB()
}

func initDB() {
	c := GetNewDbClient()
	// drop ShardSpace instead of droping series which is mu......ch slower~~~
	Logger.Println("Clear ShardSpace mktdata_daily")
	ssps, _ := c.GetShardSpaces()
	for _, ssp := range ssps {
		if ssp.Database == DBCONF["database"] {
			if ssp.Name == "mktdata_daily" ||
				ssp.Name == "mktdata" ||
				ssp.Name == "indicators" {
				c.DropShardSpace(DBCONF["database"], ssp.Name)
				c.CreateShardSpace(DBCONF["database"], ssp)
			}
		}
	}
}
func histdata(sec string) (MktEqudslice, error) {
	var histock MktEqudslice
	retry := APIMAXRETRY
	ok := false
	//1 for success , -1 for no data , other retry
	for !ok && retry > 0 {
		body, err := CallDataAPI(
			"market",
			"1.0.0",
			"getMktEqud.json",
			[]string{
				"secID=" + sec,
				"field=" + strings.Join(MktEqudFields[:], ","),
				"&beginDate=" + BeginDate,
			})
		//Logger.Print(string(body))
		if err != nil {
			Logger.Panic(err)
		}
		json.Unmarshal(body, &histock)

		if histock.RetCode == 1 {
			ok = true
		} else if histock.RetCode == -1 {
			ok = true
			Logger.Printf("Fetch OK but no Data %s : %d - %s \n", sec, histock.RetCode, histock.RetMsg)
		} else {
			retry--
			Logger.Printf("%s\n", string(body))
			Logger.Printf("Fetch Failed %s : %d - %s | RetryRemain = %d ..\n", sec, histock.RetCode, histock.RetMsg, retry)
		}
	}
	if retry == 0 {
		return histock, errors.New("Failed calling DataAPI...")
	}
	//
	return histock, nil
}

func calcMACD(MDSeq []MktdataDaily, Macd []Macd) {
	var (
		periodS int64 = 12
		periodL int64 = 26
		periodD int64 = 9
	)
	for i := range MDSeq {
		datetime, _ := time.Parse("2006-01-02", MDSeq[i].dataDate)
		Macd[i].time = datetime.UnixNano() / 1e6
		Macd[i].ticker_exchange = MDSeq[i].ticker_exchange
		Macd[i].dataDate = MDSeq[i].dataDate // "dataDate",
		if i == 0 {
			Macd[i].EMAS = MDSeq[i].closePrice
			Macd[i].EMAL = MDSeq[i].closePrice
			EMAS := Macd[i].EMAS
			EMAL := Macd[i].EMAL
			Macd[i].Dif.Sub(&EMAS, &EMAL).Round(7)
			Macd[i].Dea = Macd[i].Dif
		} else {
			closePrice := MDSeq[i].closePrice
			prevEMAS := Macd[i-1].EMAS
			prevEMAL := Macd[i-1].EMAL
			prevDEA := Macd[i-1].Dea
			Macd[i].EMAS.Div(new(Dec).Add(new(Dec).Mul(&closePrice, New(2)), new(Dec).Mul(&prevEMAS, New(periodS-1))), New(periodS+1), 7)
			Macd[i].EMAL.Div(new(Dec).Add(new(Dec).Mul(&closePrice, New(2)), new(Dec).Mul(&prevEMAL, New(periodL-1))), New(periodL+1), 7)
			EMAS := Macd[i].EMAS
			EMAL := Macd[i].EMAL
			Macd[i].Dif.Sub(&EMAS, &EMAL).Round(7)
			dif := Macd[i].Dif // Dec has some bug on Mul so duplicate the mutiplier
			Macd[i].Dea.Div(new(Dec).Add(new(Dec).Mul(&dif, New(2)), new(Dec).Mul(&prevDEA, New(periodD-1))), New(periodD+1), 7)
		}
		DIF := Macd[i].Dif
		DEA := Macd[i].Dea
		Macd[i].Macd.Sub(&DIF, &DEA)
		Macd[i].Macd = *new(Dec).Mul(&Macd[i].Macd, New(2)).Round(7)
	}

}

func calcPercentage(v1, v2 Dec, scale uint8) Dec {
	//TODO: zerodiv here
	pct := *new(Dec).Div(&v1, &v2, scale)
	pct = *new(Dec).Mul(&pct, New(100))
	pct.Round(DECIMAL_PCT)
	return pct
}

func parseMktData(MktdataDailySeq []MktdataDaily, Mktdata []MktEqud, ticker_exchange string) {
	for j := range MktdataDailySeq {
		var (
			openPrice, closePrice, preClosePrice, highestPrice,
			lowestPrice, priceChange, priceChangePct, turnoverVol, turnoverValue Dec
		)
		openPrice.SetFloat64(Mktdata[j].OpenPrice)
		closePrice.SetFloat64(Mktdata[j].ClosePrice)
		preClosePrice.SetFloat64(Mktdata[j].PreClosePrice)
		highestPrice.SetFloat64(Mktdata[j].HighestPrice)
		lowestPrice.SetFloat64(Mktdata[j].LowestPrice)
		priceChange.Sub(&closePrice, &preClosePrice)
		priceChangePct = calcPercentage(priceChange, preClosePrice, DECIMAL_PCT+2)
		turnoverVol.SetFloat64(Mktdata[j].TurnoverVol)
		turnoverValue.SetFloat64(Mktdata[j].TurnoverValue)
		//Mon Jan 2 15:04:05 -0700 MST 2006
		datetime, _ := time.Parse("2006-01-02", Mktdata[j].TradeDate)

		MktdataDailySeq[j].time = datetime.UnixNano() / 1e6
		MktdataDailySeq[j].ticker_exchange = ticker_exchange // "ticker.exchange",
		MktdataDailySeq[j].dataDate = Mktdata[j].TradeDate   // "dataDate",
		MktdataDailySeq[j].openPrice = openPrice             // "openPrice",
		MktdataDailySeq[j].closePrice = closePrice           // "closePrice",
		MktdataDailySeq[j].preClosePrice = preClosePrice     // "preClosePrice",
		MktdataDailySeq[j].highestPrice = highestPrice       // "highestPrice",
		MktdataDailySeq[j].lowestPrice = lowestPrice         // "lowestPrice",
		MktdataDailySeq[j].priceChange = priceChange         // "price_change",
		MktdataDailySeq[j].priceChangePct = priceChangePct   // "price_change_percentage",
		MktdataDailySeq[j].volume = turnoverVol              // "volume",
		MktdataDailySeq[j].ammount = turnoverValue           // "ammount"
	}
}

func correctMktData(beforeCorr []MktdataDaily, afterCorr []MktdataDaily,
	Mktdata []MktEqud) {
	ticker_exchange := beforeCorr[0].ticker_exchange
	factors := make([]Dec, len(Mktdata))
	for i, _ := range factors {
		factors[i].SetInt64(1)
	}
	//calculate factor
	for j := range Mktdata {
		var (
			preClosePrice, actPrevClose Dec
		)

		if Mktdata[j].PreClosePrice != Mktdata[j].ActPreClosePrice && j > 0 {
			var factor Dec
			preClosePrice.SetFloat64(Mktdata[j].PreClosePrice) //"preClosePrice",
			actPrevClose.SetFloat64(Mktdata[j].ActPreClosePrice)
			factor.Div(&preClosePrice, &actPrevClose, 7)
			for k := range afterCorr[:j] {
				factors[k] = *new(Dec).Mul(&factors[k], &factor).Round(7)
			}
		}
	}

	for j := range afterCorr {
		//suspention
		if Mktdata[j].OpenPrice == 0 {
			var preClosePrice Dec
			datetime, _ := time.Parse("2006-01-02", Mktdata[j].TradeDate)
			preClosePrice.SetFloat64(Mktdata[j].PreClosePrice)
			afterCorr[j].time = datetime.UnixNano() / 1e6
			afterCorr[j].ticker_exchange = ticker_exchange // "ticker.exchange",
			afterCorr[j].dataDate = Mktdata[j].TradeDate   // "dataDate",
			afterCorr[j].openPrice = preClosePrice         // "openPrice",
			afterCorr[j].closePrice = preClosePrice        // "closePrice",
			afterCorr[j].preClosePrice = preClosePrice     // "preClosePrice",
			afterCorr[j].highestPrice = preClosePrice      // "highestPrice",
			afterCorr[j].lowestPrice = preClosePrice       // "lowestPrice",
			afterCorr[j].priceChange = *New(0)             // "price_change",
			afterCorr[j].priceChangePct = *New(0)          // "price_change_percentage",
			afterCorr[j].volume = *New(0)                  // "volume",
			afterCorr[j].ammount = *New(0)                 // "ammount"
		} else {
			afterCorr[j] = beforeCorr[j]
		}
		afterCorr[j].openPrice = *new(Dec).Mul(&afterCorr[j].openPrice, &factors[j]).Round(2)
		afterCorr[j].closePrice = *new(Dec).Mul(&afterCorr[j].closePrice, &factors[j]).Round(2)
		closePrice := afterCorr[j].closePrice
		if j == 0 {
			afterCorr[j].preClosePrice = *new(Dec).Mul(&afterCorr[j].preClosePrice, &factors[j]).Round(2)
		} else {
			afterCorr[j].preClosePrice = afterCorr[j-1].preClosePrice //prevClose
		}
		preClosePrice := afterCorr[j].preClosePrice

		afterCorr[j].highestPrice = *new(Dec).Mul(&afterCorr[j].highestPrice, &factors[j]).Round(2)
		afterCorr[j].lowestPrice = *new(Dec).Mul(&afterCorr[j].lowestPrice, &factors[j]).Round(2)

		afterCorr[j].priceChange.Sub(&closePrice, &preClosePrice)
		afterCorr[j].priceChangePct = calcPercentage(afterCorr[j].priceChange, preClosePrice, DECIMAL_PCT+2)
	}
}

func loadHistdata(mds []Stock, ch chan int) {
	c := GetNewDbClient()
	for i, _ := range mds {
		startTime := time.Now()
		name_mktdata := "mktdata_daily." + mds[i].Ticker_exchange
		name_corrected := "mktdata_daily_corrected." + mds[i].Ticker_exchange
		name_macd := "indicators.macd." + mds[i].Ticker_exchange
		mktdataDaily, err := histdata(mds[i].Ticker_exchange)
		if err != nil {
			Logger.Panic(err)
		}
		if len(mktdataDaily.Data) > 0 {
			days := len(mktdataDaily.Data)
			MktdataDailySeq := make([]MktdataDaily, days)
			MktdataDailySeq_corrected := make([]MktdataDaily, days)
			MacdSeq := make([]Macd, days)
			parseMktData(MktdataDailySeq, mktdataDaily.Data, mds[i].Ticker_exchange)
			correctMktData(MktdataDailySeq, MktdataDailySeq_corrected, mktdataDaily.Data)
			calcMACD(MktdataDailySeq_corrected, MacdSeq)
			PutSeries(c, name_mktdata, columns_mktdata_daily[:], MktdataDaily2Pnts(MktdataDailySeq))
			PutSeries(c, name_corrected, columns_mktdata_daily[:], MktdataDaily2Pnts(MktdataDailySeq_corrected))
			PutSeries(c, name_macd, columns_macd[:], Macd2Pnts(MacdSeq))
		} else {
			Logger.Printf("No Data for %s\n", mds[i].Ticker_exchange)
		}
		endTime := time.Now()
		Logger.Printf("%s Done | duration %s | %d to go.\n", mds[i].Ticker_exchange, endTime.Sub(startTime), len(mds)-i-1)
		if DEBUGMODE && i > (debuggonumber-1) {
			break
		}
		mds[i].Done = true
	}
	ch <- 1
}

func Macd2Pnts(MacdSeq []Macd) [][]interface{} {
	var points [][]interface{}
	points = make([][]interface{}, len(MacdSeq))
	if len(MacdSeq) == 0 {
		return nil
	}
	for j, _ := range MacdSeq {
		points[j] = []interface{}{
			MacdSeq[j].time,
			MacdSeq[j].ticker_exchange,
			MacdSeq[j].dataDate,
			MacdSeq[j].Dif.Float64(),  // "DIF",  // "EMA12-EMA26",
			MacdSeq[j].Dea.Float64(),  // "DEA",  // "EMA(DIF,9)",
			MacdSeq[j].Macd.Float64(), // "MACD", // "(DIF-DEA)*2"
		}
	}
	return points
}
func MktdataDaily2Pnts(MktdataDailySeq []MktdataDaily) [][]interface{} {
	var points [][]interface{}
	points = make([][]interface{}, len(MktdataDailySeq))
	if len(MktdataDailySeq) == 0 {
		return nil
	}
	for j, _ := range MktdataDailySeq {
		points[j] = []interface{}{
			MktdataDailySeq[j].time,
			MktdataDailySeq[j].ticker_exchange,
			MktdataDailySeq[j].dataDate,
			MktdataDailySeq[j].openPrice.Float64(),
			MktdataDailySeq[j].closePrice.Float64(),
			MktdataDailySeq[j].preClosePrice.Float64(),
			MktdataDailySeq[j].highestPrice.Float64(),
			MktdataDailySeq[j].lowestPrice.Float64(),
			MktdataDailySeq[j].priceChange.Float64(),
			MktdataDailySeq[j].priceChangePct.Float64(),
			MktdataDailySeq[j].volume.Float64(),
			MktdataDailySeq[j].ammount.Float64(),
		}
	}
	return points
}

func main() {
	// ch := make(chan int)
	// var tst []Stock
	// tst = append(tst, Stock{"600133.XSHG", 0, "", ""})
	// go loadHistdata(tst, ch)
	// <-ch
	Main()
}

//startTime := time.Now()
//endTime := time.Now()
//Printf("[duration %s]\n", endTime.Sub(startTime))
