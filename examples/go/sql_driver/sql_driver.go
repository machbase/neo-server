//go:build ignore
// +build ignore

package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	driver "github.com/machbase/neo-server/api/machrpc"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	serverAddr := "tcp://127.0.0.1:5655"
	serverCert := filepath.Join(homeDir, ".config", "machbase", "cert", "machbase_cert.pem")
	// This example substitute server's key & cert for the client's key, cert.
	// It is just for the briefness of sample code
	// Client applications **SHOULD** issue a certificate for each one.
	// Please refer to the "API Authentication" section of the documents.
	clientKey := filepath.Join(homeDir, ".config", "machbase", "cert", "machbase_key.pem")
	clientCert := filepath.Join(homeDir, ".config", "machbase", "cert", "machbase_cert.pem")

	// register machbase-neo data source
	driver.RegisterDataSource("neo", &driver.DataSource{
		ServerAddr: serverAddr,
		ServerCert: serverCert,
		ClientKey:  clientKey,
		ClientCert: clientCert,
		User:       "sys",
		Password:   "manager",
	})

	db, err := sql.Open("machbase", "neo")
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
