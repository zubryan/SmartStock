package framework

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	. "github.com/dimdin/decimal"
	"github.com/influxdb/influxdb/client"
	"github.com/larspensjo/config"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	configFile = flag.String("configfile", "config.ini", "General configuration file")
)

var selected int = -1
var mouseX int = -1
var mouseY int = -1

const (
	STATUS_READY = iota
	STATUS_RUNNING
	STATUS_RETRYING
	STATUS_DONE
	STATUS_ERROR
)

var StatusVal = map[uint8]string{
	STATUS_READY:    "READY   ",
	STATUS_RUNNING:  "RUNNING ",
	STATUS_RETRYING: "RETRYING",
	STATUS_DONE:     "DONE    ",
	STATUS_ERROR:    "ERROR   ",
}

type Stock struct {
	Ticker_exchange string //  "ticker.exchange",
	Idx             int    // "index in the Mktdatas"
	DataDate        string // "dataDate",
	DataTime        string // "dataDate"
	ProcessStart    time.Time
	Status          uint8
	Msg             string
}

const (
	DECIMAL_PRC = 5 // decimal of price/money
	DECIMAL_QTY = 3 // decimal of qty/volume
	DECIMAL_PCT = 3 // decimal of percentage Decs
	APIMAXRETRY = 10
)

var (
	APICONF         = make(map[string]string)
	DBCONF          = make(map[string]string)
	GROUPMOD        = 100
	DEBUGMODE       = false
	LOGFILE         = "smart.log"
	LOGOPTS         = log.LstdFlags
	Stockfile       string
	STOCKCOUNT      int = 0
	termwidth       int = 80
	Mktdatas        []Stock
	Processes       []Goproc
	Logger          *log.Logger
	loggerFW        *log.Logger
	logfile         *os.File
	goInf           bool = false
	parallelrunDone bool
	showmonitor     bool = true
)

type doGo func([]Stock, chan int)

type Goproc struct {
	DoGo doGo
	Desc string
}

var title string = "Initializing..."

func DBdropShards(shardsToDrop []string) {
	c := GetNewDbClient()
	// drop ShardSpace instead of droping series which is mu......ch slower~~~
	ssps, _ := c.GetShardSpaces()
	for _, ssp := range ssps {
		if ssp.Database == DBCONF["database"] {
			for _, shardtodrop := range shardsToDrop {
				if ssp.Name == shardtodrop {
					loggerFW.Printf("Rebuild ShardSpace %s\n", ssp.Name)
					c.DropShardSpace(DBCONF["database"], ssp.Name)
					c.CreateShardSpace(DBCONF["database"], ssp)
				}
			}
		}
	}
}
func CalcPercentage(v1, v2 Dec, scale uint8) Dec {
	//TODO: zerodiv here
	pct := *new(Dec).Div(&v1, &v2, scale)
	pct = *new(Dec).Mul(&pct, New(100))
	pct.Round(DECIMAL_PCT)
	return pct
}
func GetNewDbClient() *client.Client {
	c, err := client.NewClient(&client.ClientConfig{
		Host:     DBCONF["host"],
		Username: DBCONF["username"],
		Password: DBCONF["password"],
		Database: DBCONF["database"],
	})
	if err != nil {
		loggerFW.Panic(err)
	}
	return c
}
func ReplaceSeries(c *client.Client, name string, columns []string, points [][]interface{}) {
	c.Query("drop series " + name)
	PutSeries(c, name, columns, points)
}

func PutSeries(c *client.Client, name string, columns []string, points [][]interface{}) {
	retry := APIMAXRETRY
	ok := false
	var err error
	for ok == false && retry > 0 {
		series := &client.Series{
			Name:    name,
			Columns: columns,
			Points:  points,
		}
		err = c.WriteSeries([]*client.Series{series})
		if err != nil {
			loggerFW.Println("Cannot Insert")
			loggerFW.Println(points)
			retry--
		}
		loggerFW.Printf("INFLUXDB: %d record(s) added to %s", len(series.Points), series.Name)
		ok = true
		return
	}
	loggerFW.Panic(err)
}

func SetProcess(process Goproc) {
	Processes = append(Processes, process)
}

