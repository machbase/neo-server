package ndjson_test

import (
	"bytes"
	"math"
	"strings"
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
			b, err := ndjson.PrecisionFloat64(tt.value).MarshalJSON()
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
