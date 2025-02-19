package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func main() {
	neoHttpAddr := "http://127.0.0.1:5654"
	runCount := 1
	clientCount := 1
	flag.StringVar(&neoHttpAddr, "neo-http", neoHttpAddr, "Neo HTTP address")
	flag.IntVar(&runCount, "r", runCount, "Number of requests to send")
	flag.IntVar(&clientCount, "n", clientCount, "Number of clients to run")
	flag.Parse()

	neoHttpAddr = strings.TrimSuffix(neoHttpAddr, "/")

	// Create HTTP Client
	client := &http.Client{}

	// Disconnect TCP connection after the random duration
	client.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := net.Dial(network, addr)
			if err != nil {
				return nil, err
			}

			// disconnect after random duration
			go func() {
				randomDuration := time.Duration(rand.Intn(5000)) * time.Millisecond
				time.Sleep(randomDuration)
				conn.Close()
			}()
			return conn, nil
		},
	}

	wg := sync.WaitGroup{}
	for n := 0; n < clientCount; n++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < runCount; i++ {
				sqlText := fmt.Sprintf("select * from test_table order by worker desc limit %d,10000000", rand.Intn(10000))
				body := strings.NewReader(fmt.Sprintf("SQL(`%s`)\nJSON()\n", sqlText))
				// Create HTTP request
				req, err := http.NewRequest("POST", neoHttpAddr+"/db/tql", body)
				if err != nil {
					fmt.Println("Error creating request:", err)
					continue
				}
				req.Header.Set("Content-Type", "text/plain")

				// send request
				resp, err := client.Do(req)
				if err != nil {
					fmt.Println("Error sending request:", err)
					continue
				}
				// cnt, _ := io.ReadAll(resp.Body)
				// fmt.Println("Response body:", string(cnt))
				resp.Body.Close()
				fmt.Println("Response status:", resp.Status, resp.ContentLength)
			}
		}(n)
	}
	wg.Wait()
}
