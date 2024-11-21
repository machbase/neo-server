package json_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/internal/json"
	"github.com/stretchr/testify/require"
)

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
