package charset_test

import (
	"testing"

	"github.com/machbase/neo-server/mods/util/charset"
)

func TestEncoding(t *testing.T) {
	tests := []struct {
		charset string
	}{
		{"euc-kr"},
		{"euc-jp"},
	}
	for _, tt := range tests {
		enc, ok := charset.Encoding(tt.charset)
		if enc == nil || !ok {
			t.Logf("Charset %q failed", tt.charset)
			t.Fail()
		}
	}
}
