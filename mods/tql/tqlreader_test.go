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
	task.SetLogLevel(INFO)
	task.SetConsoleLogLevel(FATAL)

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
		if e.text != l.text || e.isComment != l.isComment || e.isPragma != l.isPragma {
			t.Logf("Expect[%d] %v", i, e)
			t.Logf("Actual[%d] %v", i, *l)
			t.Fail()
		}
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
				{text: "FAKE('안녕')"},
				{text: "CSV()"},
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
				{text: "comment1", isComment: true},
				{text: "FAKE('hello')"},
				{text: " MAPVALUE(2,\n  value(1) * 10,\n  true\n )", line: 3},
				{text: " comment3 // and", isComment: true},
				{text: "CSV()"},
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
				{text: "FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100)))"},
				{text: "MAPVALUE(2,\n	sin(pow(value(0), 2) + pow(value(1), 2))\n	/\n	(pow(value(0), 2) + pow(value(1), 2))\n)"},
				{text: "CHART_LINE3D()"},
			},
		},
		{
			`FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100))) // comment
			|MAPVALUE()
			`,
			[]Line{
				{text: "FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100)))"},
				{text: "MAPVALUE()"},
			},
		},
		{
			`FAKE(meshgrid(linspace(-4,4,100 // comment
			|),
			|linspace(-4,4, 100)))
			|MAPVALUE()
			`,
			[]Line{
				{text: "FAKE(meshgrid(linspace(-4,4,100 \n),\nlinspace(-4,4, 100)))"},
				{text: "MAPVALUE()"},
			},
		},
		{
			`FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))
			|//+ stateful
			|WHEN( cond, doHttp())
			|MAPVALUE()
			`,
			[]Line{
				{text: "FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))"},
				{text: " stateful", isComment: true, isPragma: true},
				{text: "WHEN( cond, doHttp())"},
				{text: "MAPVALUE()"},
			},
		},
	}

	for _, tt := range tests {
		runTestReadLine(t, tt.code, tt.expect)
	}
}
