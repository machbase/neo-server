package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/tidwall/gjson"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func main() {
	var scenarioName string
	var neoHttpAddr string
	var cleanStart bool
	var cleanStop bool
	var slowQueryThreshold time.Duration
	var httpTimeout time.Duration
	var overrideTimeout time.Duration
	var overrideAppender int
	var overrideSelector int

	flag.StringVar(&scenarioName, "scenario", "default", "scenario name")
	flag.StringVar(&neoHttpAddr, "neo-http", "http://127.0.0.1:5654", "machbase-neo http address")
	flag.BoolVar(&cleanStart, "clean-start", false, "drop table before start")
	flag.BoolVar(&cleanStop, "clean-stop", false, "drop table after stop")
	flag.DurationVar(&slowQueryThreshold, "slow-query", 0, "slow query threshold time duration")
	flag.DurationVar(&httpTimeout, "http-timeout", 0, "http timeout duration, 0 means no timeout")
	flag.DurationVar(&overrideTimeout, "timeout", 0, "override timeout of the scenario")
	flag.IntVar(&overrideAppender, "append", -1, "override append worker count")
	flag.IntVar(&overrideSelector, "select", -1, "override select worker count")
	flag.Parse()

	if scenario, ok := scenarios[scenarioName]; !ok {
		fmt.Printf("scenario %q not found\n", scenarioName)
		fmt.Printf("available scenarios:\n")
		for name := range scenarios {
			fmt.Printf("  %s\n", name)
		}
		return
	} else {
		if overrideTimeout > 0 {
			scenario.Timeout = overrideTimeout
		}
		if scenario.Timeout == 0 {
			scenario.Timeout = 1 * time.Minute
		}
		if overrideAppender >= 0 {
			scenario.AppendWorker = overrideAppender
		}
		if overrideSelector >= 0 {
			scenario.SelectWorker = overrideSelector
		}
		fmt.Println("Run scenario:", scenarioName)
		fmt.Println("Timeout:", scenario.Timeout)
		fmt.Println("CPU count:", runtime.NumCPU())
		fmt.Println("Append worker:", scenario.AppendWorker)
		fmt.Println("Select worker:", scenario.SelectWorker)
		fmt.Println()
		start := time.Now()
		scenario.Run(neoHttpAddr, cleanStart, cleanStop, httpTimeout, slowQueryThreshold)
		fmt.Println("Total time:", time.Since(start))
	}
}

