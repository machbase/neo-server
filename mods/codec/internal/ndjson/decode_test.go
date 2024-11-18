package ndjson_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/internal/ndjson"
	"github.com/machbase/neo-server/v8/mods/stream"
	"github.com/stretchr/testify/require"
)

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
		input := &stream.ReaderInputStream{Reader: (bytes.NewBuffer([]byte(tt.input)))}
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
