package system

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
	"github.com/machbase/neo-server/v8/mods/logging"
)

type TestCase struct {
	name       string
	script     string
	input      []string
	output     []string
	expectFunc func(t *testing.T, result string)
	err        string
	vars       map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()

	w := &bytes.Buffer{}
	logging.Configure(&logging.Config{
		Console:                     true,
		Filename:                    "-",
		Append:                      false,
		DefaultPrefixWidth:          5,
		DefaultEnableSourceLocation: false,
		DefaultLevel:                "TRACE",
		Writer:                      w,
	})

	conf := engine.Config{
		Name:   tc.name,
		Code:   tc.script,
		FSTabs: []engine.FSTab{root.RootFSTab()},
		Env:    tc.vars,
		Reader: &bytes.Buffer{},
		Writer: w,
	}
	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("Failed to create JSRuntime: %v", err)
	}
	jr.RegisterNativeModule("@jsh/system", Module)

	if len(tc.input) > 0 {
		conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.input, ""))
	}
	if err := jr.Run(); err != nil {
		if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
			t.Fatalf("Unexpected error: %v", err)
		}
		return
	}

	gotOutput := conf.Writer.(*bytes.Buffer).String()
	if tc.expectFunc != nil {
		tc.expectFunc(t, gotOutput)
		return
	}
	lines := strings.Split(gotOutput, "\n")
	if len(lines) != len(tc.output)+1 { // +1 for trailing newline
		t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
	}
	for i, expectedLine := range tc.output {
		if lines[i] != expectedLine {
			t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
		}
	}
}

func TestLog(t *testing.T) {
	tests := []TestCase{
		{
			name: "log",
			script: `
				const system = require("@jsh/system");
				const log = new system.Log("jsh-log");
				log.info("test info");
				log.error("test error");
				log.warn("test warn");
				log.debug("test debug");
				log.trace("test trace");
				`,
			expectFunc: func(t *testing.T, result string) {
				lines := strings.Split(result, "\n")
				output := []string{
					`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{3})\sINFO  jsh-log test info`,
					`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{3})\sERROR jsh-log test error`,
					`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{3})\sWARN  jsh-log test warn`,
					`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{3})\sDEBUG jsh-log test debug`,
					`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{3})\sTRACE jsh-log test trace`,
				}
				for n, line := range output {
					r := regexp.MustCompile(line)
					if !r.MatchString(lines[n]) {
						t.Errorf("log line %d: expected to match %q, got %q", n, line, lines[n])
					}
				}
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestTime(t *testing.T) {
	tests := []TestCase{
		{
			name: "new-Time",
			script: `
				const system = require("@jsh/system");
				console.println(system.time(1, 2).in(system.location("UTC")).toString());
				console.println(system.time(1).in(system.location("UTC")).toString());
				console.println(system.time().in(system.location("UTC")).toString());
				console.println(system.time(0, 0).in(system.location("UTC")).toString());
				console.println(system.time(0).in(system.location("Asia/Tokyo")).toString());
				`,
			output: []string{
				"1970-01-01 00:00:01 +0000 UTC",
				"1970-01-01 00:00:01 +0000 UTC",
				"1970-01-01 00:00:00 +0000 UTC",
				"1970-01-01 00:00:00 +0000 UTC",
				"1970-01-01 09:00:00 +0900 JST",
			},
		},
		{
			name: "timeformat",
			script: `
				const system = require("@jsh/system");
				ts = system.time(1).in(system.location("UTC"));
				console.println(ts.format("2006-01-02 15:04:05"));
				`,
			output: []string{
				"1970-01-01 00:00:01",
			},
		},
		{
			name: "timeformat",
			script: `
				const system = require("@jsh/system");
				ts = system.time(1).in(system.location("UTC"));
				console.println(ts.format("2006-01-02 15:04:05"));
				`,
			output: []string{
				"1970-01-01 00:00:01",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestParseTime(t *testing.T) {
	tests := []TestCase{
		{
			name: "parseTime",
			script: `
				const system = require("@jsh/system");
				ts = system.parseTime(
					"2023-10-01 12:00:00",
					"2006-01-02 15:04:05",
					system.location("UTC"));
				console.println(ts.in(system.location("UTC")).format("2006-01-02 15:04:05"));
				console.println(ts.in(system.location("Asia/Seoul")).format("2006-01-02 15:04:05"));
				`,
			output: []string{
				"2023-10-01 12:00:00",
				"2023-10-01 21:00:00",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestStatz(t *testing.T) {
	tests := []TestCase{
		{
			name: "statz",
			script: `
				const {statz} = require("@jsh/system");
				try {
					console.println(statz("1m", "go:goroutine_max").toString());
				} catch (e) {
				 	console.println(e.toString());
				}
			`,
			output: []string{
				"no metrics found",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}