var scenarios = map[string]Scenario{
	"default": {
		CreateTableSql: `CREATE TAG TABLE IF NOT EXISTS test_table (
			name varchar(40) primary key,
			time datetime basetime,
			value double summarized)
		`,
		DropTableSql:             `DROP TABLE test_table`,
		AppendUri:                "/db/write/test_table?method=append",
		AppendWorker:             10,
		AppendWorkerRunPerSecond: 10,
		AppendRecordsPerRun:      10,
		AppendRecordDataFunc: func(workerId int, now time.Time, round int, nth int, w io.Writer) {
			fmt.Fprintf(w, "tag_%d_%d,%d,%f\n", workerId, nth, time.Now().UnixNano(), rand.Float64())
		},
		SelectWorker:             20,
		SelectWorkerRunPerSecond: 10,
		SelectSqlFunc: func(workerId int, now time.Time) string {
			return fmt.Sprintf("SELECT * FROM test_table WHERE time > %d limit 100", now.Add(-10*time.Second).UnixNano())

		},
	},
	"rollup": {
		CreateTableSql: `CREATE TAG TABLE IF NOT EXISTS test_table (
			name varchar(40) primary key,
			time datetime basetime,
			value double summarized) with rollup(MIN)
		`,
		DropTableSql:             `DROP TABLE test_table CASCADE`,
		AppendUri:                "/db/write/test_table?method=append",
		AppendWorker:             2,
		AppendWorkerRunPerSecond: 10,
		AppendRecordsPerRun:      1000,
		AppendRecordDataFunc: func(workerId int, now time.Time, round int, nth int, w io.Writer) {
			fmt.Fprintf(w, "tag_%d_%d,%d,%f\n", workerId, nth, time.Now().UnixNano(), rand.Float64())
		},
		SelectWorker:             runtime.NumCPU() - 2,
		SelectWorkerRunPerSecond: 1000,
		SelectSqlFunc: func(workerId int, now time.Time) string {
			return fmt.Sprintf("select name, rollup('min', 1, time) mtime, avg(value) from test_table where time >= %d group by name, mtime limit 20",
				now.Add(-10*time.Minute).UnixNano())
		},
	},
	"rollup-meta": {
		CreateTableSql: `CREATE TAG TABLE IF NOT EXISTS test_table (
			name varchar(40) primary key,
			time datetime basetime,
			value double summarized) 
            metadata (worker varchar(20), nth int32) with rollup(MIN)
		`,
		DropTableSql:             `DROP TABLE test_table CASCADE`,
		AppendUri:                "/db/write/test_table?method=append",
		AppendWorker:             2,
		AppendWorkerRunPerSecond: 10,
		AppendRecordsPerRun:      1000,
		AppendRecordDataFunc: func(workerId int, now time.Time, nRun int, nRecord int, w io.Writer) {
			ts := time.Now()
			if workerId == 0 && nRecord >= 900 {
				// these sensors have wrong internal clock and send data 1 minute ago
				ts = ts.Add(-1 * time.Minute)
			}
			// name, time, value, worker, round
			workerId = workerId*500 + nRun%500
			fmt.Fprintf(w, "tag_%d_%d,%d,%f,worker-%d,%d\n",
				workerId, nRecord, ts.UnixNano(), rand.Float64(), workerId, nRecord)
		},
		SelectWorker:             runtime.NumCPU() * 2,
		SelectWorkerRunPerSecond: 1000,
		SelectSqlFunc: func(workerId int, now time.Time) string {
			ts := time.Now()
			targetNRecord := rand.Int31n(1000)
			workers := []string{}
			for i := 0; i < 8; i++ {
				workers = append(workers, fmt.Sprintf("'worker-%d'", rand.Int31n(1000)))
			}
			return fmt.Sprintf(api.SqlTidy(`select
                    name, rollup('min', 1, time) mtime, avg(value)
                from
                    test_table
                where
			        worker in (%s)
                and nth = %d
                and time >= TO_DATE('%s') and time <= TO_DATE('%s')
                group by name, mtime`),
				strings.Join(workers, ","),
				targetNRecord,
				ts.Add(-2*time.Minute).In(time.Local).Format("2006-01-02 15:04:05"), ts.In(time.Local).Format("2006-01-02 15:04:05"))
		},
	},
	"part1": {
		CreateTableSql: `CREATE TAG TABLE IF NOT EXISTS test_table (
				tagid   VARCHAR(12) PRIMARY KEY,
				time    DATETIME BASETIME,
				value   DOUBLE SUMMARIZED,
				value1  DOUBLE,
				value2  DOUBLE
			)
			METADATA
			(
				meta1    VARCHAR(32),
				meta2    VARCHAR(32)
			) tag_partition_count=1, tag_data_part_size=33554432`,
		DropTableSql:             `DROP TABLE test_table CASCADE`,
		AppendUri:                "/db/write/test_table?method=append",
		AppendWorker:             2,
		AppendWorkerRunPerSecond: 10,
		AppendRecordsPerRun:      1000,
		AppendRecordDataFunc: func(workerId int, now time.Time, nRun int, nRecord int, w io.Writer) {
			ts := time.Now()
			tagId := rand.Int31n(1_000_000)
			m1 := tagId % 200
			m2 := m1 % 100
			fmt.Fprintf(w, "tag-%d,%d,%f,%f,%f,m1-%d,m2-%d\n",
				tagId, ts.UnixNano(), rand.Float64(), rand.Float64(), rand.Float64(), m1, m2)
		},
		SelectWorker:             runtime.NumCPU() * 2,
		SelectWorkerRunPerSecond: 1000,
		SelectSqlFunc: func(workerId int, now time.Time) string {
			m1s := []string{}
			m2s := []string{}
			for i := 0; i < 4; i++ {
				tagId := rand.Int31n(1_000_000)
				m1 := tagId % 200
				if i < 2 {
					m1s = append(m1s, fmt.Sprintf("'m1-%d'", m1))
				}
				m2 := m1 % 100
				m2s = append(m2s, fmt.Sprintf("'m2-%d'", m2))
			}
			return fmt.Sprintf(api.SqlTidy(`select * from test_table
				where
					meta1 in (%s)
				and meta2 in (%s)
				order by time desc
				limit 10`),
				strings.Join(m1s, ","),
				strings.Join(m2s, ","))
		},
	},
}

