package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
)

func main() {
	// Configure database connection
	conf := &machcli.Config{
		Host:         "127.0.0.1",
		Port:         5656,
		MaxOpenConn:  10,
		MaxOpenQuery: 5,
	}

	db, err := machcli.NewDatabase(conf)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	// Connect to database
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Create a sample table
	result := conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS sample_data (
            name VARCHAR(100),
            time DATETIME,
            value DOUBLE
        )
    `)
	if err := result.Err(); err != nil {
		log.Fatal(err)
	}

	// Insert sample data
	for i := 0; i < 5; i++ {
		result := conn.Exec(ctx,
			`INSERT INTO sample_data VALUES (?, ?, ?)`,
			fmt.Sprintf("sensor_%d", i), time.Now(), float64(i)*1.5)
		if err := result.Err(); err != nil {
			log.Fatal(err)
		}
	}

	// Query the data
	rows, err := conn.Query(ctx, `SELECT name, time, value FROM sample_data ORDER BY time`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Retrieved data:")
	for rows.Next() {
		var name string
		var tm time.Time
		var value float64

		if err := rows.Scan(&name, &tm, &value); err != nil {
			log.Fatal(err)
		}
		tm = tm.Local()
		fmt.Printf("Name: %s, Time: %s, Value: %.2f\n",
			name, tm.Format(time.RFC3339), value)
	}
}
