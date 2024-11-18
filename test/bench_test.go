package test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/stretchr/testify/require"
)

// go test -benchmem -run=^$ -bench ^BenchmarkAppend$ github.com/machbase/neo-server/test -benchtime=1m
//
// 2022.12.13 mac-mini(m1) utm-ubuntu (4 core, 4G mem)
// BenchmarkAppend-4         424443            167131 ns/op             560 B/op          9 allocs/op
//
// 2024.10.28 mac-mini(m1) native
// BenchmarkAppend-8       27475498              2860 ns/op             252 B/op         10 allocs/op
func BenchmarkAppend(b *testing.B) {
	var memBefore runtime.MemStats
	var memAfter runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	db, err := machsvr.NewDatabase()
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

	idgen := uuid.NewGen()

	for i := 0; i < b.N; i++ {
		id, _ := idgen.NewV6()
		idstr := id.String()
		jsonstr := `{"some":"jsondata, more length require 12345678901234567890abcdefghijklmn"}`
		appender.Append("benchmark.tagname", time.Now(), 1.001*float32(i), idstr, jsonstr)
	}
	appender.Close()

	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	b.Log("HeapInuse :", memAfter.HeapInuse-memBefore.HeapInuse)
	b.Log("TotalAlloc:", memAfter.TotalAlloc-memBefore.TotalAlloc)
	b.Log("Mallocs   :", memAfter.Mallocs-memBefore.Mallocs)
	b.Log("Frees     :", memAfter.Frees-memBefore.Frees)
	b.Log("")
}

// go test -benchmem -run=^$ -bench ^BenchmarkSelect$ github.com/machbase/neo-server/test -benchtime=1m
//
// 2022.12.13 mac-mini(m1) utm-ubuntu (4 core, 4G mem)
// BenchmarkSelect-4          17163           4625124 ns/op           40540 B/op       2711 allocs/op
//
// 2024.10.28 mac-mini(m1) native
// BenchmarkSelect-8           6807          14686285 ns/op            2045 B/op         48 allocs/op

func BenchmarkSelect(b *testing.B) {
	db, err := machsvr.NewDatabase()
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

	idgen := uuid.NewGen()

	for i := 0; i < 10000; i++ {
		id, _ := idgen.NewV6()
		idstr := id.String()
		jsonstr := `{"some":"jsondata, more length require 12345678901234567890abcdefghijklmn"}`
		appender.Append("benchmark.tagname", time.Now(), 1.001*float32(i), idstr, jsonstr)
	}
	appender.Close()

	var memBefore runtime.MemStats
	var memAfter runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&memBefore)

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

	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	b.Log("HeapInuse :", memAfter.HeapInuse-memBefore.HeapInuse)
	b.Log("TotalAlloc:", memAfter.TotalAlloc-memBefore.TotalAlloc)
	b.Log("Mallocs   :", memAfter.Mallocs-memBefore.Mallocs)
	b.Log("Frees     :", memAfter.Frees-memBefore.Frees)
	b.Log("")
}
