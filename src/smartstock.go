package main

import (
	"client"
	"fmt"
	//"io/ioutil"
	//"net/http"
)

func main() {
	/*
		req, err := http.NewRequest("GET", "https://gw.wmcloud.com/data/market/1.0.0/getTickRTSnapshot.csv", nil)
		if err != nil {
			fmt.Println("Error:", err)
			panic(err)
		}
		req.Header.Add("Authorization", "Bearer f637816e9303cea3c12981eee33f76ffd090ce9ba1599bac4853fba65a226243")

		httpClient := &http.Client{}

		resp, _ := httpClient.Do(req)

		if resp.StatusCode == 200 {
			body, _ := ioutil.ReadAll(resp.Body)

			fmt.Println(string(body))
		}
	*/

	c, err := client.NewClient(&client.ClientConfig{
		Host:     "192.168.129.136:8086",
		Username: "root",
		Password: "root",
		Database: "smartstock",
	})
	if err != nil {
		panic(err)
	}

	result, err := c.Query("select * from cpu_idle")
	if err != nil {
		panic(err)
	}

	fmt.Println(result[0].GetPoints())
}
