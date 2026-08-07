package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"
)

func TestBridgeBasicsAndURLParsing(t *testing.T) {
	path := "server=127.0.0.1:1433 user=sa password=pw database=master encrypt=disable"
	br := NewBridge("s1", path)

	if br.Name() != "s1" {
		t.Fatalf("unexpected name: %s", br.Name())
	}
	if br.Type() != "mssql" {
		t.Fatalf("unexpected type: %s", br.Type())
	}
	if br.String() != "bridge 's1' (mssql)" {
		t.Fatalf("unexpected string: %s", br.String())
	}
	if br.SupportLastInsertId() {
		t.Fatal("mssql should not support last insert id")
	}
	if br.ParameterMarker(2) != "@p3" {
		t.Fatalf("unexpected marker: %s", br.ParameterMarker(2))
	}
	if _, err := br.Connect(context.Background()); err == nil {
		t.Fatal("expected connect error before register")
	}

	if err := br.BeforeRegister(); err != nil {
		t.Fatalf("before register should succeed on open: %v", err)
	}
	if br.DB() == nil {
		t.Fatal("db should be initialized")
	}
	if br.ParsedURL() == nil {
		t.Fatal("parsed url should be set")
	}
	q := br.ParsedURL().Query()
	if q.Get("user id") != "sa" || q.Get("password") != "pw" || q.Get("database") != "master" {
		t.Fatalf("unexpected parsed query: %v", q)
	}
	if q.Get("dial timeout") != "3" || q.Get("connection timeout") != "5" || q.Get("app name") != "neo-bridge" {
		t.Fatalf("default query params missing: %v", q)
	}
	if err := br.AfterUnregister(); err != nil {
		t.Fatalf("after unregister failed: %v", err)
	}
	if err := br.AfterUnregister(); err != nil {
		t.Fatalf("after unregister on nil should pass: %v", err)
	}
}

func TestNewScanTypeMatrix(t *testing.T) {
	cases := []struct {
		reflectType string
		dbType      string
		wantType    any
	}{
		{"", "INT", new(sql.NullInt64)},
		{"", "SMALLINT", new(sql.NullInt64)},
		{"", "TINYINT", new(sql.NullInt64)},
		{"", "BIGINT", new(sql.NullInt64)},
		{"", "DECIMAL", new(sql.NullFloat64)},
		{"", "NUMERIC", new(sql.NullFloat64)},
		{"", "MONEY", new(sql.NullFloat64)},
		{"", "SMALLMONEY", new(sql.NullFloat64)},
		{"", "REAL", new(sql.NullFloat64)},
		{"", "FLOAT", new(sql.NullFloat64)},
		{"", "BIT", new(sql.NullBool)},
		{"", "VARCHAR", new(sql.NullString)},
		{"", "TEXT", new(sql.NullString)},
		{"", "NCHAR", new(sql.NullString)},
		{"", "NVARCHAR", new(sql.NullString)},
		{"", "DATETIME", new(sql.NullTime)},
		{"sql.RawBytes", "UNKNOWN", new([]byte)},
		{"[]uint8", "UNKNOWN", new([]byte)},
		{"sql.NullBool", "UNKNOWN", new(sql.NullBool)},
		{"sql.NullByte", "UNKNOWN", new(sql.NullByte)},
		{"sql.NullFloat64", "UNKNOWN", new(sql.NullFloat64)},
		{"sql.NullInt16", "UNKNOWN", new(sql.NullInt16)},
		{"sql.NullInt32", "UNKNOWN", new(sql.NullInt32)},
		{"sql.NullInt64", "UNKNOWN", new(sql.NullInt64)},
		{"sql.NullString", "UNKNOWN", new(sql.NullString)},
		{"sql.NullTime", "UNKNOWN", new(sql.NullTime)},
		{"bool", "UNKNOWN", new(bool)},
		{"int16", "UNKNOWN", new(int16)},
		{"int32", "UNKNOWN", new(int32)},
		{"int64", "UNKNOWN", new(int64)},
		{"float32", "UNKNOWN", new(float32)},
		{"float64", "UNKNOWN", new(float64)},
		{"string", "UNKNOWN", new(string)},
		{"time.Time", "UNKNOWN", new(sql.NullTime)},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%s", tc.reflectType, tc.dbType), func(t *testing.T) {
			got := NewScanType(tc.reflectType, tc.dbType)
			if reflect.TypeOf(got) != reflect.TypeOf(tc.wantType) {
				t.Fatalf("type mismatch: got=%T want=%T", got, tc.wantType)
			}
		})
	}
	if got := NewScanType("unknown", "UNKNOWN"); got != nil {
		t.Fatalf("expected nil for unknown type, got=%T", got)
	}
}
