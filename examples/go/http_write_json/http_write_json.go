//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type WriteRsp struct {
	Success bool         `json:"success"`
	Reason  string       `json:"reason"`
	Elapse  string       `json:"elapse"`
	Data    WriteRspData `json:"data"`
}

type WriteRspData struct {
	AffectedRows uint64 `json:"affectedRows"`
}

func main() {
	addr := "http://127.0.0.1:5654/db/write/TAGDATA?method=insert"

	rows := [][]any{
		{"my-car", time.Now().UnixNano(), 32.1},
		{"my-car", time.Now().UnixNano(), 65.4},
		{"my-car", time.Now().UnixNano(), 76.5},
	}
	columns := []string{"name", "time", "value"}
	writeReq := map[string]any{
		"data": map[string]any{
			"columns": columns,
			"rows":    rows,
		},
	}

	queryJson, _ := json.Marshal(&writeReq)

	client := http.Client{}
	rsp, err := client.Post(addr, "application/json", bytes.NewBuffer(queryJson))
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		panic(err)
	}

	content := string(body)

	if rsp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("ERR %s %s", rsp.Status, content))
	}

	fmt.Println(content)
}
