package system_test

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
)

type TestCase struct {
	Name      string
	Script    string
	UseRegex  bool
	Expect    []string
	ExpectLog []string
}

func runTest(t *testing.T, tc TestCase) {
	t.Helper()
	ctx := context.TODO()
	w := &bytes.Buffer{}
	j := jsh.NewJsh(ctx,
		jsh.WithNativeModules("@jsh/process", "@jsh/system"),
		jsh.WithWriter(w),
	)
	err := j.Run(tc.Name, tc.Script, nil)
	if err != nil {
		t.Fatalf("Error running script: %s", err)
	}
	lines := bytes.Split(w.Bytes(), []byte{'\n'})
	for i, line := range lines {
		if i >= len(tc.Expect) {
			break
		}
		if tc.UseRegex {
			re, err := regexp.Compile(tc.Expect[i])
			if err != nil {
				t.Fatalf("Error compiling regex: %s", err)
			}
			if !re.Match(line) {
				t.Errorf("Expected regex %q, got %q", tc.Expect[i], line)
			}
		} else {
			if !bytes.Equal(line, []byte(tc.Expect[i])) {
				t.Errorf("Expected %q, got %q", tc.Expect[i], line)
			}
		}
	}
	if len(lines) > len(tc.Expect) {
		t.Errorf("Expected %d lines, got %d", len(tc.Expect), len(lines))
	}
}

func TestTime(t *testing.T) {
	tests := []TestCase{
		{
			Name: "new-Time",
			Script: `
				const {println} = require("@jsh/process");
				const system = require("@jsh/system");
				println(system.time(1, 2).In(system.location("UTC")).toString());
				println(system.time(1).In(system.location("UTC")).toString());
				println(system.time().In(system.location("UTC")).toString());
				println(system.time(0, 0).In(system.location("UTC")).toString());
				println(system.time(0).In(system.location("Asia/Tokyo")).toString());
				`,
			Expect: []string{
				"1970-01-01 00:00:01 +0000 UTC",
				"1970-01-01 00:00:01 +0000 UTC",
				"1970-01-01 00:00:00 +0000 UTC",
				"1970-01-01 00:00:00 +0000 UTC",
				"1970-01-01 09:00:00 +0900 JST",
				"",
			},
		},
		{
			Name: "timeformat",
			Script: `
				const {println} = require("@jsh/process");
				const system = require("@jsh/system");
				ts = system.time(1).In(system.location("UTC"));
				println(ts.Format("2006-01-02 15:04:05"));
				`,
			Expect: []string{
				"1970-01-01 00:00:01",
				"",
			},
		},
		{
			Name: "timeformat",
			Script: `
				const {println} = require("@jsh/process");
				const system = require("@jsh/system");
				ts = system.time(1).In(system.location("UTC"));
				println(ts.Format("2006-01-02 15:04:05"));
				`,
			Expect: []string{
				"1970-01-01 00:00:01",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []TestCase{
		{
			Name: "parseTime",
			Script: `
				const {println} = require("@jsh/process");
				const system = require("@jsh/system");
				ts = system.parseTime(
					"2023-10-01 12:00:00",
					"2006-01-02 15:04:05",
					system.location("UTC"));
				println(ts.In(system.location("UTC")).Format("2006-01-02 15:04:05"));
				println(ts.In(system.location("Asia/Seoul")).Format("2006-01-02 15:04:05"));
				`,
			Expect: []string{
				"2023-10-01 12:00:00",
				"2023-10-01 21:00:00",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestStatz(t *testing.T) {
	tests := []TestCase{
		{
			Name: "statz",
			Script: `
				const {print} = require("@jsh/process");
				const {statz} = require("@jsh/system");
				try {
					print(statz("1m", "go:goroutine_max").toString());
				} catch (e) {
				 	print(e.toString());
				}
			`,
			Expect: []string{
				"no metrics found",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}
