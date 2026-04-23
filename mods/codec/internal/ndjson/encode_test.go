package ndjson_test

import (
	"bytes"
	"database/sql"
	gojson "encoding/json"
	"math"
	"net"
	"strings"
	"testing"
	"time"

	jsonEnc "github.com/machbase/neo-server/v8/mods/codec/internal/json"
	"github.com/machbase/neo-server/v8/mods/codec/internal/ndjson"
	"github.com/stretchr/testify/require"
)

// 2026-04-23 - direct NDJSON row encoding with trimmed float formatting
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkNDJsonEncoder-24          53876             24156 ns/op            3280 B/op         64 allocs/op
// BenchmarkNDJsonEncoder-24          48442             24049 ns/op            3280 B/op         64 allocs/op
// BenchmarkNDJsonEncoder-24          49390             23963 ns/op            3280 B/op         64 allocs/op
// BenchmarkNDJsonEncoder-24          49273             24242 ns/op            3280 B/op         64 allocs/op
// BenchmarkNDJsonEncoder-24          48763             23263 ns/op            3280 B/op         64 allocs/op

func BenchmarkNDJsonEncoder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		out := &bytes.Buffer{}

		enc := ndjson.NewEncoder()
		enc.SetOutputStream(out)
		enc.SetTimeformat("Default")
		enc.SetTimeLocation(time.UTC)
		enc.SetColumnTypes("string", "datetime", "double")
		enc.SetColumns("name", "time", "value")
		require.NoError(b, enc.Open())
		for row := 0; row < 10; row++ {
			require.NoError(b, enc.AddRow([]any{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002}))
		}
		enc.Close()
	}
}

func TestJsonEncode(t *testing.T) {
	tests := []struct {
		input      [][]any
		expect     string
		timeformat string
		tz         *time.Location
		rownum     bool
	}{
		{
			input: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002},
			},
			expect: `{"name":"my-car","time":1670380342000000000,"value":1.0001}
{"name":"my-car","time":1670380343000000000,"value":2.0002}

`,
		},
		{
			input: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002},
			},
			expect: `{"name":"my-car","time":1670380342,"value":1.0001}
{"name":"my-car","time":1670380343,"value":2.0002}

`,
			timeformat: "s",
		},
		{
			input: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002},
			},
			expect: `{"ROWNUM":1,"name":"my-car","time":"2022/12/07 02:32:22","value":1.0001}
{"ROWNUM":2,"name":"my-car","time":"2022/12/07 02:32:23","value":2.0002}

`,
			timeformat: "2006/01/02 15:04:05",
			tz:         time.UTC,
			rownum:     true,
		},
	}

	for _, tt := range tests {
		out := &bytes.Buffer{}

		enc := ndjson.NewEncoder()
		enc.SetOutputStream(out)
		enc.SetTimeformat(tt.timeformat)
		enc.SetTimeLocation(tt.tz)
		enc.SetColumnTypes("string", "datetime", "double")
		enc.SetColumns("name", "time", "value")
		enc.SetRownum(tt.rownum)
		enc.Open()
		for _, row := range tt.input {
			enc.AddRow(row)
		}
		enc.Close()
		require.Equal(t, tt.expect, out.String())
	}
}

func TestPrecisionFloat64MarshalJSON(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		expect string
	}{
		{
			name:   "dynamic-significant-digits-trims-trailing-zeros",
			value:  12.3400,
			expect: "12.34",
		},
		{
			name:   "integer-like-float-without-fixed-decimals",
			value:  10.0,
			expect: "10",
		},
		{
			name:   "normalize-negative-zero",
			value:  math.Copysign(0, -1),
			expect: "0",
		},
		{
			name:   "nan-as-string-token",
			value:  math.NaN(),
			expect: `"NaN"`,
		},
		{
			name:   "negative-infinity-as-string-token",
			value:  math.Inf(-1),
			expect: `"-Inf"`,
		},
		{
			name:   "positive-infinity-as-string-token",
			value:  math.Inf(1),
			expect: `"+Inf"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := jsonEnc.AppendJSONValue(nil, tt.value, -1)
			require.NoError(t, err)
			require.Equal(t, tt.expect, string(b))
		})
	}
}

func TestNdjsonEncodeFloatFormatting(t *testing.T) {
	tests := []struct {
		name        string
		value       float64
		expectField string
	}{
		{
			name:        "trailing-zeros-trimmed",
			value:       12.3400,
			expectField: `"value":12.34`,
		},
		{
			name:        "integer-like-float",
			value:       10.0,
			expectField: `"value":10`,
		},
		{
			name:        "nan-as-quoted-string",
			value:       math.NaN(),
			expectField: `"value":"NaN"`,
		},
		{
			name:        "negative-inf-as-quoted-string",
			value:       math.Inf(-1),
			expectField: `"value":"-Inf"`,
		},
		{
			name:        "positive-inf-as-quoted-string",
			value:       math.Inf(1),
			expectField: `"value":"+Inf"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			enc := ndjson.NewEncoder()
			enc.SetOutputStream(out)
			enc.SetColumnTypes("double")
			enc.SetColumns("value")
			enc.Open()
			require.NoError(t, enc.AddRow([]any{tt.value}))
			enc.Close()
			require.True(t, strings.Contains(out.String(), tt.expectField),
				"output %q does not contain %q", out.String(), tt.expectField)
		})
	}
}

func TestNdjsonEncodeFloatFormattingWithPrecision(t *testing.T) {
	out := &bytes.Buffer{}
	enc := ndjson.NewEncoder()
	enc.SetOutputStream(out)
	enc.SetPrecision(2)
	enc.SetColumnTypes("double", "double", "double")
	enc.SetColumns("runtime", "negzero", "whole")
	require.NoError(t, enc.Open())
	require.NoError(t, enc.AddRow([]any{
		20.55 - 22.2,
		math.Copysign(0, -1),
		float64(10),
	}))
	enc.Close()

	require.Equal(t, "{\"runtime\":-1.65,\"negzero\":0.00,\"whole\":10.00}\n\n", out.String())
}

