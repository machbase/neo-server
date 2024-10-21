//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func main() {
	addr := "http://127.0.0.1:5654/db/query"

	queryJson := `{"q":"select count(*) from M$SYS_TABLES where name = 'TAGDATA'"}`

	client := http.Client{}
	rsp, err := client.Post(addr, "application/json", bytes.NewBufferString(queryJson))
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
