package main

import (
	"context"
	"fmt"

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

	rows, err := conn.QueryContext(ctx, `select name, time, value from example where name = ? limit 10`, tagName)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	descs := rows.ColumnDescriptions()
	for _, desc := range descs {
		fmt.Println(">> column name:", desc.Name, "type:", desc.Type)
	}
	var name string
	var ts int64 // can use time.Time, string, int64
	var value float64
	for rows.Next() {
		if err := rows.Scan(&name, &ts, &value); err != nil {
			panic(err)
		}
		fmt.Println(">> name", fmt.Sprintf("%q", name), ", time:", ts, ", value:", value)
	}
	if rows.Err() != nil {
		panic(rows.Err())
	}
}
