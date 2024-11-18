package json_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/internal/json"

	"github.com/machbase/neo-server/v8/mods/stream"
	"github.com/stretchr/testify/require"
)

func TestJsonDecoder(t *testing.T) {
	tests := []struct {
		input      string
		expects    [][]any
		timeformat string
		tz         *time.Location
	}{
		{
			input: `{	"data": { "columns":["name", "time", "value"],
						"rows": [
							[ "my-car", 1670380342000000000, 1.0001 ],
							[ "my-car", 1670380343000000000, 2.0002 ]
						]} }`,
			expects: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000), 2.0002},
			},
		},
		{
			input: `{	"data": { "columns":["name", "time", "value"],
						"rows": [
							[ "my-car", 1670380342, 1.0001 ],
							[ "my-car", 1670380343, 2.0002 ]
						]} }`,
			expects: [][]any{
				{"my-car", time.Unix(0, 1670380342000000000), 1.0001},
				{"my-car", time.Unix(0, 1670380343000000000), 2.0002},
			},
			timeformat: "s",
		},
		{
			input: `{	"data": { "columns":["name", "time", "value"],
						"rows": [
							[ "my-car", "2024-09-27 10:00:01.000", 1.0001 ],
							[ "my-car", "2024-09-27 10:00:02.000", 2.0002 ]
						]} }`,
			expects: [][]any{
				{"my-car", time.Unix(0, 1727431201000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1727431202000000000).In(time.UTC), 2.0002},
			},
			timeformat: "Default",
			tz:         time.UTC,
		},
		{
			input: `{	"data": { "columns":["name", "time", "value"],
						"rows": [
							[ "my-car", "2024/09/27 10:00:01", 1.0001 ],
							[ "my-car", "2024/09/27 10:00:02", 2.0002 ]
						]} }`,
			expects: [][]any{
				{"my-car", time.Unix(0, 1727431201000000000).In(time.UTC), 1.0001},
				{"my-car", time.Unix(0, 1727431202000000000).In(time.UTC), 2.0002},
			},
			timeformat: "2006/01/02 15:04:05",
			tz:         time.UTC,
		},
	}

	for _, tt := range tests {
		dec := json.NewDecoder()
		input := &stream.ReaderInputStream{Reader: (bytes.NewBuffer([]byte(tt.input)))}
		dec.SetInputStream(input)
		dec.SetTimeformat(tt.timeformat)
		dec.SetTimeLocation(tt.tz)
		dec.SetColumnTypes(api.DataTypeString, api.DataTypeDatetime, api.DataTypeFloat64)
		dec.Open()
		for _, expect := range tt.expects {
			fields, _, err := dec.NextRow()
			require.Nil(t, err)
			require.Equal(t, expect, fields)
		}
	}
}
