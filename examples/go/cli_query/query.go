package main

import (
	"context"
	"fmt"

	"github.com/machbase/neo-server/api"
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

	rows, err := conn.Query(ctx, `select name, time, value from example where name = ? limit 10`, tagName)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		panic(err)
	}
	for _, col := range columns {
		fmt.Println(">> column name:", col.Name, "type:", col.DataType)
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
}
