package ss_framework

import (
	"bufio"
	"strings"
	// "encoding/json"
	"flag"
	"fmt"
	// "github.com/influxdb/influxdb/client"
	"github.com/larspensjo/config"
	"os"
)

var (
	configFile = flag.String("configfile", "config.ini", "General configuration file")
)

type Stock struct {
	Ticker_exchange string //  "ticker.exchange",
	DataDate        string // "dataDate",
	DataTime        string // "dataDate",
	Data            interface{}
}

var (
	DBCONF    = make(map[string]string)
	GROUPMOD  = 100
	Stockfile string
	Mktdatas  []Stock
	Process   doGo
)

type doGo func([]Stock, chan int)

func SetProcess(process doGo) {
	Process = process
}
func main() {

	fmt.Println("Preparing...")
	initialize()
	fmt.Println("Calculating Data...")
	parallelrun(Process)
	fmt.Println("Done")
}

func initialize() {
	cfg, err := config.ReadDefault(*configFile)
	if err != nil {
		panic(err)
	}

	GROUPMOD, _ = cfg.Int("GENERAL", "groupmod")

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
	s, err := r.ReadString('\n')
	for err == nil {
		var md Stock
		md.Ticker_exchange = strings.Trim(s, "\n\r")
		Mktdatas = append(Mktdatas, md)
		s, err = r.ReadString('\n')
	}
}

func parallelrun(do doGo) {
	stockLen := len(Mktdatas)
	chs := make(map[int]chan int)
	for i := 0; i*GROUPMOD < stockLen; i += 1 {
		var slen int = (i + 1) * GROUPMOD
		if slen > stockLen {
			slen = stockLen
		}
		chs[i] = make(chan int)
		go do(Mktdatas[i*GROUPMOD:slen], chs[i])
		// j := i
	}
	for i, ch := range chs {
		fmt.Printf("waiting gort %d\n", i)
		<-ch
	}
}
