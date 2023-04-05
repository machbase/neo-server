package wiresvr_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	pgx "github.com/jackc/pgx/v5"
)

func TestPgwire(t *testing.T) {
	connstr := "postgres://username:password@127.0.0.1:5651/machbase"
	conn, err := pgx.Connect(context.Background(), connstr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	tagName := "wave.sin"
	limit := 10
	rows, err := conn.Query(context.Background(),
		"SELECT name, time, value FROM example WHERE name = ? ORDER BY TIME DESC LIMIT ?", tagName, limit)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var ts time.Time
		var val float64

		rows.Scan(&name, &ts, &val)
		fmt.Println("---", name, ts, val)
	}
}
