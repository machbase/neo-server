package main

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-server/api/machcli"
)

const (
	machPort = 5656
	machHost = "127.0.0.1"
	machUser = "SYS"
	machPass = "MANAGER"

	tableName = "example"
	tagName   = "helloworld"
)

func main() {
	env, err := machcli.NewEnv()
	if err != nil {
		panic(err)
	}
	defer env.Close()

	ctx := context.TODO()

	conn, err := env.Connect(ctx, machcli.WithHost(machHost, machPort), machcli.WithPassword(machUser, machPass))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	appender, err := conn.Appender(tableName, machcli.WithErrorCheckCount(10))
	if err != nil {
		panic(err)
	}

	count := 10000
	ts := time.Now().Add(time.Duration(time.Duration(-1*count) * time.Second))
	for i := 0; i < 10000; i++ {
		if i%1000 == 0 {
			if err := appender.Flush(); err != nil {
				panic(err)
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
