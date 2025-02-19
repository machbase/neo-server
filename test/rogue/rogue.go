package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

func main() {
	neoHttpAddr := "http://127.0.0.1:5654"
	runCount := 1
	clientCount := 1
	useTql := false
	timeoutThreshold := 1 * time.Second
	scenario := "default"

	flag.StringVar(&neoHttpAddr, "neo-http", neoHttpAddr, "Neo HTTP address")
	flag.IntVar(&runCount, "r", runCount, "Number of requests to send")
	flag.IntVar(&clientCount, "n", clientCount, "Number of clients to run")
	flag.BoolVar(&useTql, "tql", useTql, "Use TQL")
	flag.DurationVar(&timeoutThreshold, "timeout", timeoutThreshold, "Timeout threshold")
	flag.StringVar(&scenario, "scenario", scenario, "Scenario")
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
				randomDuration := time.Duration(float64(timeoutThreshold) * rand.Float64())
				time.Sleep(randomDuration)
				conn.Close()
			}()
			return conn, nil
		},
	}

	scenarios := map[string]func() string{
		"default": func() string {
			return fmt.Sprintf("select * from tag limit %d,100", rand.Intn(10000))
		},
		"insert": func() string {
			return fmt.Sprintf("insert into tag (tagid, time, value) values ('%s', %d, %f)", "new_tag", time.Now().Unix(), rand.Float64())
		},
	}

	sqlTextFunc := scenarios[scenario]
	if sqlTextFunc == nil {
		fmt.Println("Invalid scenario")
		return
	}
	wg := sync.WaitGroup{}
	for n := 0; n < clientCount; n++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < runCount; i++ {
				sqlText := sqlTextFunc()
				var req *http.Request
				if useTql {
					body := strings.NewReader(fmt.Sprintf("SQL(`%s`)\nJSON()\n", sqlText))
					// Create HTTP request
					if r, err := http.NewRequest("POST", neoHttpAddr+"/db/tql", body); err != nil {
						fmt.Println("Error creating request:", err)
						continue
					} else {
						req = r
					}
					req.Header.Set("Content-Type", "text/plain")
				} else {
					if r, err := http.NewRequest("GET", neoHttpAddr+"/db/query?q="+url.QueryEscape(sqlText), nil); err != nil {
						fmt.Println("Error creating request:", err)
						continue
					} else {
						req = r
					}
				}

				// send request
				resp, err := client.Do(req)
				if err != nil {
					fmt.Println("Error sending request:", err)
					continue
				}
				content := "..."
				if resp.ContentLength > 0 && resp.ContentLength < 1024 {
					cnt, _ := io.ReadAll(resp.Body)
					content = string(cnt)
				}
				resp.Body.Close()
				fmt.Println("Response status:", resp.Status, resp.ContentLength, content)
			}
		}(n)
	}
	wg.Wait()
}
