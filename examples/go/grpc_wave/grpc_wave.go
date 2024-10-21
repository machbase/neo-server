//go:build ignore
// +build ignore

package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"time"

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
	var conn *machrpc.Conn
	if c, err := cli.Connect(ctx, machrpc.WithPassword("sys", "manager")); err != nil {
		panic(err)
	} else {
		conn = c
	}
	defer cli.Close()

	sqlText := `insert into example (name, time, value) values (?, ?, ?)`

	for ts := range time.Tick(500 * time.Millisecond) {
		delta := float64(ts.UnixMilli()%15000) / 15000
		theta := 2 * math.Pi * delta
		sin, cos := math.Sin(theta), math.Cos(theta)
		if result := conn.Exec(ctx, sqlText, "wave.sin", ts, sin); result.Err() != nil {
			panic(result.Err())
		}
		if result := conn.Exec(ctx, sqlText, "wave.cos", ts, cos); result.Err() != nil {
			panic(result.Err())
		}
	}
}
