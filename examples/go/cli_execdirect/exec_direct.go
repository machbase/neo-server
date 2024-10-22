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

	result := conn.ExecDirectContext(ctx, fmt.Sprintf(`
		create tag table if not exists %s (
			name     varchar(200) primary key,
			time     datetime basetime,
			value    double summarized
	)`, tableName))
	if err := result.Err(); err != nil {
		panic(err)
	}
}
