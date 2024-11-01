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

	result := conn.Exec(ctx, fmt.Sprintf(`
		create tag table if not exists %s (
			name     varchar(200) primary key,
			time     datetime basetime,
			value    double summarized
	)`, tableName))
	if err := result.Err(); err != nil {
		panic(err)
	}
	fmt.Println(">> result:", result)
}
