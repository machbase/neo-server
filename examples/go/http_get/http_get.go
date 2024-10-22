//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func main() {
	client := http.Client{}
	q := url.QueryEscape("select count(*) from M$SYS_TABLES where name = 'TAGDATA'")
	rsp, err := client.Get("http://127.0.0.1:5654/db/query?q=" + q)
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
