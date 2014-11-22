package framework

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/larspensjo/config"
	"math"
	"os"
	"strings"
)

var (
	configFile = flag.String("configfile", "config.ini", "General configuration file")
)

type Stock struct {
	Ticker_exchange string //  "ticker.exchange",
	Idx             int    // "index in the Mktdatas"
	DataDate        string // "dataDate",
	DataTime        string // "dataDate"
}

var (
	APICONF    = make(map[string]string)
	DBCONF     = make(map[string]string)
	GROUPMOD   = 100
	Stockfile  string
	STOCKCOUNT = 0
	Mktdatas   []Stock
	Processes  []Goproc
)

type doGo func([]Stock, chan int)
type Goproc struct {
	DoGo doGo
	Desc string
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

func SetProcess(process Goproc) {
	Processes = append(Processes, process)
}
func Main() {

	for i, process := range Processes {
		fmt.Printf("[FRAMEWORK]Step %d: %s ...\n", i+1, process.Desc)
		parallelrun(process.DoGo)
	}

	fmt.Println("[FRAMEWORK]Done")
}

func init() {
	fmt.Println("[FRAMEWORK]Preparing...")
	cfg, err := config.ReadDefault(*configFile)
	if err != nil {
		panic(err)
	}

	GROUPMOD, _ = cfg.Int("GENERAL", "groupmod")

	APICONF["url"], _ = cfg.String("API", "url")
	APICONF["market"], _ = cfg.String("API", "market")
	APICONF["version"], _ = cfg.String("API", "version")
	APICONF["auth"], _ = cfg.String("API", "auth")

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
		// fmt.Println(md)
	}
	STOCKCOUNT = len(Mktdatas)
}

func parallelrun(do doGo) {
	chs := make(map[int]chan int)
	for i := 0; i*GROUPMOD < STOCKCOUNT; i += 1 {
		var slen int = (i + 1) * GROUPMOD
		if slen > STOCKCOUNT {
			slen = STOCKCOUNT
		}
		chs[i] = make(chan int)
		go do(Mktdatas[i*GROUPMOD:slen], chs[i])
		// j := i
	}
	fmt.Printf("[FRAMEWORK]Waiting gorts\n")
	for _, ch := range chs {
		<-ch
	}
}
