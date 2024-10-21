//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	addr := "http://127.0.0.1:5654/db/write/TAGDATA"

	rows := []string{
		fmt.Sprintf("my-car,%d,32.1", time.Now().UnixNano()),
		fmt.Sprintf("my-car,%d,65.4", time.Now().UnixNano()),
		fmt.Sprintf("my-car,%d,76.5", time.Now().UnixNano()),
	}

	client := http.Client{}
	rsp, err := client.Post(addr, "text/csv", bytes.NewBuffer([]byte(strings.Join(rows, "\n"))))
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
