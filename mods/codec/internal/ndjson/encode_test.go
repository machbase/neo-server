package ndjson_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/internal/ndjson"
	"github.com/stretchr/testify/require"
)

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