type Scenario struct {
	CreateTableSql           string
	DropTableSql             string
	AppendUri                string
	AppendWorker             int
	AppendRecordsPerRun      int
	AppendWorkerRunPerSecond int
	AppendRecordDataFunc     func(workerId int, now time.Time, nRun int, nRecord int, w io.Writer)
	SelectWorker             int
	SelectWorkerRunPerSecond int
	SelectSqlFunc            func(workerId int, now time.Time) string
	Timeout                  time.Duration
}

func (s Scenario) Run(neoHttpAddr string, cleanStart bool, cleanStop bool, httpTimeout time.Duration, slowQueryThreshold time.Duration) {
	if cleanStart {
		s.DropTable(neoHttpAddr)
	}
	// Create table
	s.CreateTable(neoHttpAddr)

	wg := sync.WaitGroup{}
	closeCh := make(chan struct{})

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 20,
			MaxConnsPerHost:     (s.AppendWorker + s.SelectWorker) * 2,
		},
		Timeout: httpTimeout,
	}
	// Append Data
	for i := 0; i < s.AppendWorker; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			round := 0
			ticker := time.NewTicker(time.Second / time.Duration(s.AppendWorkerRunPerSecond))
			for {
				select {
				case <-closeCh:
					return
				case <-ticker.C:
					split := 10
					for part := 0; part < split; part++ {
						dataBuf := &bytes.Buffer{}
						for n := 0; n < s.AppendRecordsPerRun/split; n++ {
							s.AppendRecordDataFunc(workerId, time.Now(), round, part*split+n, dataBuf)
						}
						round++
						req, err := http.NewRequest("POST", neoHttpAddr+s.AppendUri, bytes.NewBuffer(dataBuf.Bytes()))
						if err != nil {
							fmt.Println("Failed to create request:", err)
							os.Exit(1)
						}
						req.Header.Set("Content-Type", "text/csv")
						req.Header.Set("Content-Length", fmt.Sprintf("%d", dataBuf.Len()))
						rsp, err := client.Do(req)
						if err != nil {
							fmt.Println("Failed to append data:", err)
							os.Exit(1)
						}
						if rsp.StatusCode != http.StatusOK {
							dumpResponse(rsp, "Failed to append data")
							continue
						}
						content, err := io.ReadAll(rsp.Body)
						if err != nil {
							fmt.Println("Failed to read response body:", err)
							os.Exit(1)
						}
						rsp.Body.Close()

						jsonStr := string(content)
						success := gjson.Get(jsonStr, "success").Bool()
						reason := gjson.Get(jsonStr, "reason").String()
						if !success {
							fmt.Println("Failed to select data:", reason)
							os.Exit(1)
						}
					}
				}
			}
		}(i)
	}

	var stat = NewStat()
	stat.Start()

	// Select Data
	for i := 0; i < s.SelectWorker; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			ticker := time.NewTicker(time.Second / time.Duration(s.SelectWorkerRunPerSecond))
			for {
				select {
				case <-closeCh:
					return
				case ts := <-ticker.C:
					sqlText := s.SelectSqlFunc(workerId, ts)
					req, err := http.NewRequest("GET", neoHttpAddr+"/db/query?q="+url.QueryEscape(sqlText), nil)
					if err != nil {
						fmt.Println("Failed to create request:", err)
						os.Exit(1)
					}
					reqTime := time.Now()
					rsp, err := client.Do(req)
					if err != nil {
						fmt.Println("Failed to select data:", err)
						os.Exit(1)
					}
					if rsp.StatusCode != http.StatusOK {
						stat.AddSelectError()
						dumpResponse(rsp, "Failed to select data")
						continue
					}
					reqElapse := time.Since(reqTime)

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
					elapseStr := gjson.Get(jsonStr, "elapse").String()
					elapse, err := time.ParseDuration(elapseStr)
					if err != nil {
						fmt.Println("Failed to parse elapse:", err)
						os.Exit(1)
					}
					stat.AddSelectRowsCount(int64(len(rows)))
					stat.AddSelectTime(elapse, reqElapse)
					if slowQueryThreshold > 0 && elapse > slowQueryThreshold {
						fmt.Println("Slow query elapse:", elapse, "\n", sqlText)
					}
				}
			}
		}(i)
	}

	if s.Timeout > 0 {
		go func() {
			timeout := time.After(s.Timeout)
			ticker := time.NewTicker(10 * time.Second)
			for {
				select {
				case <-ticker.C:
					stat.PrintAndReset()
				case <-timeout:
					close(closeCh)
					return
				}
			}
		}()
	}
	// Wait for all workers
	wg.Wait()

	stat.Stop()

	if cleanStop {
		s.DropTable(neoHttpAddr)
	}
}

