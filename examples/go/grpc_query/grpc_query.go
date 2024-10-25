//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/machrpc"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	serverAddr := "127.0.0.1:5655"
	serverCert := filepath.Join(homeDir, ".config", "machbase", "cert", "machbase_cert.pem")

	// This example substitute server's key & cert for the client's key, cert.
	// It is just for the briefness of sample code
	// Client applications **SHOULD** issue a certificate for each one.
	// Please refer to the "API Authentication" section of the documents.
	clientKey := filepath.Join(homeDir, ".config", "machbase", "cert", "machbase_key.pem")
	clientCert := filepath.Join(homeDir, ".config", "machbase", "cert", "machbase_cert.pem")

	cli, err := machrpc.NewClient(&machrpc.Config{
		ServerAddr: serverAddr,
		Tls: &machrpc.TlsConfig{
			ClientKey:  clientKey,
			ClientCert: clientCert,
			ServerCert: serverCert,
		},
	})
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	var ctx = context.TODO()
	var conn api.Conn

	if c, err := cli.Connect(ctx, api.WithPassword("sys", "manager")); err != nil {
		panic(err)
	} else {
		conn = c
	}
	defer conn.Close()

	sqlText := `select name, time, value from example limit ?`
	rows, err := conn.Query(ctx, sqlText, 3)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var ts time.Time
		var value float64
		err = rows.Scan(&name, &ts, &value)
		if err != nil {
			panic(err)
		}
		fmt.Println("name:", name, "time:", ts, "value:", value)
	}
}
