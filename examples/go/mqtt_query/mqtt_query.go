//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

type Result struct {
	Success bool       `json:"success"`
	Reason  string     `json:"reason"`
	Elapse  string     `json:"elapse"`
	Data    ResultData `json:"data"`
}

type ResultData struct {
	Columns []string `json:"columns"`
	Types   []string `json:"types"`
	Rows    [][]any  `json:"rows"`
}

func main() {
	wg := sync.WaitGroup{}

	// paho mqtt client options
	opts := paho.NewClientOptions()
	opts.SetCleanSession(true)
	opts.SetConnectRetry(false)
	opts.SetAutoReconnect(false)
	opts.SetProtocolVersion(4)
	opts.SetClientID("query-example")
	opts.AddBroker("127.0.0.1:5653")
	opts.SetKeepAlive(10 * time.Second)

	// connect to machbase-neo with paho mqtt client
	client := paho.NewClient(opts)
	connectToken := client.Connect()
	connectToken.WaitTimeout(10 * time.Second)
	if connectToken.Error() != nil {
		panic(connectToken.Error())
	}

	// subscribe to 'db/reply' to receive query result
	client.Subscribe("db/reply", 1, func(_ paho.Client, msg paho.Message) {
		defer wg.Done()

		buff := msg.Payload()
		result := Result{}
		if err := json.Unmarshal(buff, &result); err != nil {
			panic(err)
		}
		if !result.Success {
			fmt.Println("RECV: query failed:", result.Reason)
			return
		}
		if len(result.Data.Rows) == 0 {
			fmt.Println("Empty result")
			return
		}
		for i, rec := range result.Data.Rows {
			name := rec[0].(string)
			ts := time.Unix(0, int64(rec[1].(float64)))
			value := float64(rec[2].(float64))
			fmt.Println(i+1, name, ts, value)
		}
	})

	// publish query to 'db/query'
	jsonStr := `{ "q": "select * from EXAMPLE order by time desc limit 5" }`
	wg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	wg.Wait()

	// disconnect mqtt connection
	client.Disconnect(100)
}
