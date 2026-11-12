package server

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"time"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
)

func TestVectorIntegration(t *testing.T) {
	dsn := "tcp://sys:manager@" + strings.TrimPrefix(machServerAddress, "tcp://")
	db, err := sql.Open(client.DefaultDriverName, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	const table = "GO_VECTOR_4211"
	if _, err := db.ExecContext(ctx, "CREATE TRANSACTION TABLE "+table+"(ID INTEGER PRIMARY KEY,V VECTOR(3))"); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), "DROP TABLE "+table)

	stmt, err := db.PrepareContext(ctx, "INSERT INTO "+table+" VALUES(?,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.ExecContext(ctx, int32(1), api.Vector{1, 2, 3}); err != nil {
		t.Fatalf("VECTOR prepared bind: %v", err)
	}
	if _, err := stmt.ExecContext(ctx, int32(2), (*api.Vector)(nil)); err != nil {
		t.Fatalf("VECTOR NULL bind: %v", err)
	}
	if _, err := stmt.ExecContext(ctx, int32(4), api.Vector(nil)); err != nil {
		t.Fatalf("VECTOR nil slice bind: %v", err)
	}
	if _, err := stmt.ExecContext(ctx, int32(8), "[1,2,3]"); err != nil {
		t.Fatalf("VECTOR JSON bind: %v", err)
	}
	if _, err := stmt.ExecContext(ctx, int32(9), []float32{1, 2, 3}); err != nil {
		t.Fatalf("VECTOR float32 slice bind: %v", err)
	}
	if _, err := stmt.ExecContext(ctx, int32(5), api.Vector{1, 2}); err == nil {
		t.Fatal("wrong VECTOR dimension accepted")
	}
	rows, err := db.QueryContext(ctx, "SELECT V FROM "+table+" WHERE ID=1")
	if err != nil {
		t.Fatal(err)
	}
	columnTypes, err := rows.ColumnTypes()
	rows.Close()
	if err != nil || len(columnTypes) != 1 || columnTypes[0].DatabaseTypeName() != "VECTOR" ||
		columnTypes[0].ScanType() != reflect.TypeOf(api.Vector{}) {
		t.Fatalf("VECTOR column metadata = %v, %v", columnTypes, err)
	}
	buffer, err := api.ColumnTypeVector.MakeBuffer()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT V FROM "+table+" WHERE ID=1").Scan(buffer); err != nil ||
		!reflect.DeepEqual(*buffer.(**api.Vector), &api.Vector{1, 2, 3}) {
		t.Fatalf("VECTOR metadata buffer = %v, %v", buffer, err)
	}

	var got api.Vector
	if err := db.QueryRowContext(ctx, "SELECT V FROM "+table+" WHERE ID=1").Scan(&got); err != nil ||
		!reflect.DeepEqual(got, api.Vector{1, 2, 3}) {
		t.Fatalf("VECTOR query = %v, %v", got, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT V FROM "+table+" WHERE ID=2").Scan(&got); err != nil || got != nil {
		t.Fatalf("VECTOR NULL query = %v, %v", got, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT V FROM "+table+" WHERE ID=4").Scan(&got); err != nil || got != nil {
		t.Fatalf("VECTOR nil slice query = %v, %v", got, err)
	}
	for _, id := range []int32{8, 9} {
		if err := db.QueryRowContext(ctx, "SELECT V FROM "+table+" WHERE ID=?", id).Scan(&got); err != nil ||
			!reflect.DeepEqual(got, api.Vector{1, 2, 3}) {
			t.Fatalf("VECTOR alternative bind ID=%d = %v, %v", id, got, err)
		}
	}
	if err := db.QueryRowContext(ctx, "SELECT TO_VECTOR('[3,2,1]',3)").Scan(&got); err != nil ||
		!reflect.DeepEqual(got, api.Vector{3, 2, 1}) {
		t.Fatalf("dynamic VECTOR query = %v, %v", got, err)
	}

	appender := &client.Appender{}
	if err := appender.Connect(ctx, dsn, table, "ID", "V"); err != nil {
		t.Fatalf("VECTOR append open: %v", err)
	}
	if err := appender.Append(int32(6), api.Vector{1}); err == nil {
		t.Fatal("wrong VECTOR append dimension accepted")
	}
	if err := appender.Append(int32(3), []float32{0.5, 0.25, -1}); err != nil {
		t.Fatalf("VECTOR append: %v", err)
	}
	if err := appender.Append(int32(6), api.Vector(nil)); err != nil {
		t.Fatalf("VECTOR NULL append: %v", err)
	}
	if err := appender.Append(int32(7), []float64{0.5, 0.25, -1}); err != nil {
		t.Fatalf("VECTOR float64 append: %v", err)
	}
	if _, _, err := appender.Close(); err != nil {
		t.Fatalf("VECTOR append close: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT V FROM "+table+" WHERE ID=3").Scan(&got); err != nil ||
		!reflect.DeepEqual(got, api.Vector{0.5, 0.25, -1}) {
		t.Fatalf("appended VECTOR = %v, %v", got, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT V FROM "+table+" WHERE ID=6").Scan(&got); err != nil || got != nil {
		t.Fatalf("appended VECTOR NULL = %v, %v", got, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT V FROM "+table+" WHERE ID=7").Scan(&got); err != nil ||
		!reflect.DeepEqual(got, api.Vector{0.5, 0.25, -1}) {
		t.Fatalf("appended float64 VECTOR = %v, %v", got, err)
	}
}

func TestVectorLargeWireIntegration(t *testing.T) {
	dsn := "tcp://sys:manager@" + strings.TrimPrefix(machServerAddress, "tcp://")
	db, err := sql.Open(client.DefaultDriverName, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	const table = "GO_VECTOR_LARGE_4211"
	if _, err := db.ExecContext(ctx, "CREATE TRANSACTION TABLE "+table+"(ID INTEGER PRIMARY KEY,V VECTOR(17000))"); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), "DROP TABLE "+table)
	vector := make(api.Vector, 17000) // 68 KiB, larger than one CMI packet body.
	vector[0], vector[len(vector)-1] = 1.5, -2.25
	if _, err := db.ExecContext(ctx, "INSERT INTO "+table+" VALUES(?,?)", int32(1), vector); err != nil {
		t.Fatalf("large VECTOR bind: %v", err)
	}
	var result api.Vector
	if err := db.QueryRowContext(ctx, "SELECT V FROM "+table+" WHERE ID=1").Scan(&result); err != nil {
		t.Fatalf("large VECTOR fetch: %v", err)
	}
	if len(result) != len(vector) {
		t.Fatalf("large VECTOR result dimension=%d, want=%d", len(result), len(vector))
	}
	if result[0] != 1.5 || result[len(result)-1] != -2.25 {
		t.Fatalf("large VECTOR result dimension=%d first=%v last=%v", len(result), result[0], result[len(result)-1])
	}
}
