package filter

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/native/generator"
	"github.com/machbase/neo-server/v8/jsh/root"
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
	conf := engine.Config{
		Name:   tc.name,
		Code:   tc.script,
		FSTabs: []engine.FSTab{root.RootFSTab()},
		Env:    tc.vars,
		Reader: &bytes.Buffer{},
		Writer: &bytes.Buffer{},
	}
	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("Failed to create JSRuntime: %v", err)
	}

	jr.RegisterNativeModule("@jsh/filter", Module)
	jr.RegisterNativeModule("@jsh/generator", generator.Module)

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

func loadLines(file string) []string {
	data, _ := os.ReadFile(file)
	r := bufio.NewReader(bytes.NewBuffer(data))
	lines := []string{}
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			break
		}
		lines = append(lines, string(line))
	}
	return lines
}

func TestAvg(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-avg",
			script: `
				const { arrange } = require("generator");
				const m = require("filter")
				const avg = new m.avg();
				for( x of arrange(10, 30, 10) ) {
					console.println(x,  avg.eval(x).toFixed(2));
				}
			`,
			output: []string{
				"10 10.00",
				"20 15.00",
				"30 20.00",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}

func TestMovAvg(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-movavg",
			script: `
				const { linspace } = require("generator");
				const m = require("filter")
				const movAvg = new m.movavg(10);
				for( x of linspace(0, 100, 100) ) {
					console.println(""+x.toFixed(4)+","+movAvg.eval(x).toFixed(4));
				}
			`,
			output: loadLines("../../../mods/tql/test/movavg_result_nowait.csv"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}

func TestLowpass(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-lowpass",
			script: `
				const { arrange, Simplex } = require("generator");
				const m = require("filter")
				const lpf = new m.lowpass(0.3);
				const simplex = new Simplex(1);

				for( x of arrange(1, 10, 1) ) {
					v = x + simplex.eval(x) * 3;
					console.println(x, v.toFixed(2), lpf.eval(v).toFixed(2));
				}
			`,
			output: []string{
				`1 1.48 1.48`,
				`2 0.40 1.15`,
				`3 3.84 1.96`,
				`4 2.89 2.24`,
				`5 5.47 3.21`,
				`6 5.29 3.83`,
				`7 7.22 4.85`,
				`8 10.31 6.49`,
				`9 8.36 7.05`,
				`10 8.56 7.50`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}

func TestKalman(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-kalman",
			script: `
				const m = require("filter");
				const kalman = new m.kalman(1.0, 1.0, 2.0);
				var ts = 1745484444000; // ms

				for( x of [1.3, 10.2, 5.0, 3.4] ) {
					ts += 1000;
					console.println(kalman.eval(new Date(ts), x).toFixed(3));
				}
			`,
			output: []string{
				`1.300`,
				`5.750`,
				`5.375`,
				`4.388`,
			},
		},
		{
			name: "js-kalman-variances",
			script: `
				const m = require("filter");
				const kalman = new m.kalman({initialVariance: 1.0, processVariance: 1.0, observationVariance: 2.0});
				var ts = 1745484444000; // ms
			
				for( x of [1.3, 10.2, 5.0, 3.4] ) {
					ts += 1000;
					console.println(kalman.eval(new Date(ts), x).toFixed(3));
				}
			`,
			output: []string{
				`1.300`,
				`5.750`,
				`5.375`,
				`4.388`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}

func TestKalmanSmoother(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-kalman-smoother",
			script: `
				const m = require("filter");
				const kalman = new m.kalmanSmoother(1.0, 1.0, 2.0);
				var ts = 1745484444000; // ms
				for( x of [1.3, 10.2, 5.0, 3.4] ) {
					ts += 1000;
					console.println(kalman.eval(new Date(ts), x).toFixed(2));
				}
			`,
			output: []string{
				`1.30`,
				`5.75`,
				`3.52`,
				`2.70`,
			},
		},
		{
			name: "js-kalman-smoother-variances",
			script: `
				const m = require("filter");
				const kalman = new m.kalmanSmoother({initialVariance: 1.0, processVariance: 1.0, observationVariance: 2.0});
				var ts = 1745484444000; // ms			
				for( x of [1.3, 10.2, 5.0, 3.4] ) {
					ts += 1000;
					console.println(kalman.eval(new Date(ts), x).toFixed(2));
				}
			`,
			output: []string{
				`1.30`,
				`5.75`,
				`3.52`,
				`2.70`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}