func init() {
	initCfg()
	initLogger()
	initDB()
	loggerFW.Println("[FRAMEWORK]Preparing...")

	initStocklist()
}
func initDB() {
	c, err := client.NewClient(&client.ClientConfig{
		Host:     DBCONF["host"],
		Username: DBCONF["username"],
		Password: DBCONF["password"],
	})
	if err != nil {
		loggerFW.Panic(err)
	}
	dbs, _ := c.GetDatabaseList()
	dbexists := false
	for _, db := range dbs {
		if db["name"] == DBCONF["database"] {
			dbexists = true
		}
	}
	if !dbexists {
		// create schema
		loggerFW.Println("Reconstruct DB")
		c.CreateDatabase(DBCONF["database"])
		c.CreateShardSpace(DBCONF["database"], &client.ShardSpace{"mktdata_daily", DBCONF["database"], "/mktdata_daily.*/", "inf", "10000d", 1, 1})
		c.CreateShardSpace(DBCONF["database"], &client.ShardSpace{"mktdata", DBCONF["database"], "/mktdata\\..*/", "inf", "7d", 1, 1})
		c.CreateShardSpace(DBCONF["database"], &client.ShardSpace{"metrics", DBCONF["database"], "/metrics\\..*/", "inf", "7d", 1, 1})
		c.CreateShardSpace(DBCONF["database"], &client.ShardSpace{"alerts", DBCONF["database"], "/alerts/", "inf", "10000d", 1, 1})
		c.CreateShardSpace(DBCONF["database"], &client.ShardSpace{"mktdata", DBCONF["database"], "/mktdata\\..*/", "inf", "7d", 1, 1})
		c.CreateShardSpace(DBCONF["database"], &client.ShardSpace{"indicators", DBCONF["database"], "/indicators\\..*/", "inf", "10000d", 1, 1})
		//	c.CreateShardSpace(DBCONF["database"], &client.ShardSpace{"default", DBCONF["database"], "/.*/", "inf", "30d", 1, 1})
	}
}

func CallDataAPI(api_catagory string, version string, api string, parameters []string) ([]byte, error) {

	var url = APICONF["url"] + "/" + api_catagory + "/" + version + "/" + api + "?" + strings.Join(parameters, "&")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		loggerFW.Panic(err)
	} // fmt.Printf("Fetch %s on %s\n", sec, date)
	req.Header.Add("Authorization", "Bearer "+APICONF["auth"])
	retry := 0
	httpClient := &http.Client{}
	for retry < APIMAXRETRY {
		req.Header.Set("Connection", "close")
		resp, err := httpClient.Do(req)
		if err != nil {
			loggerFW.Println(err)
			time.Sleep(time.Second)
			retry++
			continue
		}
		if resp != nil && resp.StatusCode == 200 {
			loggerFW.Println("Calling API succeed %s", url)
			//loggerFW.Println(resp.Header)
			body, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			//loggerFW.Println("API body %s", body)
			return body, nil
		} else {
			loggerFW.Println("[ERROR] fail calling %s", url)
			time.Sleep(time.Second)
			retry++
		}
	}
	return nil, errors.New("Calling API failed!")

}

func initLogger() {
	var err error
	logfile, err = os.OpenFile(LOGFILE, os.O_RDWR|os.O_APPEND, 0)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}
	Logger = log.New(logfile, "[INFO]", LOGOPTS)
	loggerFW = log.New(logfile, "[FRWK]", LOGOPTS)

}
func initCfg() {

	cfg, err := config.ReadDefault(*configFile)
	if err != nil {
		panic(err)
	}

	GROUPMOD, _ = cfg.Int("GENERAL", "groupmod")
	DEBUGMODE, _ = cfg.Bool("GENERAL", "debugmode")
	showmonitor, _ = cfg.Bool("GENERAL", "monitor")
	LOGFILE, _ = cfg.String("LOGGER", "filename")

	APICONF["url"], _ = cfg.String("API", "url")
	APICONF["market"], _ = cfg.String("API", "market")
	APICONF["version"], _ = cfg.String("API", "version")
	APICONF["auth"], _ = cfg.String("API", "auth")

	DBCONF["host"], _ = cfg.String("DB", "host")
	DBCONF["username"], _ = cfg.String("DB", "username")
	DBCONF["password"], _ = cfg.String("DB", "password")
	DBCONF["database"], _ = cfg.String("DB", "database")

	Stockfile, _ = cfg.String("FILE", "stocklist")
}

func initStocklist() {

	fin, err := os.Open(Stockfile)
	defer fin.Close()
	if err != nil {
		panic(err)
	}
	r := bufio.NewReader(fin)
	idx := 0
	s, err := r.ReadString('\n')
	for err == nil {
		var md Stock
		md.Ticker_exchange = strings.Trim(s, "\n\r")
		md.DataDate = ""
		md.DataTime = ""
		md.Idx = idx
		idx++
		Mktdatas = append(Mktdatas, md)
		s, err = r.ReadString('\n')
	}

	STOCKCOUNT = len(Mktdatas)
}

func tbprint(x, y int, fg, bg termbox.Attribute, msg string) {
	for _, c := range msg {

		termbox.SetCell(x, y, c, fg, bg)
		x += runewidth.NewCondition().RuneWidth(c)
	}
}