func (s Scenario) CreateTable(neoHttpAddr string) {
	if s.CreateTableSql != "" {
		rsp, err := http.Get(neoHttpAddr + "/db/query?q=" + url.QueryEscape(s.CreateTableSql))
		if err != nil {
			fmt.Println("Failed to create table:", err)
			return
		}
		if rsp.StatusCode != http.StatusOK {
			dumpResponse(rsp, "Failed to create table")
			return
		}
		rsp.Body.Close()
	}
}

func (s Scenario) DropTable(neoHttpAddr string) {
	if s.DropTableSql != "" {
		rsp, err := http.Get(neoHttpAddr + "/db/query?q=" + url.QueryEscape(s.DropTableSql))
		if err != nil {
			fmt.Println("Failed to drop table:", err)
			return
		}
		if rsp.StatusCode != http.StatusOK {
			dumpResponse(rsp, "Failed to drop table")
			return
		}
		rsp.Body.Close()
	}
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
	createdTime         time.Time
	selectCount         int64
	selectRowsCount     int64
	selectErrorCount    int64
	selectQueryTotal    int64
	selectMinElapse     int64
	selectMaxElapse     int64
	selectHttpTotal     int64
	selectHttpMinElapse int64
	selectHttpMaxElapse int64

	selectCumulativeCount      int64
	selectCumulativeRowsCount  int64
	selectErrorCumulativeCount int64
	selectMaxElapseWatermark   int64
	selectMaxHttpWatermark     int64

	wg               sync.WaitGroup
	selectRowsCountC chan int64
	selectTimeC      chan [2]time.Duration // [query, wait]
	commandC         chan StatCommand
}

type StatCommand string

func NewStat() *Stat {
	return &Stat{
		createdTime:      time.Now(),
		selectRowsCountC: make(chan int64, 100),
		selectTimeC:      make(chan [2]time.Duration, 100),
		commandC:         make(chan StatCommand, 10),
	}
}

func (stat *Stat) AddSelectError() {
	atomic.AddInt64(&stat.selectErrorCount, 1)
}

