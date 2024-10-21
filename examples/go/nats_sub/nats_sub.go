package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

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
//     go run nats_pub.go -server nats://<ip>:<port>
func main() {
	optServer := flag.String("server", "nats://127.0.0.1:4222", "nats server address")
	optSubject := flag.String("subject", "hello", "subject to subscribe")
	optGroup := flag.String("queue", "", "queue group")
	flag.Parse()

	opts := nats.GetDefaultOptions()
	opts.Servers = []string{*optServer}
	conn, err := opts.Connect()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	var subscription *nats.Subscription
	if *optGroup == "" {
		if s, err := conn.Subscribe(*optSubject, handler); err != nil {
			panic(err)
		} else {
			subscription = s
		}
	} else {
		fmt.Println("Subscribe:", *optSubject, "Queue:", *optGroup)
		if s, err := conn.QueueSubscribe(*optSubject, *optGroup, handler); err != nil {
			panic(err)
		} else {
			subscription = s
		}
	}

	if err := subscription.SetPendingLimits(1_000_000, nats.DefaultSubPendingBytesLimit); err != nil {
		fmt.Println("pending limits", err.Error())
	}

	// wait Ctrl+C
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Waiting, press ctrl+c to continue...")
	<-done // Will block here until user hits ctrl+c

	subscription.Unsubscribe()

	fmt.Println("Total Received:", count)
}

var count int64

func handler(msg *nats.Msg) {
	if msg.Reply == "" {
		msg.Ack()
	} else {
		msg.Respond([]byte(`{"success":true, "reason":"success", "elapse": "0ms"}`))
	}
	atomic.AddInt64(&count, 1)
}
