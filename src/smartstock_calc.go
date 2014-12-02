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
	isHit func(*Metrics) bool
}
type Refdata struct {
	ticker_exchange string
	dataTime        string // "dataTime",
	dataDate        string // "dataTime",
	lasttime        int64

	shortName     string
	tradableQty   Dec
	currency      string
	criterias     []Criteria
	closePriceSeq [20]Dec
	cpsum4        Dec
	cpsum9        Dec
	cpsum19       Dec
	volSeq        [20]Dec
	volsum5       Dec
	volsum10      Dec
	prevEMAL      Dec
	prevEMAS      Dec
	prevDEA       Dec
	lastTradeDate string
	isActive      bool
	isQualified   bool // some stock has less data than needed
	isAlertRaised bool
	Metrics       Metrics
	AlertMsg      string
}
type Alert struct {
	criteriaHit string // "criteriaHit"
}

var columns_alert = [...]string{
	"ticker.exchange",
	"dataDate",
	"dataTime",
	"criteriaHit",
}

type Metrics struct {
	X1_1 Dec  // "X1-1", Volume Ratio 5d
	X1_2 Dec  // "X1-2", Volume Ratio 10d
	X2   Dec  // "X2", PrcChgPct
	Y1   bool // "Y1", MA5>=MA10>=MA20
	Y2   bool // "Y2", MA5<=MA10<=MA20
	X3   Dec  // "X3", abs(MACD)
	X4   Dec  // "X4", tradableQty * Prc
}