func (stat *Stat) AddSelectTime(queryElapse time.Duration, waitElapse time.Duration) {
	stat.selectTimeC <- [2]time.Duration{queryElapse, waitElapse}
}

func (stat *Stat) AddSelectRowsCount(count int64) {
	stat.selectRowsCountC <- count
}

func (stat *Stat) Start() {
	stat.wg.Add(1)
	go func() {
		defer stat.wg.Done()
		for {
			select {
			case dur := <-stat.selectTimeC:
				queryElapse := dur[0].Nanoseconds()
				httpElapse := dur[1].Nanoseconds()
				stat.selectCount++
				stat.selectQueryTotal += queryElapse
				stat.selectHttpTotal += httpElapse

				if queryElapse > stat.selectMaxElapse {
					stat.selectMaxElapse = queryElapse
				}
				if queryElapse < stat.selectMinElapse || stat.selectMinElapse == 0 {
					stat.selectMinElapse = queryElapse
				}
				if httpElapse > stat.selectHttpMaxElapse {
					stat.selectHttpMaxElapse = httpElapse
				}
				if httpElapse < stat.selectHttpMinElapse || stat.selectHttpMinElapse == 0 {
					stat.selectHttpMinElapse = httpElapse
				}
			case count := <-stat.selectRowsCountC:
				stat.selectRowsCount += count
			case cmd := <-stat.commandC:
				if cmd == "stop" {
					return
				}
				stat.selectCumulativeCount += stat.selectCount
				stat.selectCumulativeRowsCount += stat.selectRowsCount
				stat.selectErrorCumulativeCount += stat.selectErrorCount
				if stat.selectMaxElapseWatermark < stat.selectMaxElapse {
					stat.selectMaxElapseWatermark = stat.selectMaxElapse
				}
				if stat.selectMaxHttpWatermark < stat.selectHttpMaxElapse {
					stat.selectMaxHttpWatermark = stat.selectHttpMaxElapse
				}
				stat.print()

				if cmd == "print-reset" {
					stat.selectCount = 0
					stat.selectRowsCount = 0
					stat.selectErrorCount = 0
					stat.selectQueryTotal = 0
					stat.selectMinElapse = 0
					stat.selectMaxElapse = 0
					stat.selectHttpTotal = 0
					stat.selectHttpMinElapse = 0
					stat.selectHttpMaxElapse = 0
				}
			}
		}
	}()
}

func (stat *Stat) Stop() {
	stat.commandC <- "stop"
	stat.wg.Wait()
	close(stat.selectTimeC)
	close(stat.commandC)
	stat.print()
}

func (stat *Stat) Print() {
	stat.commandC <- "print"
}

func (stat *Stat) PrintAndReset() {
	stat.commandC <- "print-reset"
}

var printer = message.NewPrinter(language.English)

func (stat *Stat) print() {
	if stat.selectCount > 0 {
		printer.Printf("Elapsed: %v\n", time.Since(stat.createdTime))
		printer.Printf("Cumulative select: %d error: %d rows: %d query-max: %v http-max: %v\n",
			stat.selectCumulativeCount, stat.selectErrorCumulativeCount,
			stat.selectCumulativeRowsCount,
			time.Duration(stat.selectMaxElapseWatermark),
			time.Duration(stat.selectMaxHttpWatermark))
		printer.Printf("This cycle select: %d error: %d rows: %d\n",
			stat.selectCount, stat.selectErrorCount, stat.selectRowsCount)
		printer.Printf("         http-avg: %v http-min: %v http-max: %v\n",
			time.Duration(stat.selectHttpTotal/stat.selectCount),
			time.Duration(stat.selectHttpMinElapse),
			time.Duration(stat.selectHttpMaxElapse))
		printer.Printf("        query-avg: %v query-min: %v query-max: %v\n",
			time.Duration(stat.selectQueryTotal/stat.selectCount),
			time.Duration(stat.selectMinElapse),
			time.Duration(stat.selectMaxElapse))
		printer.Println()
	}
}
