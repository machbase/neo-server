//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

type WriteReq struct {
	Table string       `json:"table"`
	Data  WriteReqData `json:"data"`
}

type WriteReqData struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

func main() {
	client := http.Client{}
	for ts := range time.Tick(500 * time.Millisecond) {
		delta := float64(ts.UnixMilli()%15000) / 15000
		theta := 2 * math.Pi * delta
		sin, cos := math.Sin(theta), math.Cos(theta)

		content, _ := json.Marshal(&WriteReq{
			Table: "EXAMPLE",
			Data: WriteReqData{
				Columns: []string{"name", "time", "value"},
				Rows: [][]any{
					{"wave.sin", ts.UTC().UnixNano(), sin},
					{"wave.cos", ts.UTC().UnixNano(), cos},
				},
			},
		})
		rsp, err := client.Post(
			"http://127.0.0.1:5654/db/write/example",
			"application/json",
			bytes.NewBuffer(content))
		if err != nil {
			panic(err)
		}
		if rsp.StatusCode != http.StatusOK {
			panic(fmt.Errorf("response %d", rsp.StatusCode))
		}
	}
}
