package main

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
	"github.com/machbase/neo-server/v8/mods/util/metric"
)

func DefaultConfig() *machcli.Config {
	return &machcli.Config{
		Host:         "127.0.0.1",
		Port:         5656,
		TrustUsers:   map[string]string{},
		MaxOpenConn:  -1,
		MaxOpenQuery: -1,
	}
}

func main() {
	N := 1000_000
	C := 20
	user := "sys"
	pass := "manager"

	cfg := DefaultConfig()
	flag.StringVar(&cfg.Host, "host", cfg.Host, "Database host")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "Database port")
	flag.StringVar(&user, "user", user, "Database user")
	flag.StringVar(&pass, "pass", pass, "Database password")
	flag.IntVar(&N, "n", N, "number of repeats per client")
	flag.IntVar(&C, "c", C, "number of client connections")
	flag.Parse()

	cfg.TrustUsers[user] = pass
	db, err := machcli.NewDatabase(cfg)
	if err != nil {
		fmt.Println("Error open to database:", err)
		panic(err)
	}

	defer db.Close()
	ctx := context.TODO()

	seriesID, _ := metric.NewSeriesID("bench", "", 10*time.Second, 10)
	measure := metric.NewTimeSeries(10*time.Second, 6*10,
		metric.NewHistogram(100, 0.50, 0.90, 0.99),
		metric.WithListener(func(p metric.Product) {
			hist := p.Value.(*metric.HistogramValue)
			fmt.Println(p.Time.Format("01/02 15:04:05"),
				"samples:", hist.Samples,
				fmt.Sprintf("tps: %.1f/s", float64(hist.Samples)/10.0),
				"p50:", elapsed(hist.Values[0]),
				"p90:", elapsed(hist.Values[1]),
				"p99:", elapsed(hist.Values[2]),
			)
		}),
		metric.WithMeta(metric.SeriesInfo{
			MeasureName: "elapse_time",
			MeasureType: metric.HistogramType(metric.UnitDuration),
			SeriesID:    seriesID,
		}),
	)

	wg := sync.WaitGroup{}
	for c := 0; c < C; c++ {
		conn, err := db.Connect(ctx, api.WithPassword(user, pass))
		if err != nil {
			fmt.Println("Error connecting to database:", err)
			panic(err)
		}
		defer conn.Close()
		wg.Add(1)
		go func(cliConn api.Conn, N int, cliNo int) {
			defer wg.Done()
			for n := 0; n < N; n++ {
				tm := time.Now()
				tmstr := tm.In(time.Local).Format("2006-01-02 15:04:05") // '2025-10-15 09:05:59'
				rows, err := cliConn.Query(ctx, querySQL, "tag1", tmstr, "tag1", tmstr)
				if err != nil {
					fmt.Println("Error query:", err)
					panic(err)
				}
				for rows.Next() {
					var mtime time.Time
					var avg float64
					rows.Scan(&mtime, &avg)
				}
				rows.Close()
				measure.Add(float64(time.Since(tm).Nanoseconds()))
			}
		}(conn, N, c)
	}
	wg.Wait()
}

func elapsed(f float64) string {
	if f < 1e3 {
		return fmt.Sprintf("%.f ns", f)
	} else if f < 1e6 {
		return fmt.Sprintf("%.2f µs", f/1e3)
	} else if f < 1e9 {
		return fmt.Sprintf("%.2f ms", f/1e6)
	} else {
		return fmt.Sprintf("%.2f s", f/1e9)
	}
}

const querySQL = `SELECT mtime, avg
FROM (
	SELECT ROLLUP('minute', 1, time) as mtime, AVG(value) 
	FROM tag 
	WHERE name = ? AND 
		time < DATE_TRUNC('minute', TO_DATE(?)) - 1m 
	GROUP BY mtime 
	ORDER BY mtime
		UNION ALL
	SELECT DATE_TRUNC('minute', time) as mtime, AVG(value)
	FROM TAG
	WHERE name = ? AND
		time >= DATE_TRUNC('minute', TO_DATE(?)) - 1m
	GROUP BY mtime
	ORDER BY mtime
)`
