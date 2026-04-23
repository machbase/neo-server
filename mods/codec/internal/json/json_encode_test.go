package json_test

import (
	"bytes"
	"database/sql"
	gojson "encoding/json"
	"math"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/internal/json"
	"github.com/stretchr/testify/require"
)

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

// previous version of json_encode.go
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkJsonEncoder-24            58428             23664 ns/op            4386 B/op        112 allocs/op
// BenchmarkJsonEncoder-24            49105             23536 ns/op            4386 B/op        112 allocs/op
// BenchmarkJsonEncoder-24            49855             23845 ns/op            4386 B/op        112 allocs/op
// BenchmarkJsonEncoder-24            50280             23721 ns/op            4386 B/op        112 allocs/op
// BenchmarkJsonEncoder-24            49945             23315 ns/op            4386 B/op        112 allocs/op
// BenchmarkJsonEncoder-24            49743             23441 ns/op            4386 B/op        112 allocs/op
// BenchmarkJsonEncoder-24            49254             24084 ns/op            4386 B/op        112 allocs/op
// BenchmarkJsonEncoder-24            51152             22968 ns/op            4386 B/op        112 allocs/op
// BenchmarkJsonEncoder-24            49750             23917 ns/op            4386 B/op        112 allocs/op
// BenchmarkJsonEncoder-24            49808             23883 ns/op            4386 B/op        112 allocs/op

// new version of json_encode.go
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkJsonEncoder-24            54735             22211 ns/op            3729 B/op         96 allocs/op
// BenchmarkJsonEncoder-24            53389             22149 ns/op            3728 B/op         96 allocs/op
// BenchmarkJsonEncoder-24            52502             22371 ns/op            3728 B/op         96 allocs/op
// BenchmarkJsonEncoder-24            52509             22616 ns/op            3729 B/op         96 allocs/op
// BenchmarkJsonEncoder-24            52633             22663 ns/op            3728 B/op         96 allocs/op
// BenchmarkJsonEncoder-24            52543             22691 ns/op            3728 B/op         96 allocs/op
// BenchmarkJsonEncoder-24            52564             22580 ns/op            3729 B/op         96 allocs/op
// BenchmarkJsonEncoder-24            52594             22584 ns/op            3728 B/op         96 allocs/op
// BenchmarkJsonEncoder-24            52461             22694 ns/op            3729 B/op         96 allocs/op
// BenchmarkJsonEncoder-24            51908             22689 ns/op            3728 B/op         96 allocs/op

// 2026-04-23 - refactor json_encode.go to reduce allocations and improve performance
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkJsonEncoder-24            65336             18398 ns/op            3375 B/op         85 allocs/op
// BenchmarkJsonEncoder-24            65083             18203 ns/op            3375 B/op         85 allocs/op
// BenchmarkJsonEncoder-24            65349             17986 ns/op            3375 B/op         85 allocs/op
// BenchmarkJsonEncoder-24            65488             18143 ns/op            3375 B/op         85 allocs/op
// BenchmarkJsonEncoder-24            64909             18116 ns/op            3375 B/op         85 allocs/op
// BenchmarkJsonEncoder-24            69688             18246 ns/op            3375 B/op         85 allocs/op
// BenchmarkJsonEncoder-24            65103             18232 ns/op            3375 B/op         85 allocs/op
// BenchmarkJsonEncoder-24            64610             18266 ns/op            3375 B/op         85 allocs/op
// BenchmarkJsonEncoder-24            65314             18069 ns/op            3375 B/op         85 allocs/op
// BenchmarkJsonEncoder-24            65167             18272 ns/op            3375 B/op         85 allocs/op

func BenchmarkJsonEncoder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		out := &bytes.Buffer{}

		enc := json.NewEncoder()
		enc.SetOutputStream(out)
		enc.SetTimeformat("Default")
		enc.SetTimeLocation(time.UTC)
		enc.SetColumnTypes("string", "datetime", "double")
		enc.SetColumns("name", "time", "value")
		enc.Open()
		for i := 0; i < 10; i++ {
			enc.AddRow([]interface{}{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002})
		}
		enc.Close()
	}
}

