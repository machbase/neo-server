//go:build run

package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/api/machrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Use insecure transport
// server should start with `--grpc-insecure=true` to allow insecure connection
// (default is false)
func main() {
	serverAddr := "tcp://127.0.0.1:5655"

	// Explicitly customize the transport to insecure
	sql.Register("neo", &machrpc.Driver{
		ConnProvider: func() (*grpc.ClientConn, error) {
			return grpc.NewClient("127.0.0.1:5655",
				grpc.WithTransportCredentials(insecure.NewCredentials()))
		},
	})

	db, err := sql.Open("neo", fmt.Sprintf("server=%s; user=sys; password=manager;", serverAddr))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// INSERT with Exec
	tag := "tag01"
	var result sql.Result
	result, err = db.Exec("INSERT INTO EXAMPLE(name, time, value) VALUES(?, ?, ?)", tag, time.Now(), 1.234)
	if err != nil {
		panic(err)
	}
	fmt.Println("INSERT: ", result)

	// QueryRow
	row := db.QueryRow("SELECT count(*) FROM EXAMPLE WHERE name = ?", tag)
	if row.Err() != nil {
		panic(row.Err())
	}
	var count int
	if err = row.Scan(&count); err != nil {
		panic(err)
	}
	fmt.Println("count:", count)

	// Query
	rows, err := db.Query("SELECT name, time, value FROM EXAMPLE WHERE name = ? ORDER BY TIME DESC", tag)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var ts time.Time
		var value float64
		rows.Scan(&name, &ts, &value)
		fmt.Println("name:", name, "time:", ts.Local().String(), "value:", value)
	}
}
