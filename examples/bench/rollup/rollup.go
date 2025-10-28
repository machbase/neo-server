package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"slices"
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

// !!!! important !!!!
// This test program works only with 1 minute rollup
// !!!!
func main() {
	N := 100
	user := "sys"
	pass := "manager"
	forceFlush := false

	cfg := DefaultConfig()
	flag.StringVar(&cfg.Host, "host", cfg.Host, "Database host")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "Database port")
	flag.StringVar(&user, "user", user, "Database user")
	flag.StringVar(&pass, "pass", pass, "Database password")
	flag.IntVar(&N, "n", N, "number of repeats per client")
	flag.BoolVar(&forceFlush, "force-flush", forceFlush, "force table flush after each append")
	flag.Parse()

	cfg.TrustUsers[user] = pass
	db, err := machcli.NewDatabase(cfg)
	if err != nil {
		fmt.Println("Error open to database:", err)
		panic(err)
	}

	defer db.Close()
	ctx := context.TODO()

	seriesID, _ := metric.NewSeriesID("bench", "", 60*time.Second, 10)
	measure := metric.NewTimeSeries(60*time.Second, 10,
		metric.NewMeter(),
		metric.WithListener(func(p metric.Product) {
			conn, err := db.Connect(ctx, api.WithPassword(user, pass))
			if err != nil {
				fmt.Println("Error connecting to database:", err)
				panic(err)
			}
			defer conn.Close()

			ms := queryRollupWithConn(ctx, conn, time.Now())
			slices.Reverse(ms)
			for _, m := range ms {
				fmt.Println(
					"database",
					m.Time.In(time.Local).Format("01/02 15:04:05"),
					"min:", fmt.Sprintf("%.f", m.Min),
					"max:", fmt.Sprintf("%.f", m.Max),
					"avg:", fmt.Sprintf("%.1f", m.Avg))
			}

			stm := p.Time
			meter := p.Value.(*metric.MeterValue)
			fmt.Println(
				"expected",
				stm.Add(-1*time.Minute).Format("01/02 15:04:05"),
				"min:", fmt.Sprintf("%.f", meter.Min),
				"max:", fmt.Sprintf("%.f", meter.Max),
				"avg:", fmt.Sprintf("%.1f", meter.Sum/float64(meter.Samples)))
			fmt.Println()
		}),
		metric.WithMeta(metric.SeriesInfo{
			MeasureName: "append_rollup",
			MeasureType: metric.MeterType(metric.UnitShort),
			SeriesID:    seriesID,
		}),
	)

	wg := sync.WaitGroup{}
	// start appender
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := db.Connect(ctx, api.WithPassword(user, pass))
		if err != nil {
			fmt.Println("Error connecting to database:", err)
			panic(err)
		}
		defer conn.Close()
		appd, err := conn.Appender(ctx, "tag")
		if err != nil {
			fmt.Println("Error creating appender:", err)
			panic(err)
		}
		defer appd.Close()

		ticker := time.NewTicker(time.Second)
		round := 0
		for ts := range ticker.C {
			ts = ts.Round(time.Second)
			round++
			if round > N {
				break
			}
			go func(round int, ts time.Time) {
				for i := 1; i <= 100; i++ {
					tm := ts.Add(time.Duration((i-1)*10) * time.Millisecond)
					val := float64(round*1000000 + i)
					values := []any{"tag1", tm.UnixNano(), val}
					err := appd.Append(values...)
					if err != nil {
						fmt.Println("Error appending data:", err)
						panic(err)
					}
					measure.AddTime(tm, val)
				}
				if flusher, ok := appd.(api.Flusher); ok {
					flusher.Flush()
				}
				if forceFlush {
					fc, err := db.Connect(ctx, api.WithPassword(user, pass))
					if err != nil {
						fmt.Println("Error connecting to database for flush:", err)
						panic(err)
					}
					defer fc.Close()
					fc.Exec(ctx, `EXEC table_flush(tag)`)
				}
			}(round, ts)
		}
	}()

	wg.Wait()
}

type RollupValue struct {
	Time time.Time
	Min  float64
	Max  float64
	Avg  float64
}

func queryRollupWithConn(ctx context.Context, conn api.Conn, now time.Time) []RollupValue {
	rows, err := conn.Query(ctx, rollupSQL, "tag1", now, "tag1", now)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("no data", now.In(time.Local).Format("01/02 15:04:05"))
			return []RollupValue{}
		}
		fmt.Println("Error querying data:", err)
		panic(err)
	}
	defer rows.Close()

	ret := []RollupValue{}
	for rows.Next() {
		var r RollupValue
		err := rows.Scan(&r.Time, &r.Min, &r.Max, &r.Avg)
		if err != nil {
			fmt.Println("Error scanning data:", err)
			panic(err)
		}
		ret = append(ret, r)
	}
	return ret
}

// TO_DATE('2025-10-15 09:05:59')
const rollupSQL = `SELECT MTIME, MINVALUE, MAXVALUE, VALUE
FROM (
	SELECT
		ROLLUP('minute', 1, time) as mtime,
		MIN(value) as minvalue,
		MAX(value) as maxvalue,
		AVG(value) as value
	FROM 
		tag 
	WHERE
		name = ?
	AND time < DATE_TRUNC('minute', ?) - 1m 
	GROUP BY mtime 
	ORDER BY mtime
	UNION ALL
	SELECT
		DATE_TRUNC('minute', time) as mtime,
		MIN(value) as minvalue,
		MAX(value) as maxvalue,
		AVG(value)
	FROM
		tag
	WHERE
		name = ?
	AND time >= DATE_TRUNC('minute', ?) - 1m
	GROUP BY mtime
	ORDER BY mtime
)
ORDER BY MTIME DESC LIMIT 2`
