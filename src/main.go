package main

import (
	"fmt"
	"time"
)

func main() {

	// datetime, _ := time.Parse("2006-01-02",
	// 	time.Now().String()[:10])
	// timeInt := datetime.UnixNano() / 1e6
	fmt.Println(time.Now().String()[:10])
	datetime, _ := time.Parse("2006-01-02 15:04:05 MST -0700",
		"2014-11-26 15:02:22 GMT +0800")
	fmt.Println(datetime.UnixNano() / 1e6)
}
