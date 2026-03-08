package pretty_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestBytes(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "Bytes_various_sizes",
			Script: `
				const pretty = require('pretty');
				console.println(pretty.Bytes(512));
				console.println(pretty.Bytes(1536));
				console.println(pretty.Bytes(1048576));
				console.println(pretty.Bytes(1073741824));
				console.println(pretty.Bytes(1099511627776));
			`,
			Output: []string{
				"512B",
				"1.5KB",
				"1.0MB",
				"1.0GB",
				"1.0TB",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestInts(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "Ints_formatting",
			Script: `
				const pretty = require('pretty');
				console.println(pretty.Ints(1234567890));
				console.println(pretty.Ints(0));
				console.println(pretty.Ints(-999));
			`,
			Output: []string{
				"1,234,567,890",
				"0",
				"-999",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestDurations(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "Durations_nanoseconds",
			Script: `
				const pretty = require('pretty');
				console.println(pretty.Durations(1));
				console.println(pretty.Durations(500));
				console.println(pretty.Durations(999));
			`,
			Output: []string{
				"1ns",
				"500ns",
				"999ns",
			},
		},
		{
			Name: "Durations_microseconds",
			Script: `
				const pretty = require('pretty');
				console.println(pretty.Durations(1000));
				console.println(pretty.Durations(1234));
				console.println(pretty.Durations(5000));
				console.println(pretty.Durations(999000));
			`,
			Output: []string{
				"1μs",
				"1.23μs",
				"5μs",
				"999μs",
			},
		},
		{
			Name: "Durations_milliseconds",
			Script: `
				const pretty = require('pretty');
				console.println(pretty.Durations(1000000));
				console.println(pretty.Durations(2340000));
				console.println(pretty.Durations(100000000));
				console.println(pretty.Durations(999000000));
			`,
			Output: []string{
				"1ms",
				"2.34ms",
				"100ms",
				"999ms",
			},
		},
		{
			Name: "Durations_seconds",
			Script: `
				const pretty = require('pretty');
				console.println(pretty.Durations(1000000000));
				console.println(pretty.Durations(3010000000));
				console.println(pretty.Durations(45000000000));
				console.println(pretty.Durations(59000000000));
			`,
			Output: []string{
				"1s",
				"3.01s",
				"45s",
				"59s",
			},
		},
		{
			Name: "Durations_minutes_hours",
			Script: `
				const pretty = require('pretty');
				console.println(pretty.Durations(60000000000));
				console.println(pretty.Durations(125000000000));
				console.println(pretty.Durations(3661000000000));
				console.println(pretty.Durations(7200000000000));
			`,
			Output: []string{
				"1m 0s",
				"2m 5s",
				"1h 1m",
				"2h 0m",
			},
		},
		{
			Name: "Durations_days",
			Script: `
				const pretty = require('pretty');
				console.println(pretty.Durations(86400000000000));
				console.println(pretty.Durations(90061000000000));
				console.println(pretty.Durations(172861000000000));
			`,
			Output: []string{
				"1d 0h",
				"1d 1h",
				"2d 0h",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
