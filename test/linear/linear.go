package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func main() {
	numberOfWorkers := 1
	neoHttpAddr := "http://127.0.0.1:5654"
	numberOfRuns := 1

	flag.StringVar(&neoHttpAddr, "neo-http", neoHttpAddr, "Neo HTTP address")
	flag.IntVar(&numberOfWorkers, "n", numberOfWorkers, "Number of workers to use")
	flag.IntVar(&numberOfRuns, "r", numberOfRuns, "Number of runs")
	flag.Parse()

	stat := NewStat(numberOfWorkers, numberOfRuns)
	stat.Start()

	wg := sync.WaitGroup{}
	for i := 0; i < numberOfWorkers; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for r := 0; r < numberOfRuns; r++ {
				start := time.Now()

				queryNeo(neoHttpAddr, stat)

				runElapse := time.Since(start)
				stat.RunCh <- runElapse
			}
		}(i)
	}
	wg.Wait()
	stat.Stop()
}

var client = &http.Client{
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
	},
}

func queryNeo(neoHttpAddr string, stat *Stat) {
	//sqlText := "select * from test_table order by time limit 10"
	sqlText := "select * from test_table limit 10"

	req, err := http.NewRequest("GET", neoHttpAddr+"/db/query?q="+url.QueryEscape(sqlText), nil)
	if err != nil {
		fmt.Println("Failed to create request:", err)
		os.Exit(1)
	}
	rsp, err := client.Do(req)
	if err != nil {
		fmt.Println("Failed to select data:", err)
		os.Exit(1)
	}
	if rsp.StatusCode != http.StatusOK {
		dumpResponse(rsp, "Failed to select data")
		os.Exit(1)
	}

	content, err := io.ReadAll(rsp.Body)
	if err != nil {
		fmt.Println("Failed to read response body:", err)
		os.Exit(1)
	}
	rsp.Body.Close()

	jsonStr := string(content)
	success := gjson.Get(jsonStr, "success").Bool()
	if !success {
		reason := gjson.Get(jsonStr, "reason").String()
		fmt.Println("Failed to select data:", reason)
		os.Exit(1)
	}
	rows := gjson.Get(jsonStr, "data.rows").Array()
	_ = rows
	elapseStr := gjson.Get(jsonStr, "elapse").String()
	elapse, err := time.ParseDuration(elapseStr)
	if err != nil {
		fmt.Println("Failed to parse elapse:", err)
		os.Exit(1)
	}
	stat.QueryCh <- elapse
}

func dumpResponse(rsp *http.Response, msg string) {
	fmt.Println("Log:", msg)
	fmt.Println("Status:", rsp.Status)
	fmt.Println("Header:")
	for k, v := range rsp.Header {
		fmt.Printf("  %s: %v\n", k, v)
	}
	fmt.Println("Body:")
	io.Copy(os.Stdout, rsp.Body)
}

type Stat struct {
	RunCh         chan time.Duration
	runCount      int64
	prevRunCount  int64
	runElapsedSum time.Duration
	runElapseMin  time.Duration
	runElapseMax  time.Duration

	QueryCh         chan time.Duration
	queryElapsedSum time.Duration
	queryElapsedMin time.Duration
	queryElapsedMax time.Duration

	startTime time.Time
	closeCh   chan struct{}
	closeWg   sync.WaitGroup
	ticker    *time.Ticker

	workers int
	runs    int
}

func NewStat(worker, run int) *Stat {
	return &Stat{
		RunCh:     make(chan time.Duration, 1000),
		QueryCh:   make(chan time.Duration, 1000),
		closeCh:   make(chan struct{}),
		ticker:    time.NewTicker(10 * time.Second),
		startTime: time.Now(),
		workers:   worker,
		runs:      run,
	}
}

func (s *Stat) Start() {
	s.closeWg.Add(1)
	go func() {
		defer s.closeWg.Done()
		for {
			select {
			case d := <-s.RunCh:
				s.runCount++
				s.runElapsedSum += d
				if s.runElapseMin == 0 || d < s.runElapseMin {
					s.runElapseMin = d
				}
				if d > s.runElapseMax {
					s.runElapseMax = d
				}
			case d := <-s.QueryCh:
				s.queryElapsedSum += d
				if s.queryElapsedMin == 0 || d < s.queryElapsedMin {
					s.queryElapsedMin = d
				}
				if d > s.queryElapsedMax {
					s.queryElapsedMax = d
				}
			case <-s.ticker.C:
				s.Print()
			case <-s.closeCh:
				return
			}
		}
	}()
}

func (s *Stat) Stop() {
	close(s.closeCh)
	s.closeWg.Wait()
	s.Print()
}

var printer = message.NewPrinter(language.English)

func (s *Stat) Print() {
	thisRunCount := s.runCount - s.prevRunCount

	printer.Println(" Elapsed:", time.Since(s.startTime), "Workers:", s.workers, "Runs:", s.runs)
	printer.Println(" Query runs:", s.runCount, "/", s.workers*s.runs, ", This cycle:", thisRunCount)
	printer.Println(" http   avg:", s.runElapsedSum/time.Duration(s.runCount), "min:", s.runElapseMin, "max:", s.runElapseMax)
	printer.Println(" query  avg:", s.queryElapsedSum/time.Duration(s.runCount), "min:", s.queryElapsedMin, "max:", s.queryElapsedMax)
	fmt.Println()

	s.prevRunCount = s.runCount
}
