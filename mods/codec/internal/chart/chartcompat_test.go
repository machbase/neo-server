package chart_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/chart"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func TestCompat(t *testing.T) {
	for _, output := range []string{"json", "html"} {
		buffer := &bytes.Buffer{}
		fsmock := &VolatileFileWriterMock{}
		c := chart.NewRectChart("line")
		c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
		c.SetVolatileFileWriter(fsmock)
		c.SetChartJson(output == "json")
		c.SetChartId("WejMYXCGcYNL")
		c.SetTheme("westeros")
		c.SetGlobalOptions(`{"animation":true, "color":["#80FFA5", "#00DDFF", "#37A2FF"]}`)
		c.SetSize("400px", "300px")
		c.SetVisualMapColor(-2.0, 2.0,
			"#a50026", "#d73027", "#f46d43", "#fdae61", "#e0f3f8",
			"#abd9e9", "#74add1", "#4575b4", "#313695", "#313695",
			"#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#fdae61",
			"#f46d43", "#d73027", "#a50026")
		if output == "json" {
			require.Equal(t, "application/json", c.ContentType())
		} else {
			require.Equal(t, "text/html", c.ContentType())
		}

		tick := time.Unix(0, 1692670838086467000)

		c.Open()
		c.AddRow([]any{tick.Add(0 * time.Second), -2.0})
		c.AddRow([]any{tick.Add(1 * time.Second), -1.0})
		c.AddRow([]any{tick.Add(2 * time.Second), 0.0})
		c.AddRow([]any{tick.Add(3 * time.Second), 1.0})
		c.AddRow([]any{tick.Add(4 * time.Second), 2.0})
		c.Close()

		expect, err := os.ReadFile(filepath.Join("test", fmt.Sprintf("compat_line.%s", output)))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr := string(expect)
		if output == "json" {
			require.JSONEq(t, expectStr, buffer.String(), "json result unmatched\n%s", buffer.String())
		} else {
			if !StringsEq(t, expectStr, buffer.String()) {
				require.Equal(t, expectStr, buffer.String(), "html result unmatched\n%s", buffer.String())
			}
		}

		expect, err = os.ReadFile(filepath.Join("test", "compat_line.js"))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr = string(expect)
		if !StringsEq(t, expectStr, fsmock.buff.String()) {
			require.Equal(t, expectStr, fsmock.buff.String(), "js result unmatched\n%s", fsmock.buff.String())
		}
	}
}
