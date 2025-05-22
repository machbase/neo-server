package tql

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func runTestReadLine(t *testing.T, code string, expect []Line) {
	t.Helper()

	timeCtx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	output := &bytes.Buffer{}
	logBuf := &bytes.Buffer{}

	task := NewTaskContext(timeCtx)
	task.SetOutputWriter(output)
	task.SetLogWriter(logBuf)
	task.SetConsoleLogLevel(ERROR)

	buf := []string{}
	src := strings.Split(code, "\n")
	for _, l := range src {
		l = strings.TrimPrefix(strings.TrimSpace(l), "|")
		buf = append(buf, l)
	}
	code = strings.Join(buf, "\n")
	reader := bytes.NewBufferString(code)
	lines, err := readLines(task, reader)
	if err != nil {
		t.Fail()
		return
	}
	if len(expect) != len(lines) {
		t.Logf("Expect %d lines, got %d", len(expect), len(lines))
		t.Log("Expect:")
		for i, l := range expect {
			t.Logf("  expect[%d] %s", i, l.text)
		}
		t.Log("Actual:")
		for i, l := range lines {
			t.Logf("  actual[%d] %s", i, l.text)
		}
		t.Fail()
		return
	}
	for i := 0; i < len(expect); i++ {
		e := expect[i]
		l := lines[i]
		require.Equal(t, e.text, l.text)
		if e.text != l.text || e.isComment != l.isComment || e.isPragma != l.isPragma || e.line != l.line {
			t.Logf("Expect[%d]:%d %v", i, e.line, e)
			t.Logf("Actual[%d]:%d %v", i, l.line, *l)
			t.Fail()
		}
	}
	if len(lines) > len(expect) {
		for i := len(expect); i < len(lines); i++ {
			l := lines[i]
			t.Logf("Actual[%d]:%d %v", i, l.line, *l)
		}
		t.Fail()
	}
}

func TestReadLine(t *testing.T) {
	tests := []struct {
		code   string
		expect []Line
	}{
		{
			`FAKE('안녕') // comment
			|CSV()
			`,
			[]Line{
				{text: "FAKE('안녕')", line: 1},
				{text: "CSV()", line: 2},
			},
		},
		{
			`//comment1
			|FAKE('hello') // comment2
			| MAPVALUE(2,
			|  value(1) * 10, // inline comment
			|  true
			| ) // end of MAPVALUE
			|// comment3 // and
			|CSV()
			`,
			[]Line{
				{text: "comment1", isComment: true, line: 1},
				{text: "\nFAKE('hello')", line: 2},
				{text: " MAPVALUE(2,\n  value(1) * 10,\n  true\n )", line: 3},
				{text: " comment3 // and", isComment: true, line: 7},
				{text: "\nCSV()", line: 8},
			},
		},
		{
			`FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100)))
			|MAPVALUE(2,
			|	sin(pow(value(0), 2) + pow(value(1), 2))
			|	/
			|	(pow(value(0), 2) + pow(value(1), 2))
			|)
			|CHART_LINE3D()
			`,
			[]Line{
				{text: "FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100)))", line: 1},
				{text: "MAPVALUE(2,\n	sin(pow(value(0), 2) + pow(value(1), 2))\n	/\n	(pow(value(0), 2) + pow(value(1), 2))\n)", line: 2},
				{text: "CHART_LINE3D()", line: 7},
			},
		},
		{
			`FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100))) // comment
			|MAPVALUE()
			`,
			[]Line{
				{text: "FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100)))", line: 1},
				{text: "MAPVALUE()", line: 2},
			},
		},
		{
			`FAKE(meshgrid(linspace(-4,4,100 // comment
			|),
			|linspace(-4,4, 100)))
			|MAPVALUE()
			`,
			[]Line{
				{text: "FAKE(meshgrid(linspace(-4,4,100 \n),\nlinspace(-4,4, 100)))", line: 1},
				{text: "MAPVALUE()", line: 4},
			},
		},
		{
			`FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))
			|//+ stateful
			|WHEN( cond, doHttp())
			|MAPVALUE()
			`,
			[]Line{
				{text: "FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))", line: 1},
				{text: " stateful", isComment: true, isPragma: true, line: 2},
				{text: "WHEN( cond, doHttp())", line: 3},
				{text: "MAPVALUE()", line: 4},
			},
		},
		{
			`FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))
			|#pragma stateful
			|WHEN( cond, doHttp())
			|MAPVALUE()
			`,
			[]Line{
				{text: "FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))", line: 1},
				{text: " stateful", isComment: true, isPragma: true, line: 2},
				{text: "WHEN( cond, doHttp())", line: 3},
				{text: "MAPVALUE()", line: 4},
			},
		},
		{
			`SCRIPT({
			|
			|  // comment-first
			|  line2;
			|  // comment-second
			|  line4;
			|
			|}) // comment
			|CSV()
			`,
			[]Line{
				{text: " comment-first", isComment: true, line: 3},
				{text: " comment-second", isComment: true, line: 5},
				{text: "SCRIPT({\n\n  line2;\n\n  line4;\n})", line: 1},
				{text: "CSV()", line: 9},
			},
		},
	}

	for _, tt := range tests {
		runTestReadLine(t, tt.code, tt.expect)
	}
}
