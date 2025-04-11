package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/stretchr/testify/require"
)

// go test -benchmem -run=^$ -bench ^BenchmarkAppend$ github.com/machbase/neo-server/v8/test -benchtime=1m
//
// 2022.12.13 mac-mini(m1) utm-ubuntu (4 core, 4G mem)
// BenchmarkAppend-4         424443            167131 ns/op             560 B/op          9 allocs/op
//
// 2024.10.28 mac-mini(m1) native
// BenchmarkAppend-8       27475498              2860 ns/op             252 B/op         10 allocs/op
//
// 2024.11.29 mac-mini(m1) native
// BenchmarkAppend-8       26549180              3011 ns/op             252 B/op         10 allocs/op
//
// 2025.04.11
// cpu: mac-mini(m1) native
// BenchmarkAppend-8       28999105              2741 ns/op             252 B/op         10 allocs/op
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkAppend-24       7882057              9398 ns/op             252 B/op         10 allocs/op

func BenchmarkAppend(b *testing.B) {
	db, err := machsvr.NewDatabase(machsvr.DatabaseOption{})
	require.NoError(b, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		b.Error(err.Error())
	}
	defer conn.Close()

	appender, err := conn.Appender(ctx, benchmarkTableName)
	require.Nil(b, err)

	var appendFunc func(...any) error
	if syncAppender, ok := appender.(interface{ AppendSync(...any) error }); ok {
		appendFunc = syncAppender.AppendSync
	} else {
		appendFunc = appender.Append
	}

	idGen := uuid.NewGen()

	for i := 0; i < b.N; i++ {
		id, _ := idGen.NewV6()
		idStr := id.String()
		jsonStr := `{"some":"jsondata, more length require 12345678901234567890abcdefghijklmn"}`
		appendFunc("benchmark.tagname", time.Now(), 1.001*float32(i), idStr, jsonStr)
	}
	appender.Close()
}

// go test -benchmem -run=^$ -bench ^BenchmarkSelect$ github.com/machbase/neo-server/v8/test -benchtime=1m
//
// 2022.12.13 mac-mini(m1) utm-ubuntu (4 core, 4G mem)
// BenchmarkSelect-4          17163           4625124 ns/op           40540 B/op       2711 allocs/op
//
// 2024.10.28 mac-mini(m1) native
// BenchmarkSelect-8           6807          14686285 ns/op            2045 B/op         48 allocs/op
//
// 2024.11.29 mac-mini(m1) native
// BenchmarkSelect-8           6524          14599373 ns/op            2139 B/op         49 allocs/op
//
// 2025.04.11
// cpu: mac-mini(m1) native
// BenchmarkSelect-8           6546          14871775 ns/op            2401 B/op         54 allocs/op
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkSelect-24          5942          13417658 ns/op            2418 B/op         56 allocs/op

func BenchmarkSelect(b *testing.B) {
	db, err := machsvr.NewDatabase(machsvr.DatabaseOption{})
	require.NoError(b, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		b.Error(err.Error())
	}
	defer conn.Close()

	appender, err := conn.Appender(ctx, benchmarkTableName)
	require.Nil(b, err)

	idGen := uuid.NewGen()

	for i := 0; i < 10000; i++ {
		id, _ := idGen.NewV6()
		idStr := id.String()
		jsonStr := `{"some":"jsondata, more length require 12345678901234567890abcdefghijklmn"}`
		appender.Append("benchmark.tagname", time.Now(), 1.001*float32(i), idStr, jsonStr)
	}
	appender.Close()

	var prevId = ""
	for i := 0; i < b.N; i++ {
		rows, err := conn.Query(ctx, fmt.Sprintf("select name, time, value, id, jsondata from %s where id > ? limit 100", benchmarkTableName), prevId)
		require.Nil(b, err)

		var sName string
		var sTime time.Time
		var sValue float64
		var sJson string
		var fetched bool

		for rows.Next() {
			err = rows.Scan(&sName, &sTime, &sValue, &prevId, &sJson)
			require.Nil(b, err)
			fetched = true
		}
		rows.Close()

		if !fetched {
			prevId = ""
		}
	}
}