func TestJsonEncode(t *testing.T) {
	tests := []struct {
		name       string
		input      [][]any
		expect     string
		timeformat string
		tz         *time.Location
	}{
		{
			name: "utc-default",
			input: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002},
			},
			expect: `{"data":{"columns":["name","time","value"],"types":["string","datetime","double"],"rows":[["my-car",1670380342000000000,1.0001],["my-car",1670380343000000000,2.0002]]},"success":true,"reason":"success","elapse":`,
		},
		{
			name: "utc-timeformat-s",
			input: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002},
			},
			expect:     `{"data":{"columns":["name","time","value"],"types":["string","datetime","double"],"rows":[["my-car",1670380342,1.0001],["my-car",1670380343,2.0002]]},"success":true,"reason":"success","elapse":`,
			timeformat: "s",
		},
		{
			name: "utc-timeformat",
			input: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002},
			},
			expect:     `{"data":{"columns":["name","time","value"],"types":["string","datetime","double"],"rows":[["my-car","2022/12/07 02:32:22",1.0001],["my-car","2022/12/07 02:32:23",2.0002]]},"success":true,"reason":"success","elapse":`,
			timeformat: "2006/01/02 15:04:05",
			tz:         time.UTC,
		},
		{
			name:   "empty-result",
			input:  [][]any{},
			expect: `{"data":{"columns":["name","time","value"],"types":["string","datetime","double"],"rows":[]},"success":true,"reason":"success","elapse":`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			enc := json.NewEncoder()
			enc.SetOutputStream(out)
			enc.SetTimeformat(tt.timeformat)
			enc.SetTimeLocation(tt.tz)
			enc.SetColumnTypes("string", "datetime", "double")
			enc.SetColumns("name", "time", "value")
			enc.Open()
			for _, row := range tt.input {
				enc.AddRow(row)
			}
			enc.Close()
			if len(out.String()) < len(tt.expect) {
				t.Errorf("\noutput: %s\nexpect: %s\n", out.String(), tt.expect)
				t.Fail()
			}
			substr := out.String()[:len(tt.expect)]
			require.Equal(t, tt.expect, substr)
		})
	}
}

func runtimeSubtractFloat64(a, b float64) float64 {
	return a - b
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
		{
			name:   "regular-float-runtime-expression",
			value:  runtimeSubtractFloat64(20.55, 22.2),
			expect: "-1.65",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.AppendJSONValue(nil, tt.value, -1)
			require.NoError(t, err)
			require.Equal(t, tt.expect, string(b))
		})
	}
}

func TestJsonEncodeFlushAndClose(t *testing.T) {
	out := &flushCloseBuffer{}
	enc := json.NewEncoder()
	enc.SetOutputStream(out)
	enc.SetColumnTypes("string")
	enc.SetColumns("name")
	require.NoError(t, enc.Open())

	enc.Flush(false)
	require.Equal(t, 1, out.flushCount)

	enc.Close()
	require.Equal(t, 1, out.closeCount)
}

func TestJsonEncodeRowsArray(t *testing.T) {
	out := &bytes.Buffer{}
	enc := json.NewEncoder()
	enc.SetOutputStream(out)
	enc.SetColumnTypes("string", "long", "double")
	enc.SetColumns("name", "seq", "value")
	enc.SetRowsArray(true)
	require.NoError(t, enc.Open())
	require.NoError(t, enc.AddRow([]any{"car-1", int64(7), float64(12.3400)}))
	enc.Close()

	var payload map[string]any
	require.NoError(t, gojson.Unmarshal(out.Bytes(), &payload))

	data, ok := payload["data"].(map[string]any)
	require.True(t, ok)
	rows, ok := data["rows"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 1)

	row, ok := rows[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "car-1", row["name"])
	require.Equal(t, float64(7), row["seq"])
	require.Equal(t, float64(12.34), row["value"])
}

func TestJsonEncodeRowsFlatten(t *testing.T) {
	out := &bytes.Buffer{}
	enc := json.NewEncoder()
	enc.SetOutputStream(out)
	enc.SetColumnTypes("string", "long", "double")
	enc.SetColumns("name", "seq", "value")
	enc.SetRowsFlatten(true)
	require.NoError(t, enc.Open())
	require.NoError(t, enc.AddRow([]any{"car-1", int64(1), float64(1.2500)}))
	require.NoError(t, enc.AddRow([]any{"car-2", int64(2), float64(2.5000)}))
	enc.Close()

	var payload map[string]any
	require.NoError(t, gojson.Unmarshal(out.Bytes(), &payload))

	data, ok := payload["data"].(map[string]any)
	require.True(t, ok)
	rows, ok := data["rows"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 6)
	require.Equal(t, "car-1", rows[0])
	require.Equal(t, float64(1), rows[1])
	require.Equal(t, float64(1.25), rows[2])
	require.Equal(t, "car-2", rows[3])
	require.Equal(t, float64(2), rows[4])
	require.Equal(t, float64(2.5), rows[5])
}

func TestJsonEncodeTranspose(t *testing.T) {
	out := &bytes.Buffer{}
	enc := json.NewEncoder()
	enc.SetOutputStream(out)
	enc.SetColumnTypes("string", "double")
	enc.SetColumns("name", "value")
	enc.SetTranspose(true)
	require.NoError(t, enc.Open())
	require.NoError(t, enc.AddRow([]any{"car-1", float64(1.0)}))
	require.NoError(t, enc.AddRow([]any{"car-2", float64(2.5000)}))
	enc.Close()

	var payload map[string]any
	require.NoError(t, gojson.Unmarshal(out.Bytes(), &payload))

	data, ok := payload["data"].(map[string]any)
	require.True(t, ok)
	cols, ok := data["cols"].([]any)
	require.True(t, ok)
	require.Len(t, cols, 2)

	col0, ok := cols[0].([]any)
	require.True(t, ok)
	require.Equal(t, []any{"car-1", "car-2"}, col0)

	col1, ok := cols[1].([]any)
	require.True(t, ok)
	require.Equal(t, []any{float64(1), float64(2.5)}, col1)
}

