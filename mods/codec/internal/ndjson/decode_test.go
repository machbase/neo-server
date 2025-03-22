package ndjson_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/internal/ndjson"
	"github.com/stretchr/testify/require"
)

// prev. version of decode.go
// BenchmarkNDJsonDecoder-24         123276             11733 ns/op            2400 B/op         61 allocs/op
// BenchmarkNDJsonDecoder-24         104146             11845 ns/op            2400 B/op         61 allocs/op
// BenchmarkNDJsonDecoder-24          99580             11854 ns/op            2400 B/op         61 allocs/op
// BenchmarkNDJsonDecoder-24         100100             11543 ns/op            2400 B/op         61 allocs/op
// BenchmarkNDJsonDecoder-24          96801             11790 ns/op            2400 B/op         61 allocs/op
// BenchmarkNDJsonDecoder-24          99336             11841 ns/op            2400 B/op         61 allocs/op
// BenchmarkNDJsonDecoder-24          99252             11837 ns/op            2400 B/op         61 allocs/op
// BenchmarkNDJsonDecoder-24          98857             12018 ns/op            2400 B/op         61 allocs/op
// BenchmarkNDJsonDecoder-24          98611             11908 ns/op            2400 B/op         61 allocs/op
// BenchmarkNDJsonDecoder-24          99546             11882 ns/op            2400 B/op         61 allocs/op

// new version of decode.go
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkNDJsonDecoder-24         129907             10724 ns/op            2240 B/op         49 allocs/op
// BenchmarkNDJsonDecoder-24         106989             11041 ns/op            2240 B/op         49 allocs/op
// BenchmarkNDJsonDecoder-24         106771             10961 ns/op            2240 B/op         49 allocs/op
// BenchmarkNDJsonDecoder-24         107634             10992 ns/op            2240 B/op         49 allocs/op
// BenchmarkNDJsonDecoder-24         105990             11000 ns/op            2240 B/op         49 allocs/op
// BenchmarkNDJsonDecoder-24         107485             10890 ns/op            2240 B/op         49 allocs/op
// BenchmarkNDJsonDecoder-24         106746             10993 ns/op            2240 B/op         49 allocs/op
// BenchmarkNDJsonDecoder-24         106914             10980 ns/op            2240 B/op         49 allocs/op
// BenchmarkNDJsonDecoder-24         106693             10994 ns/op            2240 B/op         49 allocs/op
// BenchmarkNDJsonDecoder-24         106910             10972 ns/op            2240 B/op         49 allocs/op

var benchRawInput = []byte{}

func TestMain(m *testing.M) {
	for i := 0; i < 30; i++ {
		benchRawInput = append(benchRawInput, []byte(`{"NAME":"my-car", "TIME":1670380343000000000, "VALUE":2.0002}`)...)
		benchRawInput = append(benchRawInput, '\n')
	}
	benchRawInput = append(benchRawInput, '\n')
	m.Run()
}

func BenchmarkNDJsonDecoder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		input := bytes.NewBuffer(benchRawInput)
		dec := ndjson.NewDecoder()
		dec.SetInputStream(input)
		dec.SetTimeformat("Default")
		dec.SetTimeLocation(time.UTC)
		dec.SetColumnTypes(api.COLUMN_TYPE_DATETIME, api.COLUMN_TYPE_VARCHAR, api.COLUMN_TYPE_DOUBLE)
		dec.SetColumns("TIME", "NAME", "VALUE")
		dec.Open()
		for i := 0; i < 2; i++ {
			_, _, err := dec.NextRow()
			require.Nil(b, err, "i=%d, err=%s", i, err)
		}
	}
}

func TestNDJsonDecoder(t *testing.T) {
	tests := []struct {
		input      string
		expects    [][]any
		timeformat string
		tz         *time.Location
	}{
		{
			input: `{"NAME":"my-car", "TIME":1670380342000000000, "VALUE":1.0001}
					{"NAME":"my-car", "TIME":1670380343000000000, "VALUE":2.0002}
					`,
			expects: [][]any{
				{time.Unix(0, 1670380342000000000), "my-car", 1.0001},
				{time.Unix(0, 1670380343000000000), "my-car", 2.0002},
			},
		},
		{
			input: `{"NAME":"my-car", "TIME":1670380342, "VALUE":1.0001}
					{"NAME":"my-car", "TIME":1670380343, "VALUE":2.0002}
					`,
			expects: [][]any{
				{time.Unix(0, 1670380342000000000), "my-car", 1.0001},
				{time.Unix(0, 1670380343000000000), "my-car", 2.0002},
			},
			timeformat: "s",
		},
		{
			input: `{"NAME":"my-car", "TIME":"2024-09-27 10:00:01.000", "VALUE":1.0001}
					{"NAME":"my-car", "TIME":"2024-09-27 10:00:02.000", "VALUE":2.0002}
					`,
			expects: [][]any{
				{time.Unix(0, 1727431201000000000).In(time.UTC), "my-car", 1.0001},
				{time.Unix(0, 1727431202000000000).In(time.UTC), "my-car", 2.0002},
			},
			timeformat: "Default",
			tz:         time.UTC,
		},
		{
			input: `{"NAME":"my-car", "TIME":"2024/09/27 10:00:01", "VALUE":1.0001}
					{"NAME":"my-car", "TIME":"2024/09/27 10:00:02", "VALUE":2.0002}
					`,
			expects: [][]any{
				{time.Unix(0, 1727431201000000000).In(time.UTC), "my-car", 1.0001},
				{time.Unix(0, 1727431202000000000).In(time.UTC), "my-car", 2.0002},
			},
			timeformat: "2006/01/02 15:04:05",
			tz:         time.UTC,
		},
	}

	for _, tt := range tests {
		dec := ndjson.NewDecoder()
		input := bytes.NewBuffer([]byte(tt.input))
		dec.SetInputStream(input)
		dec.SetTimeformat(tt.timeformat)
		dec.SetTimeLocation(tt.tz)
		dec.SetColumnTypes(api.COLUMN_TYPE_DATETIME, api.COLUMN_TYPE_VARCHAR, api.COLUMN_TYPE_DOUBLE)
		dec.SetColumns("TIME", "NAME", "VALUE")
		dec.Open()
		for _, expect := range tt.expects {
			values, columns, err := dec.NextRow()
			require.Nil(t, err)
			require.Equal(t, expect, values)
			require.Equal(t, []string{"TIME", "NAME", "VALUE"}, columns)
		}
	}
}
