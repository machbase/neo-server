package markdown_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/markdown"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func StringsEq(t *testing.T, expect string, actual string) bool {
	matched := false
	t.Helper()
	re := strings.Split(strings.TrimSpace(actual), "\n")
	ex := strings.Split(strings.TrimSpace(expect), "\n")
	if len(re) == len(ex) {
		for i := range re {
			if strings.TrimSpace(re[i]) != strings.TrimSpace(ex[i]) {
				t.Logf("Expect: %s", strings.TrimSpace(ex[i]))
				t.Logf("Actual: %s", strings.TrimSpace(re[i]))
				goto mismatched
			}
		}
		matched = true
	}
mismatched:
	return matched
}

func TestMarkdown(t *testing.T) {

	tests := []struct {
		opts   func(*markdown.Exporter)
		result string
	}{
		{
			opts:   func(md *markdown.Exporter) {},
			result: "output_md.txt",
		},
		{
			opts: func(md *markdown.Exporter) {
				md.SetHtml(true)
			},
			result: "output_md.html",
		},
		{
			opts: func(md *markdown.Exporter) {
				md.SetTimeformat("2006/01/02 15:04:05.999")
				md.SetTimeLocation(time.Local)
			},
			result: "output_timeformat.txt",
		},
		{
			opts: func(md *markdown.Exporter) {
				md.SetHtml(true)
				md.SetTimeformat("2006/01/02 15:04:05.999")
				md.SetTimeLocation(time.Local)
			},
			result: "output_timeformat.html",
		},
		{
			opts: func(md *markdown.Exporter) {
				md.SetTimeformat("2006/01/02 15:04:05.999")
				md.SetTimeLocation(time.Local)
				md.SetBriefCount(1)
			},
			result: "output_brief.txt",
		},
		{
			opts: func(md *markdown.Exporter) {
				md.SetHtml(true)
				md.SetTimeformat("2006/01/02 15:04:05.999")
				md.SetTimeLocation(time.Local)
				md.SetBriefCount(1)
			},
			result: "output_brief.html",
		},
	}

	for _, tt := range tests {
		buffer := &bytes.Buffer{}

		md := markdown.NewEncoder()
		md.SetOutputStream(stream.NewOutputStreamWriter(buffer))
		tt.opts(md)

		tick := time.Unix(0, 1692670838086467000)

		md.Open()
		md.AddRow([]any{tick.Add(0 * time.Second), 0.0, true})
		md.AddRow([]any{tick.Add(1 * time.Second), 1.0, false})
		md.AddRow([]any{tick.Add(2 * time.Second), 2.0, true})
		md.Close()

		if strings.HasSuffix(tt.result, ".html") {
			require.Equal(t, "application/xhtml+xml", md.ContentType())
		} else {
			require.Equal(t, "text/markdown", md.ContentType())
		}

		expect, err := os.ReadFile(filepath.Join("test", tt.result))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr := string(expect)

		if !StringsEq(t, expectStr, buffer.String()) {
			require.Equal(t, expectStr, buffer.String(), "md result unmatched\n%s", buffer.String())
		}
	}
}