func TestAppendJSONValueCoversPrimitiveTypes(t *testing.T) {
	tests := []struct {
		name      string
		value     any
		precision int
		expect    string
	}{
		{name: "nil", value: nil, precision: -1, expect: "null"},
		{name: "string", value: "hello", precision: -1, expect: `"hello"`},
		{name: "bool", value: true, precision: -1, expect: "true"},
		{name: "float32", value: float32(1.25), precision: 2, expect: "1.25"},
		{name: "int", value: int(1), precision: -1, expect: "1"},
		{name: "int8", value: int8(2), precision: -1, expect: "2"},
		{name: "int16", value: int16(3), precision: -1, expect: "3"},
		{name: "int32", value: int32(4), precision: -1, expect: "4"},
		{name: "int64", value: int64(5), precision: -1, expect: "5"},
		{name: "uint", value: uint(6), precision: -1, expect: "6"},
		{name: "uint8", value: uint8(7), precision: -1, expect: "7"},
		{name: "uint16", value: uint16(8), precision: -1, expect: "8"},
		{name: "uint32", value: uint32(9), precision: -1, expect: "9"},
		{name: "uint64", value: uint64(10), precision: -1, expect: "10"},
		{name: "number", value: gojson.Number("11.5"), precision: -1, expect: "11.5"},
		{name: "map", value: map[string]any{"k": "v"}, precision: -1, expect: `{"k":"v"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.AppendJSONValue(nil, tt.value, tt.precision)
			require.NoError(t, err)
			require.Equal(t, tt.expect, string(got))
		})
	}

	_, err := json.AppendJSONValue(nil, func() {}, -1)
	require.Error(t, err)
}

func TestJsonEncodeSettersAndTypedRows(t *testing.T) {
	out := &flushCloseBuffer{}
	enc := json.NewEncoder()
	enc.SetOutputStream(out)
	enc.SetHeader(true)
	enc.SetHeading(false)
	enc.SetPrecision(2)
	enc.SetRowsArray(true)
	enc.SetRownum(true)
	enc.SetColumnTypes("datetime", "double", "double", "string", "bool", "byte", "double", "int16", "int32", "double", "int64", "string", "datetime", "string", "string")
	enc.SetColumns("ptime", "pfloat64", "pfloat32", "ip", "nbool", "nbyte", "nfloat64", "nint16", "nint32", "nfloat32", "nint64", "nstring", "ntime", "nip", "plain")
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
	enc.Flush(false)
	enc.Close()

	require.Equal(t, 1, out.flushCount)
	require.Equal(t, 1, out.closeCount)

	var payload map[string]any
	require.NoError(t, gojson.Unmarshal(out.Bytes(), &payload))
	row := payload["data"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	require.Equal(t, float64(1), row["ROWNUM"])
	require.Equal(t, float64(tm.UnixNano()), row["ptime"])
	require.Equal(t, float64(3.5), row["pfloat64"])
	require.Equal(t, float64(1.25), row["pfloat32"])
	require.Equal(t, "127.0.0.1", row["ip"])
	require.Equal(t, true, row["nbool"])
	require.Equal(t, float64(7), row["nbyte"])
	require.Equal(t, float64(8.5), row["nfloat64"])
	require.Equal(t, float64(16), row["nint16"])
	require.Equal(t, float64(32), row["nint32"])
	require.Equal(t, float64(2.5), row["nfloat32"])
	require.Equal(t, float64(64), row["nint64"])
	require.Equal(t, "text", row["nstring"])
	require.Equal(t, tm.Format(time.RFC3339), time.Unix(0, int64(row["ptime"].(float64))).UTC().Format(time.RFC3339))
	require.Equal(t, "127.0.0.1", row["nip"])
	require.Equal(t, map[string]any{"nested": float64(1)}, row["plain"])
}

func TestJsonEncodeErrorPaths(t *testing.T) {
	t.Run("rows array marshal error", func(t *testing.T) {
		enc := json.NewEncoder()
		enc.SetOutputStream(&bytes.Buffer{})
		enc.SetRowsArray(true)
		enc.SetColumns("bad")
		enc.SetColumnTypes("string")
		require.NoError(t, enc.Open())
		require.Error(t, enc.AddRow([]any{func() {}}))
	})

	t.Run("rows flatten marshal error", func(t *testing.T) {
		enc := json.NewEncoder()
		enc.SetOutputStream(&bytes.Buffer{})
		enc.SetRowsFlatten(true)
		enc.SetColumns("bad")
		enc.SetColumnTypes("string")
		require.NoError(t, enc.Open())
		require.Error(t, enc.AddRow([]any{func() {}}))
	})
}
