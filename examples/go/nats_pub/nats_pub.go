package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// Before run this program,
//  1. Add bridge in machbase-neo server
//     bridge add -t nats my_nats server=127.0.0.1:4222 name=hello
//  2. Add subscriber
//     subscriber add hello-nats my_nats test.topic db/write/EXAMPLE:csv;
//  3. Start subscriber
//     subscriber start hello-nats
//  4. Run
//     go run nats_pub.go -server nats://<ip>:<port> -subject hello
func main() {
	optServer := flag.String("server", "nats://127.0.0.1:4222", "nats server address")
	optSubject := flag.String("subject", "hello", "subject to subscribe")
	optRequest := flag.Bool("request", false, "request-response model")
	flag.Parse()

	opts := nats.GetDefaultOptions()
	opts.Servers = []string{*optServer}
	conn, err := opts.Connect()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	tick := time.Now()
	lines := []string{}
	linesPerMsg := 1
	msgCount := 1000000
	serial := 0

	for n := 0; n < msgCount; n++ {
		for i := 0; i < linesPerMsg; i++ {
			line := fmt.Sprintf("hello-nats,%d,1.2345", tick.Add(time.Duration(serial)*time.Microsecond).UnixNano())
			lines = append(lines, line)
			serial++
		}
		reqData := []byte(strings.Join(lines, "\n"))
		lines = lines[0:0]

		if *optRequest {
			// A) request-respond model
			if rsp, err := conn.Request(*optSubject, reqData, 100*time.Millisecond); err != nil {
				panic(err)
			} else {
				fmt.Println("RESP:", string(rsp.Data))
			}
		} else {
			// B) fire-and-forget model
			if err := conn.Publish(*optSubject, reqData); err != nil {
				panic(err)
			}
		}
	}

	fmt.Println("msg sent: ", conn.OutMsgs)
}
