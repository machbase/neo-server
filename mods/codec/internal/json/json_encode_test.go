package json_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/internal/json"
	"github.com/stretchr/testify/require"
)

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
		input      [][]any
		expect     string
		timeformat string
		tz         *time.Location
	}{
		{
			input: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002},
			},
			expect: `{"data":{"columns":["name","time","value"],"types":["string","datetime","double"],"rows":[["my-car",1670380342000000000,1.0001],["my-car",1670380343000000000,2.0002]]},"success":true,"reason":"success","elapse":`,
		},
		{
			input: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002},
			},
			expect:     `{"data":{"columns":["name","time","value"],"types":["string","datetime","double"],"rows":[["my-car",1670380342,1.0001],["my-car",1670380343,2.0002]]},"success":true,"reason":"success","elapse":`,
			timeformat: "s",
		},
		{
			input: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000).In(time.UTC), 2.0002},
			},
			expect:     `{"data":{"columns":["name","time","value"],"types":["string","datetime","double"],"rows":[["my-car","2022/12/07 02:32:22",1.0001],["my-car","2022/12/07 02:32:23",2.0002]]},"success":true,"reason":"success","elapse":`,
			timeformat: "2006/01/02 15:04:05",
			tz:         time.UTC,
		},
	}

	for _, tt := range tests {
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
			continue
		}
		substr := out.String()[:len(tt.expect)]
		require.Equal(t, tt.expect, substr)
	}
}
