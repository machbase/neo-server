package main

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
)

const (
	machPort = 5656
	machHost = "127.0.0.1"
	machUser = "SYS"
	machPass = "MANAGER"

	tableName = "example"
	tagName   = "hello-world"
)

func main() {
	db, err := machcli.NewDatabase(&machcli.Config{Host: machHost, Port: machPort})
	if err != nil {
		panic(err)
	}
	defer db.Close()

	ctx := context.TODO()

	conn, err := db.Connect(ctx, api.WithPassword(machUser, machPass))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	appender, err := conn.Appender(ctx, tableName)
	if err != nil {
		panic(err)
	}

	count := 10000
	ts := time.Now().Add(time.Duration(time.Duration(-1*count) * time.Second))
	for i := 0; i < 10000; i++ {
		if i%1000 == 0 {
			if flusher, ok := appender.(api.Flusher); ok {
				if err := flusher.Flush(); err != nil {
					panic(err)
				}
			}
		}
		if err := appender.Append(tagName, ts.Add(time.Duration(i)*time.Second), i); err != nil {
			panic(err)
		}
	}
	s, f, err := appender.Close()
	if err != nil {
		panic(err)
	}
	fmt.Println(">> success:", s, ", fail:", f)
}
