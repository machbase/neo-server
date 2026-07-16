package markdown_test

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/internal/markdown"
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
				md.SetTimeLocation(time.UTC)
			},
			result: "output_timeformat.txt",
		},
		{
			opts: func(md *markdown.Exporter) {
				md.SetHtml(true)
				md.SetTimeformat("2006/01/02 15:04:05.999")
				md.SetTimeLocation(time.UTC)
			},
			result: "output_timeformat.html",
		},
		{
			opts: func(md *markdown.Exporter) {
				md.SetTimeformat("2006/01/02 15:04:05.999")
				md.SetTimeLocation(time.UTC)
				md.SetBriefCount(1)
			},
			result: "output_brief.txt",
		},
		{
			opts: func(md *markdown.Exporter) {
				md.SetHtml(true)
				md.SetTimeformat("2006/01/02 15:04:05.999")
				md.SetTimeLocation(time.UTC)
				md.SetBriefCount(1)
			},
			result: "output_brief.html",
		},
	}

	for _, tt := range tests {
		buffer := &bytes.Buffer{}

		md := markdown.NewEncoder()
		md.SetOutputStream(buffer)
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
			require.Equal(t, expectStr, buffer.String(), "md result %q unmatched\n%s", tt.result, buffer.String())
		}
	}
}

func TestMarkdownAddRowTypes(t *testing.T) {
	tick := time.Unix(0, 1692670838086467000).UTC()
	boolValue := true
	stringValue := "text"
	float64Value := 1.25
	float32Value := float32(2.5)
	intValue := 3
	int8Value := int8(4)
	int16Value := int16(5)
	int32Value := int32(6)
	int64Value := int64(7)
	ipValue := net.ParseIP("127.0.0.1")

	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{name: "nil", value: nil, expected: "NULL"},
		{name: "bool", value: true, expected: "true"},
		{name: "bool pointer", value: &boolValue, expected: "true"},
		{name: "string", value: "text", expected: "text"},
		{name: "string pointer", value: &stringValue, expected: "text"},
		{name: "time", value: tick, expected: "2023/08/22 02:20:38.086"},
		{name: "time pointer", value: &tick, expected: "2023/08/22 02:20:38.086"},
		{name: "float64", value: 1.25, expected: "1.250000"},
		{name: "float64 pointer", value: &float64Value, expected: "1.250000"},
		{name: "float32", value: float32(2.5), expected: "2.500000"},
		{name: "float32 pointer", value: &float32Value, expected: "2.500000"},
		{name: "int", value: 3, expected: "3"},
		{name: "int pointer", value: &intValue, expected: "3"},
		{name: "int8", value: int8(4), expected: "4"},
		{name: "int8 pointer", value: &int8Value, expected: "4"},
		{name: "int16", value: int16(5), expected: "5"},
		{name: "int16 pointer", value: &int16Value, expected: "5"},
		{name: "int32", value: int32(6), expected: "6"},
		{name: "int32 pointer", value: &int32Value, expected: "6"},
		{name: "int64", value: int64(7), expected: "7"},
		{name: "int64 pointer", value: &int64Value, expected: "7"},
		{name: "ip", value: ipValue, expected: "127.0.0.1"},
		{name: "ip pointer", value: &ipValue, expected: "127.0.0.1"},
		{name: "byte slice", value: []byte{97, 98, 99, 100, 101}, expected: "0x6162636465"},
		{name: "byte slice pointer", value: &[]byte{97, 98, 99, 100, 101}, expected: "0x6162636465"},
		{name: "fallback", value: struct{ Name string }{Name: "x"}, expected: "struct { Name string }{Name:\"x\"}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}

			md := markdown.NewEncoder()
			md.SetOutputStream(buffer)
			md.SetColumns("value")
			md.SetTimeformat("2006/01/02 15:04:05.999")
			md.SetTimeLocation(time.UTC)

			require.NoError(t, md.Open())
			require.NoError(t, md.AddRow([]any{tt.value}))
			md.Close()

			expected := "|value|\n|:-----|\n|" + tt.expected + "|\n"
			require.Equal(t, expected, buffer.String())
		})
	}
}

func TestBinaryFormat(t *testing.T) {
	tests := []struct {
		input        []byte
		binaryformat string
		expect       string
	}{
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "preview", "|1|preview|0x0102030405..|"},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "hex", "|1|hex|0x010203040506|"},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "bytes", "|1|bytes|[1 2 3 4 5 6]|"},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "base64", "|1|base64|AQIDBAUG|"},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "_unknown_", "|1|_unknown_|0x010203040506|"},
	}

	for _, tt := range tests {
		enc := markdown.NewEncoder()

		require.Equal(t, "text/markdown", enc.ContentType())

		w := &bytes.Buffer{}
		enc.SetOutputStream(w)
		enc.SetBinaryformat(tt.binaryformat)
		enc.SetRownum(true)
		enc.SetColumns("FORMAT", "BIN")
		err := enc.Open()
		require.Nil(t, err)
		enc.AddRow([]any{tt.binaryformat, tt.input})
		enc.Close()

		result := w.String()
		require.Contains(t, result, tt.expect)
	}
}

func TestMarkdownTemplatePathText(t *testing.T) {
	buffer := &bytes.Buffer{}

	md := markdown.NewEncoder()
	md.SetOutputStream(buffer)
	md.SetColumns("name", "value")
	md.SetTemplate(`{{- if .IsFirst -}}|name|value|{{"\n"}}|:-----|:-----|{{"\n"}}{{- end -}}|{{ .Value 0 }}|{{ .Value 1 }}|{{"\n"}}{{- if .IsLast -}}> *Total* {{ .Num }} *records*{{"\n"}}{{- end -}}`)

	require.NoError(t, md.Open())
	require.NoError(t, md.AddRow([]any{"alpha", 1}))
	require.NoError(t, md.AddRow([]any{"beta", 2}))
	md.Close()

	expected := "|name|value|\n|:-----|:-----|\n|alpha|1|\n|beta|2|\n> *Total* 2 *records*\n"
	require.Equal(t, expected, buffer.String())
}

func TestMarkdownTemplatePathHtml(t *testing.T) {
	buffer := &bytes.Buffer{}

	md := markdown.NewEncoder()
	md.SetOutputStream(buffer)
	md.SetHtml(true)
	md.SetTemplate(`# Title

|name|value|
|:-----|:-----|
|{{ .Value 0 }}|{{ .Value 1 }}|
`)

	require.NoError(t, md.Open())
	require.NoError(t, md.AddRow([]any{"alpha", 1}))
	md.Close()

	result := buffer.String()
	require.Equal(t, "application/xhtml+xml", md.ContentType())
	require.Contains(t, result, "<div>")
	require.Contains(t, result, "<h1>Title</h1>")
	require.Contains(t, result, "<table>")
	require.Contains(t, result, "<td align=\"left\">alpha</td>")
}