func redraw(cntlast int, debugflag string, startTime time.Time, fin_flag bool) int {
	cnt := 0
	runcnt := 0
	errcnt := 0
	retrycnt := 0
	c, r := 0, 0
	emptyline := fmt.Sprintf("%+s", " ", termwidth)
	welcome := "    SMARTSTOCK JOB MONITOR by miuzel : " + title
	termwidth, _ = termbox.Size()
	termbox.Clear(termbox.ColorWhite, termbox.ColorBlack)
	emptyline = fmt.Sprintf("%*c", termwidth, ' ')
	tbprint(0, r, termbox.ColorBlack, termbox.ColorWhite, emptyline)
	tbprint(c, r, termbox.ColorBlack, termbox.ColorWhite, welcome)
	r++
	if selected >= 0 && selected <= STOCKCOUNT && STOCKCOUNT > 0 {
		msg := fmt.Sprintf("%4d:%s[%s %s][ST:%s] %s ",
			Mktdatas[selected].Idx,
			Mktdatas[selected].Ticker_exchange,
			Mktdatas[selected].DataDate,
			Mktdatas[selected].DataTime,
			StatusVal[Mktdatas[selected].Status],
			Mktdatas[selected].Msg) + emptyline + emptyline + emptyline
		if strings.Contains(msg, "\n") {
			var msgt string = ""
			for _, s := range strings.Split(msg, "\n") {
				msgt += fmt.Sprintf("%-*s", termwidth, s)
			}
			msg = msgt
		}
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, emptyline)
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, msg[:termwidth])
		r++
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, emptyline)
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, msg[termwidth:2*termwidth])
		r++
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, emptyline)
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, msg[2*termwidth:3*termwidth])
	} else {
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, emptyline)
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack,
			fmt.Sprintf("           [Left Click to Select]"))
		r++
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, emptyline)
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack,
			fmt.Sprintf("           [Selected: None] X:%d Y:%d", mouseX, mouseY))
		r++
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, emptyline)
	}
	for i := range Mktdatas {
		if i%termwidth == 0 {
			r++
			c = 0
		}
		if i == selected {
			termbox.SetCell(c, r, ' ', termbox.ColorGreen, termbox.ColorGreen)
		} else {
			switch Mktdatas[i].Status {
			case STATUS_DONE:
				termbox.SetCell(c, r, ' ', termbox.ColorWhite, termbox.ColorWhite)
				cnt++
			case STATUS_ERROR:
				termbox.SetCell(c, r, 'x', termbox.ColorWhite, termbox.ColorRed)
				errcnt++
			case STATUS_READY:
				termbox.SetCell(c, r, ' ', termbox.ColorWhite, termbox.ColorMagenta)
			case STATUS_RUNNING:
				termbox.SetCell(c, r, '>', termbox.ColorWhite, termbox.ColorCyan)
				runcnt++
			case STATUS_RETRYING:
				termbox.SetCell(c, r, 'r', termbox.ColorWhite, termbox.ColorYellow)
				retrycnt++
			}
		}

		c++
	}
	termbox.SetCell(c, r, ' ', termbox.ColorBlack, termbox.ColorBlack)
	r++
	tbprint(0, r, termbox.ColorBlack, termbox.ColorWhite, emptyline)
	tbprint(0, r, termbox.ColorBlack, termbox.ColorWhite,
		fmt.Sprintf(" RUNNING[ ]%5d | ERROR  [ ]%5d | RETRY  [ ]%5d | ",
			runcnt, errcnt, retrycnt))
	termbox.SetCell(9, r, '>', termbox.ColorWhite, termbox.ColorCyan)
	termbox.SetCell(10, r, ']', termbox.ColorBlack, termbox.ColorWhite)
	termbox.SetCell(27, r, 'x', termbox.ColorWhite, termbox.ColorRed)
	termbox.SetCell(28, r, ']', termbox.ColorBlack, termbox.ColorWhite)
	termbox.SetCell(45, r, 'r', termbox.ColorWhite, termbox.ColorYellow)
	termbox.SetCell(46, r, ']', termbox.ColorBlack, termbox.ColorWhite)
	r++
	duration := time.Now().Sub(startTime)
	tbprint(0, r, termbox.ColorBlack, termbox.ColorWhite, emptyline)
	tbprint(0, r,
		termbox.ColorBlack,
		termbox.ColorWhite,
		fmt.Sprintf(" Sum | %5d Stocks |  %5d Done | %3.3f %% ",
			len(Mktdatas),
			cnt,
			float64(cnt)*100/float64(len(Mktdatas))))
	r++
	tbprint(0, r, termbox.ColorBlack, termbox.ColorWhite, emptyline)
	tbprint(0, r,
		termbox.ColorBlack,
		termbox.ColorWhite,
		fmt.Sprintf("     |     +%5d   | elps %5.2fs | estRm %5.2fs",
			int(cnt-cntlast), duration.Seconds(), duration.Seconds()/float64(cnt)*float64(len(Mktdatas)-cnt)))
	r++
	if cnt == len(Mktdatas) || fin_flag {
		tbprint(0, r, termbox.ColorWhite, termbox.ColorBlack, emptyline)
		tbprint(0, r,
			termbox.ColorWhite,
			termbox.ColorBlack,
			fmt.Sprint("   ESC For Exit        [ All Jobs Done! ]                         ", debugflag))
	} else {
		tbprint(0, r, termbox.ColorBlack, termbox.ColorWhite, emptyline)
		tbprint(0, r,
			termbox.ColorBlack,
			termbox.ColorWhite,
			fmt.Sprint("   ESC For Exit                                                   ", debugflag))
	}
	termbox.Flush()
	return cnt
}

