package json_test

import (
	"bytes"
	gojson "encoding/json"
	"math"
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
			t.Log("output: ", substr)
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
			b, err := json.PrecisionFloat64(tt.value).MarshalJSON()
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