type flushCloseBuffer struct {
	bytes.Buffer
	flushCount int
	closeCount int
}

func (b *flushCloseBuffer) Flush() error {
	b.flushCount++
	return nil
}

func (b *flushCloseBuffer) Close() error {
	b.closeCount++
	return nil
}

func TestNdjsonUtilityPaths(t *testing.T) {
	out := &flushCloseBuffer{}
	enc := ndjson.NewEncoder()
	require.Equal(t, "application/x-ndjson", enc.ContentType())
	enc.SetOutputStream(out)
	enc.SetHeader(true)
	enc.SetHeading(false)
	enc.SetColumns("name")
	enc.SetColumnTypes("string")
	require.NoError(t, enc.Open())
	enc.Flush(false)
	enc.Close()
	require.Equal(t, 1, out.flushCount)
	require.Equal(t, 1, out.closeCount)
}

func TestNdjsonEncodeTypedRowsAndNulls(t *testing.T) {
	out := &bytes.Buffer{}
	enc := ndjson.NewEncoder()
	enc.SetOutputStream(out)
	enc.SetColumns("ptime", "pfloat64", "pfloat32", "ip", "nbool", "nbyte", "nfloat64", "nint16", "nint32", "nfloat32", "nint64", "nstring", "ntime", "nip", "plain")
	enc.SetColumnTypes("datetime", "double", "double", "string", "bool", "byte", "double", "int16", "int32", "double", "int64", "string", "datetime", "string", "string")
	require.NoError(t, enc.Open())

	tm := time.Unix(1700000000, 0).UTC()
	f64 := 3.5
	f32 := float32(1.25)
	ip := net.ParseIP("127.0.0.1")
	require.NoError(t, enc.AddRow([]any{
		&tm,
		&f64,
		&f32,
		&ip,
		&sql.NullBool{Bool: true, Valid: true},
		&sql.NullByte{Byte: 7, Valid: true},
		&sql.NullFloat64{Float64: 8.5, Valid: true},
		&sql.NullInt16{Int16: 16, Valid: true},
		&sql.NullInt32{Int32: 32, Valid: true},
		&sql.Null[float32]{V: float32(2.5), Valid: true},
		&sql.NullInt64{Int64: 64, Valid: true},
		&sql.NullString{String: "text", Valid: true},
		&sql.NullTime{Time: tm, Valid: true},
		&sql.Null[net.IP]{V: ip, Valid: true},
		map[string]any{"nested": 1},
	}))
	require.NoError(t, enc.AddRow([]any{
		nil,
		nil,
		nil,
		nil,
		&sql.NullBool{Valid: false},
		&sql.NullByte{Valid: false},
		&sql.NullFloat64{Valid: false},
		&sql.NullInt16{Valid: false},
		&sql.NullInt32{Valid: false},
		&sql.Null[float32]{Valid: false},
		&sql.NullInt64{Valid: false},
		&sql.NullString{Valid: false},
		&sql.NullTime{Valid: false},
		&sql.Null[net.IP]{Valid: false},
		nil,
	}))
	enc.Close()

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 2)

	var row1 map[string]any
	require.NoError(t, gojson.Unmarshal([]byte(lines[0]), &row1))
	require.Equal(t, float64(tm.UnixNano()), row1["ptime"])
	require.Equal(t, float64(3.5), row1["pfloat64"])
	require.Equal(t, float64(1.25), row1["pfloat32"])
	require.Equal(t, "127.0.0.1", row1["ip"])
	require.Equal(t, true, row1["nbool"])
	require.Equal(t, float64(7), row1["nbyte"])
	require.Equal(t, float64(8.5), row1["nfloat64"])
	require.Equal(t, float64(16), row1["nint16"])
	require.Equal(t, float64(32), row1["nint32"])
	require.Equal(t, float64(2.5), row1["nfloat32"])
	require.Equal(t, float64(64), row1["nint64"])
	require.Equal(t, "text", row1["nstring"])
	require.Equal(t, "1700000000000000000", row1["ntime"])
	require.Equal(t, "127.0.0.1", row1["nip"])
	require.Equal(t, map[string]any{"nested": float64(1)}, row1["plain"])

	var row2 map[string]any
	require.NoError(t, gojson.Unmarshal([]byte(lines[1]), &row2))
	for _, key := range []string{"ptime", "pfloat64", "pfloat32", "ip", "nbool", "nbyte", "nfloat64", "nint16", "nint32", "nfloat32", "nint64", "nstring", "ntime", "nip", "plain"} {
		_, ok := row2[key]
		require.True(t, ok)
		require.Nil(t, row2[key])
	}
}

func TestNdjsonEncodeErrorPaths(t *testing.T) {
	t.Run("column mismatch", func(t *testing.T) {
		enc := ndjson.NewEncoder()
		enc.SetOutputStream(&bytes.Buffer{})
		enc.SetColumns("only")
		enc.SetColumnTypes("string")
		require.NoError(t, enc.Open())
		require.Error(t, enc.AddRow([]any{"a", "b"}))
	})

	t.Run("json encode error", func(t *testing.T) {
		enc := ndjson.NewEncoder()
		enc.SetOutputStream(&bytes.Buffer{})
		enc.SetColumns("bad")
		enc.SetColumnTypes("string")
		require.NoError(t, enc.Open())
		require.Error(t, enc.AddRow([]any{func() {}}))
	})
}
