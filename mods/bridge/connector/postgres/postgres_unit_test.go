package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"
)

func TestBridgeBasics(t *testing.T) {
	br := NewBridge("p1", "host=127.0.0.1 port=1 user=x password=y dbname=z sslmode=disable")
	if br.Name() != "p1" {
		t.Fatalf("unexpected name: %s", br.Name())
	}
	if br.Type() != "postgres" {
		t.Fatalf("unexpected type: %s", br.Type())
	}
	if br.String() != "bridge 'p1' (postgres)" {
		t.Fatalf("unexpected string: %s", br.String())
	}
	if br.SupportLastInsertId() {
		t.Fatal("postgres should not support last insert id")
	}
	if br.ParameterMarker(2) != "$3" {
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
		{"sql.RawBytes", "", new([]byte)},
		{"[]uint8", "", new([]byte)},
		{"sql.NullBool", "", new(sql.NullBool)},
		{"sql.NullByte", "", new(sql.NullByte)},
		{"sql.NullFloat64", "", new(sql.NullFloat64)},
		{"sql.NullInt16", "", new(sql.NullInt16)},
		{"sql.NullInt32", "", new(sql.NullInt32)},
		{"sql.NullInt64", "", new(sql.NullInt64)},
		{"sql.NullString", "", new(sql.NullString)},
		{"sql.NullTime", "", new(sql.NullTime)},
		{"interface {}", "FLOAT4", new(float32)},
		{"interface {}", "UUID", new(sql.NullString)},
		{"bool", "", new(sql.NullBool)},
		{"int16", "", new(int16)},
		{"int32", "", new(sql.NullInt32)},
		{"int64", "", new(sql.NullInt64)},
		{"float32", "", new(float32)},
		{"float64", "", new(float64)},
		{"string", "", new(sql.NullString)},
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
	if got := NewScanType("interface {}", "OTHER"); got != nil {
		t.Fatalf("expected nil for unmatched interface{}, got=%T", got)
	}
	if got := NewScanType("unknown", ""); got != nil {
		t.Fatalf("expected nil for unknown type, got=%T", got)
	}
}
