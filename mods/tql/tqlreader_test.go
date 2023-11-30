package tql

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
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
		t.Logf("Code:\n%s", code)
		t.Log("Expect:")
		for i, l := range expect {
			t.Logf("  expect[%d] %s", i, l.text)
		}
		t.Log("Actual:")
		for i, l := range lines {
			t.Logf("  actual[%d] %s", i, l.text)
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
	}

	for _, tt := range tests {
		runTestReadLine(t, tt.code, tt.expect)
	}
}
