package internal

import (
	"math"
	"testing"
)

func runtimeSubtractFloat64(a, b float64) float64 {
	return a - b
}

func TestAppendPrecisionFloat64(t *testing.T) {
	tests := []struct {
		name         string
		dst          []byte
		value        float64
		precision    int
		quoteSpecial bool
		expect       string
	}{
		{
			name:      "default precision trims trailing zeros",
			value:     12.3400,
			precision: -1,
			expect:    "12.34",
		},
		{
			name:      "default precision rounds runtime expression",
			value:     runtimeSubtractFloat64(20.55, 22.2),
			precision: -1,
			expect:    "-1.65",
		},
		{
			name:      "explicit precision keeps fixed digits",
			value:     3.1,
			precision: 3,
			expect:    "3.100",
		},
		{
			name:      "explicit precision pads integer fraction zeros",
			value:     10,
			precision: 2,
			expect:    "10.00",
		},
		{
			name:      "negative zero normalized",
			value:     math.Copysign(0, -1),
			precision: -1,
			expect:    "0",
		},
		{
			name:         "quoted nan",
			value:        math.NaN(),
			precision:    -1,
			quoteSpecial: true,
			expect:       `"NaN"`,
		},
		{
			name:         "plain negative infinity",
			value:        math.Inf(-1),
			precision:    -1,
			quoteSpecial: false,
			expect:       "-Inf",
		},
		{
			name:         "quoted positive infinity with prefix",
			dst:          []byte("prefix:"),
			value:        math.Inf(1),
			precision:    -1,
			quoteSpecial: true,
			expect:       `prefix:"+Inf"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AppendPrecisionFloat64(tt.dst, tt.value, tt.precision, tt.quoteSpecial)
			if string(got) != tt.expect {
				t.Fatalf("AppendPrecisionFloat64()=%q, want %q", string(got), tt.expect)
			}
		})
	}
}

func TestFormatPrecisionFloat64(t *testing.T) {
	if got := FormatPrecisionFloat64(10.0, -1, false); got != "10" {
		t.Fatalf("FormatPrecisionFloat64()=%q, want %q", got, "10")
	}

	if got := FormatPrecisionFloat64(10.0, 4, false); got != "10.0000" {
		t.Fatalf("FormatPrecisionFloat64()=%q, want %q", got, "10.0000")
	}

	if got := FormatPrecisionFloat64(math.NaN(), -1, true); got != `"NaN"` {
		t.Fatalf("FormatPrecisionFloat64()=%q, want %q", got, `"NaN"`)
	}
}
