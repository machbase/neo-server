package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"
)

func TestBridgeBasics(t *testing.T) {
	br := NewBridge("m1", "invalid-dsn")
	if br.Name() != "m1" {
		t.Fatalf("unexpected name: %s", br.Name())
	}
	if br.Type() != "mysql" {
		t.Fatalf("unexpected type: %s", br.Type())
	}
	if br.String() != "bridge 'm1' (mysql)" {
		t.Fatalf("unexpected string: %s", br.String())
	}
	if !br.SupportLastInsertId() {
		t.Fatal("mysql should support last insert id")
	}
	if br.ParameterMarker(42) != "?" {
		t.Fatalf("unexpected marker: %s", br.ParameterMarker(42))
	}
	if _, err := br.Connect(context.Background()); err == nil {
		t.Fatal("expected connect error before register")
	}
	if err := br.BeforeRegister(); err == nil {
		t.Fatal("expected before register to fail without mysql server")
	}
	if br.DB() != nil {
		t.Fatal("db should stay nil when before register fails")
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
		{"sql.RawBytes", "VARCHAR", new(sql.NullString)},
		{"sql.RawBytes", "CHAR", new(sql.NullString)},
		{"sql.RawBytes", "TEXT", new(sql.NullString)},
		{"sql.RawBytes", "BLOB", new([]byte)},
		{"[]uint8", "", new([]byte)},
		{"sql.NullBool", "", new(sql.NullBool)},
		{"sql.NullByte", "", new(sql.NullByte)},
		{"sql.NullFloat64", "", new(sql.NullFloat64)},
		{"sql.NullInt16", "", new(sql.NullInt16)},
		{"sql.NullInt32", "", new(sql.NullInt32)},
		{"sql.NullInt64", "", new(sql.NullInt64)},
		{"sql.NullString", "", new(sql.NullString)},
		{"sql.NullTime", "DATE", new(sql.NullString)},
		{"sql.NullTime", "DATETIME", new(sql.NullTime)},
		{"bool", "", new(bool)},
		{"int16", "", new(int16)},
		{"int32", "", new(int32)},
		{"int64", "", new(int64)},
		{"float32", "", new(float32)},
		{"float64", "", new(float64)},
		{"string", "", new(string)},
		{"time.Time", "", new(sql.NullTime)},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%s", tc.reflectType, tc.dbType), func(t *testing.T) {
			got := NewScanType(tc.reflectType, tc.dbType)
			if reflect.TypeOf(got) != reflect.TypeOf(tc.wantType) {
				t.Fatalf("type mismatch: got=%T want=%T", got, tc.wantType)
			}
		})
	}
	if got := NewScanType("unknown", ""); got != nil {
		t.Fatalf("expected nil for unknown type, got=%T", got)
	}
}