var columns_metrics = [...]string{
	"dataDate",
	"dataTime",
	"X1-1",
	"X1-2",
	"X2",
	"Y1",
	"Y2",
	"X3",
	"X4",
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

var (
	periodS int64 = 12
	periodL int64 = 26
	periodD int64 = 9
)

var Ref []Refdata

func isHitCriteria_1(m *Metrics) bool {
	var macdx Dec
	macdx.SetFloat64(0.2)
	return (*m).X1_2.Cmp(New(2)) > 0 &&
		(*m).X2.Cmp(New(3)) > 0 &&
		(*m).Y1 &&
		(*m).X3.Cmp(&macdx) < 0 &&
		(*m).X4.Cmp(New(500000)) < 0
}
func isHitCriteria_2(m *Metrics) bool {
	var macdx Dec
	macdx.SetFloat64(0.2)
	return (*m).X1_1.Cmp(New(2)) > 0 &&
		(*m).X2.Cmp(New(-3)) < 0 &&
		!(*m).Y1 &&
		!(*m).Y2 &&
		(*m).X3.Cmp(&macdx) < 0 &&
		(*m).X4.Cmp(New(500000)) < 0
}

func init() {
	Ref = make([]Refdata, STOCKCOUNT)
	if !DEBUGMODE {
		SetGoInf()
	}
	SetProcess(Goproc{loadRefData, "Loading RefData..."})
	SetProcess(Goproc{calcRealTimeMktData, "Continuously Monitor the Markets ..."})
	//DBdropShards([]string{"metrics", "alerts"})
	DBdropShards([]string{"metrics"})
}
func getRefdataDB(ticker string, Idx int) (Refdata, error) {
	var ref Refdata
	ref.isQualified = false
	ref.isAlertRaised = false
	ref.lasttime = 0
	// ..........m(_._)m
	datetime, _ := time.Parse("2006-01-02 MST -0700",
		time.Now().String()[:10]+" GMT +0800")
	// ..........m(_._)m
	timeInt := datetime.UnixNano()
	ref.lasttime = timeInt

	query := fmt.Sprintf("select dataDate,closePrice,volume "+
		"from mktdata_daily_corrected.%s where time < %d limit 19 order desc", ticker, timeInt)
	series, err := c.Query(query)
	if err != nil {
		SetStockStatus(Idx, STATUS_ERROR, "Call DB ERROR: "+err.Error())
		//Logger.Panic(err)
		Logger.Println(query + "\nNo Data")
		return ref, errors.New("No Data")
	}
	if len(series) == 0 {
		Logger.Println(query + "\nNo Data")
		return ref, errors.New("No Data")
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
	ref.ticker_exchange = ticker
	// ref.shortName = "undefined"
	// ref.tradableQty
	// ref.currency
	ref.criterias = DefaultCriterias
	for i, p := range points {
		f, ok := p[idxClosePrice].(float64)
		if !ok {
			Logger.Println("invalid prc")
			SetStockStatus(Idx, STATUS_ERROR, "Invalid ClosePrice ")
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
		switch i + 1 {
		case 4:
			ref.cpsum4 = sum
		case 9:
			ref.cpsum9 = sum
		case 19:
			ref.cpsum19 = sum
			break
		default:
		}
	}
	for i, p := range points {
		f, ok := p[idxVolume].(float64)
		if !ok {
			Logger.Println("invalid volume")
			SetStockStatus(Idx, STATUS_ERROR, "Invalid Volume ")
			return ref, errors.New("invalid volume")
		}
		ref.volSeq[i].SetFloat64(f)
	}

	sum = *New(0)
	for i, val := range ref.volSeq {
		sum.Add(&sum, &val)
		switch i + 1 {
		case 5:
			ref.volsum5 = sum
		case 10:
			ref.volsum10 = sum
			break
		default:
		}
	}

	s, ok := points[0][idxDataDate].(string)
	if !ok {
		Logger.Println("invalid date")
		SetStockStatus(Idx, STATUS_ERROR, "Invalid Date ")
		return ref, errors.New("invalid date")
	}

	ref.lastTradeDate = s
	ref.isActive = false

	query = fmt.Sprintf("select EMAL,EMAS,DEA "+
		"from indicators.macd.%s limit 1", ticker)
	series, err = c.Query(query)
	if err != nil {
		Logger.Panic(err)
	}
	if len(series) == 0 {
		Logger.Panic(err)
	}
	columns = series[0].GetColumns()
	points = series[0].GetPoints()
	if len(points) == 0 {
		Logger.Println("Data Error! No MACD")
		SetStockStatus(Idx, STATUS_ERROR, "\nData Error!\nNo MACD")
		return ref, errors.New("Data Error! No MACD")
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
		SetStockStatus(Idx, STATUS_ERROR, "Data Error! No prevEMAL")
		return ref, errors.New("invalid prevEMAL")
	}
	ref.prevEMAL.SetFloat64(f)

	f, ok = points[0][idxEMAS].(float64)
	if !ok {
		Logger.Println("invalid prevEMAL")
		SetStockStatus(Idx, STATUS_ERROR, "Data Error! No prevEMAL")
		return ref, errors.New("invalid prevEMAL")
	}
	ref.prevEMAS.SetFloat64(f)

	f, ok = points[0][idxDEA].(float64)
	if !ok {
		Logger.Println("invalid prevEMAL")
		SetStockStatus(Idx, STATUS_ERROR, "Data Error! No prevEMAL")
		return ref, errors.New("invalid prevEMAL")
	}
	ref.prevDEA.SetFloat64(f)
	ref.isQualified = true
	return ref, nil
}

func getRefdataDataAPI(ticker string, Idx int, date string) (MktEqudRefslice, error) {
	var refdata MktEqudRefslice
	retry := APIMAXRETRY
	ok := false
	//1 for success , -1 for no data , other retry
	api := "getMktEqud.json"
	ts := strings.Split(ticker, ".")

	if len(ts) > 1 {
		if ts[1] == "XHKG" {
			api = "getMktHKEqud.json"
		}
	}
	date = strings.Join(strings.Split(date, "-"), "")
	for !ok && retry > 0 {
		body, err := CallDataAPI(
			"market",
			"1.0.0",
			api,
			[]string{
				"secID=" + ticker,
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
			Logger.Printf("Fetch OK but no Data %s : %d - %s \n", ticker, refdata.RetCode, refdata.RetMsg)
			fallthrough
		case 1:
			SetStockStatus(Idx, STATUS_RUNNING, "Call DataAPI OK")
			ok = true
		default:
			SetStockStatus(Idx, STATUS_RETRYING, "Call DataAPI Failed Retry ...")
			retry--
			time.Sleep(100 * time.Millisecond)
			Logger.Printf("%s\n", string(body))
			Logger.Printf("Fetch Failed %s : %d - %s | RetryRemain = %d ..\n", ticker, refdata.RetCode, refdata.RetMsg, retry)
		}
	}
	if retry == 0 {
		return refdata, errors.New("Failed calling DataAPI...")
	}
	return refdata, nil
}

var c = GetNewDbClient()

func loadRefData(mds []Stock, ch chan int) {
	// c := GetNewDbClient()
	var err error
	var res MktEqudRefslice
	for i := range mds {

		if DEBUGMODE && i >= 2 {
			//Logger.Println(*pRef)
			break
		}
		Idx := mds[i].Idx
		StartProcess(Idx)
		var pRef *Refdata = &Ref[Idx]
		*pRef, err = getRefdataDB(mds[i].Ticker_exchange, Idx)
		if err != nil {
			Logger.Println(err)
			SetStockStatus(Idx, STATUS_ERROR, "Not enough data! getRefdataDB")
			continue
		}
		res, err = getRefdataDataAPI(mds[i].Ticker_exchange, Idx, pRef.lastTradeDate)
		if err != nil {
			Logger.Println(err)
			SetStockStatus(Idx, STATUS_ERROR, "Not enough data! getRefdataDataAPI")
			continue
		}
		if len(res.Data) == 0 {
			Logger.Println("No data from API")
			SetStockStatus(Idx, STATUS_ERROR, "Not enough data! No data from getRefdataDataAPI")
			continue
		}
		pRef.currency = res.Data[0].TransCurrCD
		pRef.shortName = res.Data[0].SecShortName
		var NegMv, preClose Dec
		NegMv.SetFloat64(res.Data[0].NegMarketValue)
		if pRef.closePriceSeq[0].Cmp(New(0)) > 0 {
			preClose = pRef.closePriceSeq[0]
			pRef.tradableQty = *new(Dec).Div(&NegMv, &preClose, 0)
		} else {
			pRef.isQualified = false
		}
		if pRef.isQualified {
			SetStockStatus(Idx, STATUS_DONE, "GetRef OK.")
		} else {
			SetStockStatus(Idx, STATUS_ERROR, "Not enough data! no PrevClose")
		}

	}
	ch <- 1
}

func calcRealTimeMktData(mds []Stock, ch chan int) {
	// c := GetNewDbClient()
	for {
		for i := range mds {
			if DEBUGMODE && i >= 2 {
				Logger.Printf("OK.  \n")
				break
			}
			var idx = mds[i].Idx
			var pRef = &Ref[idx]
			if pRef.isQualified {
				if !pRef.isAlertRaised {
					StartProcess(idx)
					if HaveAlerts(idx) {
						SetStockStatus(idx, STATUS_DONE, "Alert"+pRef.AlertMsg)
					} else {
						SetStockStatus(idx, STATUS_READY, "Standby\nLstTime:"+Ref[idx].dataTime)
					}
				}
			} else {
				SetStockStatus(idx, STATUS_ERROR, "Not enough data!")
			}
		}
		// if DEBUGMODE {
		// 	break
		// }

		time.Sleep(1000 * time.Millisecond)
	}
	ch <- 1
}
func HaveAlerts(Idx int) bool {
	var pRef = &Ref[Idx]
	var haveAlerts bool = false

	query := fmt.Sprintf("select dataDate,dataTime,lastPrice,price_change_percentage,volume "+
		"from mktdata.%s where time > %d", pRef.ticker_exchange, pRef.lasttime)

	if DEBUGMODE {
		Logger.Println(query)
	}
	series, err := c.Query(query)
	if err != nil {
		Logger.Println(err)
	}
	if len(series) == 0 {
		Logger.Printf("%s No data", pRef.ticker_exchange)
		return false
	}

	columns := series[0].GetColumns()
	points := series[0].GetPoints()
	if DEBUGMODE {
		Logger.Println(len(points), " records Read")
	}
	if len(points) == 0 {
		Logger.Printf("%s No data", pRef.ticker_exchange)
		return false
	}

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
	TotalMinute := TotalMinute()
	ok := true
	f, ok := points[0][idxtime].(float64)
	if !ok {
		Logger.Panic("No lasttime")
	}
	pRef.lasttime = int64(f) * 1e6
loopMktdata:

	for _, p := range points {
		m := &pRef.Metrics
		var volume, lstprice, prcChg float64
		var volDec, prcDec Dec
		pRef.dataTime, _ = p[idxdataTime].(string)
		pRef.dataDate, _ = p[idxdataDate].(string)
		volume, _ = p[idxVol].(float64)
		lstprice, _ = p[idxlastPrice].(float64)
		prcChg, _ = p[idxPriceChgPct].(float64)
		volDec.SetFloat64(volume)
		MinuteFromOpen := getMinuteFromOpen(pRef.dataTime)
		(*m).X1_1 = calcX1_1(&volDec, &pRef.volsum5, &MinuteFromOpen, &TotalMinute)
		(*m).X1_2 = calcX1_2(&volDec, &pRef.volsum10, &MinuteFromOpen, &TotalMinute)

		(*m).X2.SetFloat64(prcChg)

		prcDec.SetFloat64(lstprice)
		MA5 := *new(Dec).Div(new(Dec).Add(&prcDec, &pRef.cpsum4), New(5), DECIMAL_PRC)
		MA10 := *new(Dec).Div(new(Dec).Add(&prcDec, &pRef.cpsum9), New(10), DECIMAL_PRC)
		MA20 := *new(Dec).Div(new(Dec).Add(&prcDec, &pRef.cpsum19), New(20), DECIMAL_PRC)
		(*m).Y1 = MA5.Cmp(&MA10) >= 0 && MA10.Cmp(&MA20) >= 0
		(*m).Y2 = MA5.Cmp(&MA10) <= 0 && MA10.Cmp(&MA20) <= 0
		(*m).X3 = calcX3(&prcDec,
			&pRef.prevEMAS,
			&pRef.prevEMAL,
			&pRef.prevDEA)
		prcDec.SetFloat64(lstprice)
		(*m).X4 = *new(Dec).Div(new(Dec).Mul(&prcDec, &pRef.tradableQty), New(10000), 0)

		// PutSeries(c, "metrics."+pRef.ticker_exchange, columns_metrics[:],
		// 	Metrics2Pnts(Idx, []Metrics{pRef.Metrics}))
		for i := range pRef.criterias {
			if pRef.criterias[i].isHit(m) {
				genAlert(Idx, &pRef.criterias[i], m,
					[]string{
						"Prc", prcDec.Round(3).String(),
						"Vol", volDec.Round(0).String(),
						"MA5", MA5.Round(3).String(),
						"MA10", MA10.Round(3).String(),
						"MA20", MA20.Round(3).String()})
				haveAlerts = true
				pRef.isAlertRaised = true
				break loopMktdata
			}
		}
	}
	// pRef.lasttime =
	return haveAlerts
}

func genAlert(Idx int, cri *Criteria, m *Metrics, params []string) {
	var alert Alert
	alert.criteriaHit = Ref[Idx].shortName + "\n@" + Ref[Idx].dataTime + ":" + (*cri).name
	alert.criteriaHit += fmt.Sprintf(" X11:%.2f X12:%.2f X2:%.2f X3:%.3f X4:%.0f Y1:%s Y2:%s ",
		(*m).X1_1.Float64(),
		(*m).X1_2.Float64(),
		(*m).X2.Float64(),
		(*m).X3.Float64(),
		(*m).X4.Float64(),
		fmt.Sprint((*m).Y1), fmt.Sprint((*m).Y2))
	alert.criteriaHit += "\n"
	for i := 0; i+1 < len(params); i += 2 {
		alert.criteriaHit += fmt.Sprintf(" %s:%s", params[i], params[i+1])
	}
	Ref[Idx].AlertMsg = alert.criteriaHit

	PutSeries(c, "alerts", columns_alert[:], Alert2Pnts(Idx, []Alert{alert}))
}

func calcX1_1(volume, volsum5, MinuteFromOpen, TotalMinute *Dec) Dec {
	// X1_1     Dec    // "X1-1", Volume Ratio 5d
	if (*MinuteFromOpen).Cmp(New(0)) == 0 ||
		(*volsum5).Cmp(New(0)) == 0 {
		return *New(0)
	}
	return *new(Dec).Div(new(Dec).Mul(TotalMinute, volume),
		new(Dec).Mul(MinuteFromOpen, new(Dec).Div(volsum5, New(5), 7)), 7)
}

func calcX1_2(volume, volsum10, MinuteFromOpen, TotalMinute *Dec) Dec {
	// X1_2     Dec    // "X1-2", Volume Ratio 10d
	if (*MinuteFromOpen).Cmp(New(0)) == 0 ||
		(*volsum10).Cmp(New(0)) == 0 {
		return *New(0)
	}
	return *new(Dec).Div(new(Dec).Mul(TotalMinute, volume),
		new(Dec).Mul(MinuteFromOpen, new(Dec).Div(volsum10, New(10), 7)), 7)
}

func calcX3(lstprc, prevEMAS, prevEMAL, prevDEA *Dec) Dec {
	var EMAS, EMAL, Dif, Dea, Macd Dec
	EMAS.Div(new(Dec).Add(new(Dec).Mul(lstprc, New(2)), new(Dec).Mul(prevEMAS, New(periodS-1))), New(periodS+1), 7)
	EMAL.Div(new(Dec).Add(new(Dec).Mul(lstprc, New(2)), new(Dec).Mul(prevEMAL, New(periodL-1))), New(periodL+1), 7)
	Dif.Sub(&EMAS, &EMAL).Round(7)
	Dea.Div(new(Dec).Add(new(Dec).Mul(&Dif, New(2)), new(Dec).Mul(prevDEA, New(periodD-1))), New(periodD+1), 7)
	Macd.Sub(&Dif, &Dea)
	return *new(Dec).Abs(new(Dec).Mul(&Macd, New(2)).Round(7))
}

var TradeTimeWindows []string = []string{"09:30", "11:30", "13:00", "15:00"}

func TotalMinute() Dec {
	var v int64

	for i := 0; i < len(TradeTimeWindows)-1; i = i + 2 {
		t1, _ := time.Parse("15:04 MST -0700", TradeTimeWindows[i]+" GMT +0800")
		t2, _ := time.Parse("15:04 MST -0700", TradeTimeWindows[i+1]+" GMT +0800")
		v += int64(t2.Sub(t1).Minutes())
	}
	return *New(v)
}
func getMinuteFromOpen(t string) Dec {
	var v int64
	tt, _ := time.Parse("15:04:05 MST -0700", t+" GMT +0800")
	if len(TradeTimeWindows) < 2 {
		return *New(0)
	}
	tBgnTrd, _ := time.Parse("15:04 MST -0700", TradeTimeWindows[0]+" GMT +0800")
	tEndTrd, _ := time.Parse("15:04 MST -0700", TradeTimeWindows[len(TradeTimeWindows)-1]+" GMT +0800")
	if tt.Before(tBgnTrd) {
		return *New(0)
	} else if tt.Before(tEndTrd) {
		for i := 0; i < len(TradeTimeWindows)-1; i = i + 2 {
			t1, _ := time.Parse("15:04 MST -0700", TradeTimeWindows[i]+" GMT +0800")
			t2, _ := time.Parse("15:04 MST -0700", TradeTimeWindows[i+1]+" GMT +0800")
			if tt.Equal(t1) {
				v++
				break
			}
			if tt.Equal(t2) {
				v += int64(t2.Sub(t1).Minutes())
				break
			}
			if tt.Before(t2) &&
				tt.After(t1) {
				v += int64(tt.Sub(t1).Minutes())
				break
			} else {
				v += int64(t2.Sub(t1).Minutes())
			}
		}
	} else {
		return TotalMinute()
	}
	return *New(v)
}

func Alert2Pnts(Idx int, alerts []Alert) [][]interface{} {
	var pRef = &Ref[Idx]
	var points [][]interface{}
	points = make([][]interface{}, len(alerts))
	// datetime, _ := time.Parse("2006-01-02 15:04:05",
	// 	pRef.dataDate+" "+pRef.dataTime)
	// var timeInt int64 = datetime.UnixNano() / 1e6
	for i := range alerts {
		points[i] = []interface{}{
			pRef.ticker_exchange,
			pRef.dataDate,
			pRef.dataTime,
			alerts[i].criteriaHit,
		}
	}
	if DEBUGMODE {
		Logger.Println("AlertP:", points)
	}
	return points
}
func Metrics2Pnts(Idx int, metrics []Metrics) [][]interface{} {
	var pRef = &Ref[Idx]
	var points [][]interface{}
	points = make([][]interface{}, len(metrics))
	// datetime, _ := time.Parse("2006-01-02 15:04:05",
	// 	pRef.dataDate+" "+pRef.dataTime)
	// var timeInt int64 = datetime.UnixNano() / 1e6
	if DEBUGMODE {
		Logger.Println(metrics)
	}
	for i := range metrics {
		points[i] = []interface{}{
			pRef.dataDate,
			pRef.dataTime,
			metrics[i].X1_1.Float64(), // X1_1 Dec  // "X1-1", Volume Ratio 5d
			metrics[i].X1_2.Float64(), // X1_2 Dec  // "X1-2", Volume Ratio 10d
			metrics[i].X2.Float64(),   // X2   Dec  // "X2", PrcChgPct
			fmt.Sprint(metrics[i].Y1), // Y1   bool // "Y1", MA5>=MA10>=MA20
			fmt.Sprint(metrics[i].Y2), // Y2   bool // "Y2", MA5<=MA10<=MA20
			metrics[i].X3.Float64(),   // X3   Dec  // "X3", abs(MACD)
			metrics[i].X4.Float64(),   // X4   Dec  // "X4", tradableQty * Prc
		}
	}
	if DEBUGMODE {
		Logger.Println("Metric:", points)
	}
	return points
}

func main() {

	Main()
}