func SetGoInf() {
	goInf = true // infinite go
}

func monitor(ch chan int) {
	var cnt int
	debugflag := ""
	if DEBUGMODE {
		debugflag = " DEBUG ON "
	}
	startTime := time.Now()
	for goInf || (cnt < len(Mktdatas) && !parallelrunDone) {
		cnt = redraw(cnt, debugflag, startTime, false)
		time.Sleep(500 * time.Millisecond)
	}
	redraw(cnt, debugflag, startTime, true)
	ch <- 1
}
func SetStockStatus(idx int, status uint8, msg string) {
	Mktdatas[idx].Status = status
	Mktdatas[idx].Msg = msg
}

func StartProcess(idx int) {
	SetStockStatus(idx, STATUS_RUNNING,
		fmt.Sprintf("%s Processing...", Mktdatas[idx].Ticker_exchange))
	Mktdatas[idx].ProcessStart = time.Now()
}
func parallelrun(process Goproc) {
	parallelrunDone = false
	for i := range Mktdatas {
		Mktdatas[i].Status = STATUS_READY
	}
	chs := make(map[int]chan int)
	for i := 0; i*GROUPMOD < STOCKCOUNT; i += 1 {
		var slen int = (i + 1) * GROUPMOD
		if slen > STOCKCOUNT {
			slen = STOCKCOUNT
		}
		chs[i] = make(chan int)
		loggerFW.Printf("[FRAMEWORK]Start from %d to %d\n", i*GROUPMOD+1, slen)
		go process.DoGo(Mktdatas[i*GROUPMOD:slen], chs[i])
		// j := i
	}

	loggerFW.Printf("[FRAMEWORK]Waiting gorts\n")
	for _, ch := range chs {
		<-ch
	}
	parallelrunDone = true
}
func doMouse(ev *termbox.Event) {
	mouseX = ev.MouseX
	mouseY = ev.MouseY
	c := ev.MouseX
	r := ev.MouseY - 4
	selected = r*termwidth + c
}

func termEvent() {
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyEsc:
				welcome := "    SMARTSTOCK JOB MONITOR by miuzel : Finished. Bye"
				tbprint(0, 0, termbox.ColorBlack, termbox.ColorWhite, welcome)
				termbox.Flush()
				for _, x := range Mktdatas {
					loggerFW.Println(x)
				}
				time.Sleep(time.Second)
				termbox.Close()
				os.Exit(0)
				loggerFW.Panic("Cannot exit Console")
			}
		case termbox.EventResize:
			termwidth, _ = termbox.Size()
			welcome := "    Resizing..."
			tbprint(0, 0, termbox.ColorBlack, termbox.ColorWhite, welcome)
		case termbox.EventMouse:
			doMouse(&ev)
			termbox.Flush()
		}
	}
}
func Main() {

	if showmonitor {

		err := termbox.Init()
		if err != nil {
			showmonitor = false
		} else {

			defer termbox.Close()
			termbox.SetInputMode(termbox.InputEsc | termbox.InputMouse)
			termwidth, _ = termbox.Size()
			termbox.Clear(termbox.ColorWhite, termbox.ColorBlack)
			go termEvent()
			// don't exit waiting for ESC
		}
	}
	chm := make(chan int)
	if showmonitor {
		go monitor(chm)
	}
	for i, process := range Processes {
		loggerFW.Printf("[FRAMEWORK]Step %d: %s ...\n", i+1, process.Desc)
		title = process.Desc
		parallelrun(process)
	}
	if showmonitor {
		<-chm
	}

	loggerFW.Println("[FRAMEWORK]Done")
	logfile.Close()
	if showmonitor {
		for {
			time.Sleep(time.Hour) // don't exit wait for ESC
		}
	}
}
