package filter_test

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
)

type TestCase struct {
	Name       string
	Script     string
	Expect     []string
	ExpectFunc func(t *testing.T, result string)
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
	if strings.HasSuffix(file, ".csv") {
		lines = append(lines, "")
	}
	return lines
}

func runTestCase(t *testing.T, tc TestCase) {
	t.Helper()
	ctx := context.TODO()
	w := &bytes.Buffer{}
	j := jsh.NewJsh(ctx,
		jsh.WithNativeModules("@jsh/process", "@jsh/generator", "@jsh/filter", "@jsh/system"),
		jsh.WithWriter(w),
	)
	err := j.Run(tc.Name, tc.Script, nil)
	if err != nil {
		t.Fatalf("Error running script: %s", err)
	}

	if tc.ExpectFunc != nil {
		tc.ExpectFunc(t, w.String())
		return
	} else {
		lines := bytes.Split(w.Bytes(), []byte{'\n'})
		for i, line := range lines {
			if i >= len(tc.Expect) {
				break
			}
			if !bytes.Equal(line, []byte(tc.Expect[i])) {
				t.Errorf("Expected %q, got %q", tc.Expect[i], line)
			}
		}
		if len(lines) > len(tc.Expect) {
			t.Errorf("Expected %d lines, got %d", len(tc.Expect), len(lines))
		}
	}
}

func TestAvg(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-avg",
			Script: `
				const { arrange } = require("@jsh/generator");
				const m = require("@jsh/filter")
				const avg = new m.Avg();
				for( x of arrange(10, 30, 10) ) {
					console.log(x,  avg.eval(x).toFixed(2));
				}
			`,
			Expect: []string{
				"10 10.00",
				"20 15.00",
				"30 20.00",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestMovAvg(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-movavg",
			Script: `
				const { linspace } = require("@jsh/generator");
				const m = require("@jsh/filter")
				const movAvg = new m.MovAvg(10);
				for( x of linspace(0, 100, 100) ) {
					console.log(""+x.toFixed(4)+","+movAvg.eval(x).toFixed(4));
				}
			`,
			Expect: loadLines("../../tql/test/movavg_result_nowait.csv"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestLowpass(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-lowpass",
			Script: `
				const { arrange, Simplex } = require("@jsh/generator");
				const m = require("@jsh/filter")
				const lpf = new m.Lowpass(0.3);
				const simplex = new Simplex(1);

				for( x of arrange(1, 10, 1) ) {
					v = x + simplex.eval(x) * 3;
					console.log(x, v.toFixed(2), lpf.eval(v).toFixed(2));
				}
			`,
			Expect: []string{
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
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestKalman(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-kalman",
			Script: `
				const s = require("@jsh/system")
				const m = require("@jsh/filter");
				const kalman = new m.Kalman(1.0, 1.0, 2.0);
				var ts = 1745484444000; // ms

				for( x of [1.3, 10.2, 5.0, 3.4] ) {
					ts += 1000;
					console.log(kalman.eval(s.parseTime(ts,"ms"), x).toFixed(3));
				}
			`,
			Expect: []string{
				`1.300`,
				`5.750`,
				`5.375`,
				`4.388`,
				"",
			},
		},
		{
			Name: "js-kalman-variances",
			Script: `
				const s = require("@jsh/system")
				const m = require("@jsh/filter");
				const kalman = new m.Kalman({initialVariance: 1.0, processVariance: 1.0, observationVariance: 2.0});
				var ts = 1745484444000; // ms
			
				for( x of [1.3, 10.2, 5.0, 3.4] ) {
					ts += 1000;
					console.log(kalman.eval(s.parseTime(ts,"ms"), x).toFixed(3));
				}
			`,
			Expect: []string{
				`1.300`,
				`5.750`,
				`5.375`,
				`4.388`,
				"",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestKalmanSmoother(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-kalman-smoother",
			Script: `
				const s = require("@jsh/system");
				const m = require("@jsh/filter");
				const kalman = new m.KalmanSmoother(1.0, 1.0, 2.0);
				var ts = 1745484444000; // ms
				for( x of [1.3, 10.2, 5.0, 3.4] ) {
					ts += 1000;
					console.log(kalman.eval(s.parseTime(ts,"ms"), x).toFixed(2));
				}
			`,
			Expect: []string{
				`1.30`,
				`5.75`,
				`3.52`,
				`2.70`,
				"",
			},
		},
		{
			Name: "js-kalman-smoother-variances",
			Script: `
				const s = require("@jsh/system");
				const m = require("@jsh/filter");
				const kalman = new m.KalmanSmoother({initialVariance: 1.0, processVariance: 1.0, observationVariance: 2.0});
				var ts = 1745484444000; // ms			
				for( x of [1.3, 10.2, 5.0, 3.4] ) {
					ts += 1000;
					console.log(kalman.eval(s.parseTime(ts,"ms"), x).toFixed(2));
				}
			`,
			Expect: []string{
				`1.30`,
				`5.75`,
				`3.52`,
				`2.70`,
				"",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}
