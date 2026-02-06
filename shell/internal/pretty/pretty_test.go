package pretty

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machbase/jsh/engine"
	"github.com/machbase/jsh/root"
)

type TestCase struct {
	name   string
	script string
	output []string
	err    string
	vars   map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name: tc.name,
			Code: tc.script,
			FSTabs: []engine.FSTab{
				root.RootFSTab(),
				{MountPoint: "/usr", Source: "../usr/"},
			},
			Env: map[string]any{
				"PATH": "/sbin:/lib:/usr/bin:/usr/lib:/work",
				"PWD":  "/work",
			},
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)
		jr.RegisterNativeModule("@jsh/pretty", Module)

		for k, v := range tc.vars {
			jr.Env.Set(k, v)
		}
		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		lines := strings.Split(gotOutput, "\n")
		if len(lines) != len(tc.output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
		}
		for i, expectedLine := range tc.output {
			if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

func TestBytes(t *testing.T) {
	tests := []TestCase{
		{
			name: "Bytes_various_sizes",
			script: `
				const pretty = require('/usr/lib/pretty');
				console.println(pretty.Bytes(512));
				console.println(pretty.Bytes(1536));
				console.println(pretty.Bytes(1048576));
				console.println(pretty.Bytes(1073741824));
				console.println(pretty.Bytes(1099511627776));
			`,
			output: []string{
				"512B",
				"1.5KB",
				"1.0MB",
				"1.0GB",
				"1.0TB",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestInts(t *testing.T) {
	tests := []TestCase{
		{
			name: "Ints_formatting",
			script: `
				const pretty = require('/usr/lib/pretty');
				console.println(pretty.Ints(1234567890));
				console.println(pretty.Ints(0));
				console.println(pretty.Ints(-999));
			`,
			output: []string{
				"1,234,567,890",
				"0",
				"-999",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestDurations(t *testing.T) {
	tests := []TestCase{
		{
			name: "Durations_nanoseconds",
			script: `
				const pretty = require('/usr/lib/pretty');
				console.println(pretty.Durations(1));
				console.println(pretty.Durations(500));
				console.println(pretty.Durations(999));
			`,
			output: []string{
				"1ns",
				"500ns",
				"999ns",
			},
		},
		{
			name: "Durations_microseconds",
			script: `
				const pretty = require('/usr/lib/pretty');
				console.println(pretty.Durations(1000));
				console.println(pretty.Durations(1234));
				console.println(pretty.Durations(5000));
				console.println(pretty.Durations(999000));
			`,
			output: []string{
				"1μs",
				"1.23μs",
				"5μs",
				"999μs",
			},
		},
		{
			name: "Durations_milliseconds",
			script: `
				const pretty = require('/usr/lib/pretty');
				console.println(pretty.Durations(1000000));
				console.println(pretty.Durations(2340000));
				console.println(pretty.Durations(100000000));
				console.println(pretty.Durations(999000000));
			`,
			output: []string{
				"1ms",
				"2.34ms",
				"100ms",
				"999ms",
			},
		},
		{
			name: "Durations_seconds",
			script: `
				const pretty = require('/usr/lib/pretty');
				console.println(pretty.Durations(1000000000));
				console.println(pretty.Durations(3010000000));
				console.println(pretty.Durations(45000000000));
				console.println(pretty.Durations(59000000000));
			`,
			output: []string{
				"1s",
				"3.01s",
				"45s",
				"59s",
			},
		},
		{
			name: "Durations_minutes_hours",
			script: `
				const pretty = require('/usr/lib/pretty');
				console.println(pretty.Durations(60000000000));
				console.println(pretty.Durations(125000000000));
				console.println(pretty.Durations(3661000000000));
				console.println(pretty.Durations(7200000000000));
			`,
			output: []string{
				"1m 0s",
				"2m 5s",
				"1h 1m",
				"2h 0m",
			},
		},
		{
			name: "Durations_days",
			script: `
				const pretty = require('/usr/lib/pretty');
				console.println(pretty.Durations(86400000000000));
				console.println(pretty.Durations(90061000000000));
				console.println(pretty.Durations(172861000000000));
			`,
			output: []string{
				"1d 0h",
				"1d 1h",
				"2d 0h",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}
