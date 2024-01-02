package markdown_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/codec/internal/markdown"
	"github.com/machbase/neo-server/mods/stream"
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
	var expectStr = ""

	buffer := &bytes.Buffer{}

	md := markdown.NewEncoder()
	md.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	tick := time.Unix(0, 1692670838086467000)

	md.Open()
	md.AddRow([]any{tick.Add(0 * time.Second), 0.0, true})
	md.AddRow([]any{tick.Add(1 * time.Second), 1.0, false})
	md.AddRow([]any{tick.Add(2 * time.Second), 2.0, true})
	md.Close()

	require.Equal(t, "text/markdown", md.ContentType())

	expectStr = `|column0|column1|column2|
	|:-----|:-----|:-----|
	|1692670838086467000|0.000000|true|
	|1692670839086467000|1.000000|false|
	|1692670840086467000|2.000000|true|
	`
	if !StringsEq(t, expectStr, buffer.String()) {
		require.Equal(t, expectStr, buffer.String(), "md result unmatched\n%s", buffer.String())
	}
}
