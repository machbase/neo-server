package ymds_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/util/ymds"
)

func TestParser(t *testing.T) {
	tickUTC, _ := time.Parse("2006/01/02 15:04:05.999999999", "2001/10/20 12:13:14.123456789")

	mtc := "KST"
	tzLocal, _ := time.LoadLocation(mtc)
	tickLocal, err := time.Parse("2006/01/02 15:04:05.999999999 MST", fmt.Sprintf("2001/10/20 12:13:14.123456789 %s", mtc))
	if err != nil {
		t.Fatal(err.Error())
	}

	tests := []struct {
		layout string
		input  string
		tz     *time.Location
		expect time.Time
		err    string
	}{
		{"YYYY/MM/DD HH24:MI:SS.mmmuuunnn", "2001/10/20 12:13:14.123456789", time.UTC, tickUTC, ""},
		{"YYYY/MM/DD HH24:MI:SS.mmmuuunnn", "2001/10/20 12:13:14.123456789", tzLocal, tickLocal, ""},
		{"YYYY/MM/DD HH24:MI:SS mmm.uuu.nnn", "2001/10/20 12:13:14 123.456.789", time.UTC, tickUTC, ""},
		{"YYYY/MM/DD HH24:MI:SS mmm.uuu.nnn", "2001/10/20 12:13:14 123.456.789", tzLocal, tickLocal, ""},
	}
	for _, tt := range tests {
		p := ymds.NewParser(tt.layout) //.WithDebug()
		if tt.tz != nil {
			p = p.WithLocation(tt.tz)
		}
		result, err := p.Parse(tt.input)
		if tt.err == "" {
			if tt.expect != result {
				t.Logf("expect %q, got=%q in %q diff:%d", tt.expect, result, tt.input, tt.expect.Sub(result))
				t.Fail()
			}
		} else {
			if err == nil || err.Error() != tt.err {
				t.Logf("expect error, got=%v in %q", err, tt.input)
			}
		}
	}
}

func BenchmarkYmdsformat(b *testing.B) {
	parser := ymds.NewParser("YYYY/MM/DD HH24:MI:SS mmm.uuu.nnn")
	for i := 0; i < b.N; i++ {
		data := fmt.Sprintf("2001/10/20 12:13:%02d 123.456.789", i%60)
		_, err := parser.Parse(data)
		if err != nil {
			b.Log("ERR", err.Error())
			b.Fail()
		}
	}
}

func BenchmarkGoTimeformat(b *testing.B) {
	format := `2006/01/02 15:04:05.999999999 MST`
	for i := 0; i < b.N; i++ {
		data := fmt.Sprintf("2001/10/20 12:13:%02d.123456789 KST", i%60)
		_, err := time.Parse(format, data)
		if err != nil {
			b.Log("ERR", err.Error())
			b.Fail()
		}
	}
}
