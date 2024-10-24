package machsvr_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/api/machsvr"
	"github.com/stretchr/testify/require"
)

func TestColumns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := database.Connect(ctx, machsvr.WithPassword("sys", "manager"))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	result := conn.Exec(ctx, "delete from log")
	if result == nil {
		t.Fatal("result is nil")
		return
	}
	if result.Err() != nil {
		t.Fatalf("result is error: %s", result.Err())
		return
	}

	rows, err := conn.Query(ctx, "select * from log")
	if err != nil {
		panic(err)
	}
	require.NotNil(t, rows, "no rows selected")
	defer rows.Close()
	colNames, colTypes, err := rows.Columns()
	if err != nil {
		panic(err)
	}

	data := []struct {
		name   string
		typ    string
		size   int
		length int
	}{
		{"SHORT", "int16", 2, 0},
		{"USHORT", "int16", 2, 0},
		{"INTEGER", "int32", 4, 0},
		{"UINTEGER", "int32", 4, 0},
		{"LONG", "int64", 8, 0},
		{"ULONG", "int64", 8, 0},
		{"FLOAT", "float", 4, 0},
		{"DOUBLE", "double", 8, 0},
		{"IPV4", "ipv4", 5, 0},
		{"IPV6", "ipv6", 17, 0},
		{"VARCHAR", "string", 20, 0},
		{"TEXT", "string", 67108864, 0},
		{"JSON", "string", 32767, 0},
		{"BINARY", "binary", 67108864, 0},
		{"BLOB", "binary", 67108864, 0},
		{"CLOB", "string", 67108864, 0},
		{"DATETIME", "datetime", 8, 0},
		{"DATETIME_NOW", "datetime", 8, 0},
	}
	for i, cd := range data {
		require.Equal(t, cd.name, colNames[i], "column[%d] name was %q, want %q", i, colNames[i], cd.name)
		require.Equal(t, cd.typ, string(colTypes[i]), "column[%d] %q's type was %q, want %q", i, colNames[i], colTypes[i], cd.typ)
	}
}

func TestExec(t *testing.T) {
	ctx := context.TODO()
	conn, err := database.Connect(ctx, connectOpts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	var one int = 1
	var two int = 2
	var three int16 = 3
	var four int16 = 4
	var five int32 = 5
	var six int32 = 6
	var seven int64 = 7
	var eight int64 = 8
	var f32 float32 = 6.6
	var f64 float64 = 7.77
	var tick time.Time = time.Now()
	var clob01 string = "clob_01"

	result := conn.Exec(ctx, "insert into log values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		0, one, &two, three, &four, five, f32, f64,
		net.ParseIP("127.0.0.1"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:FF"),
		fmt.Sprintf("varchar_1_%s.", randomVarchar()),
		"text_1", "{\"json\":1}", []byte("binary_00"), "blob_01", clob01, 1, tick)
	if result.Err() != nil {
		panic(result.Err())
	}

	result = conn.Exec(ctx, "insert into log values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&six, seven, &eight, 3, 4, 5, &f32, &f64,
		net.ParseIP("127.0.0.2"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:DD"),
		fmt.Sprintf("varchar_2_%s.", randomVarchar()),
		"text_2", "{\"json\":1}", []byte("binary_01"), "blob_01", &clob01, 1, &tick)
	if result.Err() != nil {
		panic(result.Err())
	}

	result = conn.Exec(ctx, "insert into log values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		2, 1, 2, 3, 4, 5, 6.6, 7.77,
		net.ParseIP("127.0.0.3"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:AA"),
		fmt.Sprintf("varchar_3_%s.", randomVarchar()),
		"text_3", "{\"json\":2}", []byte("binary_02"), "blob_01", "clob_01", 1, time.Now())
	if result.Err() != nil {
		panic(result.Err())
	}
}
