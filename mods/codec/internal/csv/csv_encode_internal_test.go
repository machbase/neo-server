package csv

import "testing"

func TestEncodeBinary(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		mode   string
		expect string
	}{
		{name: "empty", input: []byte{}, mode: "hex", expect: ""},
		{name: "base64", input: []byte{0x01, 0x02, 0x03}, mode: "base64", expect: "AQID"},
		{name: "hex fallback", input: []byte{0x0a, 0x0b}, mode: "unknown", expect: "0x0a0b"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := encodeBinary(tc.input, tc.mode); got != tc.expect {
				t.Fatalf("encodeBinary(%v, %q) = %q, want %q", tc.input, tc.mode, got, tc.expect)
			}
		})
	}
}
