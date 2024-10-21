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
	optAsync := flag.Bool("async", false, "publish async")
	flag.Parse()

	opts := nats.GetDefaultOptions()
	opts.Servers = []string{*optServer}
	conn, err := opts.Connect()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Use the JetStream context to produce messages that have been persisted.
	js, err := conn.JetStream(nats.PublishAsyncMaxPending(256))
	if err != nil {
		panic(err)
	}

	streamName := "hello-nats"
	js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{*optSubject},
	})

	tick := time.Now()
	lines := []string{}
	linesPerMsg := 1
	//msgCount := 2_000_000
	msgCount := 10
	serial := 0

	for n := 0; n < msgCount; n++ {
		for i := 0; i < linesPerMsg; i++ {
			line := fmt.Sprintf("hello-nats,%d,1.2345", tick.Add(time.Duration(serial)*time.Microsecond).UnixNano())
			lines = append(lines, line)
			serial++
		}
		reqData := []byte(strings.Join(lines, "\n"))
		lines = lines[0:0]

		if *optAsync {
			if _, err := js.PublishAsync(*optSubject, reqData); err != nil {
				panic(err)
			}
		} else {
			if _ /*ack*/, err := js.Publish(*optSubject, reqData); err != nil {
				panic(err)
			}
		}
	}

	select {
	case <-js.PublishAsyncComplete():
	case <-time.After(5 * time.Second):
		fmt.Println("Did not resolve in time")
	}

	nfo, _ := js.StreamInfo(streamName)
	fmt.Println("msgs: ", nfo.State.Msgs)
}
