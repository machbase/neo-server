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

	row := conn.QueryRowContext(ctx, `select name, count(*) as c from example group by name having name = ?`, tagName)
	if row.Err() != nil {
		panic(row.Err())
	}

	var name string
	var count int
	if err := row.Scan(&name, &count); err != nil {
		panic(err)
	}
	fmt.Println(">> name", fmt.Sprintf("%q", name), ", count(*):", count)
}
