package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"sync"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
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

	wg := sync.WaitGroup{}
	for c := 0; c < C; c++ {
		conn, err := db.Connect(ctx, api.WithPassword(user, pass))
		if err != nil {
			fmt.Println("Error connecting to database:", err)
			panic(err)
		}
		defer conn.Close()
		cliConn, ok := conn.(Preparer)
		if !ok {
			fmt.Println("Error: conn does not support Prepare")
			panic(errors.New("invalid connection type"))
		}
		wg.Add(1)
		go func(cliConn Preparer, N int, cliNo int) {
			defer wg.Done()
			for n := 0; n < N; n++ {
				err := cliConn.Prepare(ctx, prepareSQL)
				if err != nil {
					fmt.Println("Error preparing statement:", err)
					panic(err)
				}
			}
		}(cliConn, N, c)
	}
	wg.Wait()
}

type Preparer interface {
	Prepare(ctx context.Context, query string) error
}

const prepareSQL = `SELECT COUNT(*)
FROM (
	SELECT ROLLUP('minute', 1, time) as mtime, AVG(value) 
	FROM tag 
	WHERE name = ? AND 
		time < DATE_TRUNC('minute', TO_DATE('2025-10-15 09:05:59')) - 1m 
	GROUP BY mtime 
	ORDER BY mtime
		UNION ALL
	SELECT DATE_TRUNC('minute', time) as mtime, AVG(value)
	FROM TAG
	WHERE name = ? AND
		time >= DATE_TRUNC('minute', TO_DATE('2025-10-15 09:05:59')) - 1m
	GROUP BY mtime
	ORDER BY mtime
)`
