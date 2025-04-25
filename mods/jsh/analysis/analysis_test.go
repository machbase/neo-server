package analysis_test

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
	Name   string
	Script string
	Expect []string
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
		jsh.WithNativeModules("@jsh/process", "@jsh/analysis", "@jsh/generator"),
		jsh.WithJshWriter(w),
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
		if !bytes.Equal(line, []byte(tc.Expect[i])) {
			t.Errorf("Expected %q, got %q", tc.Expect[i], line)
		}
	}
	if len(lines) > len(tc.Expect) {
		t.Errorf("Expected %d lines, got %d", len(tc.Expect), len(lines))
	}
}

func TestSort(t *testing.T) {
	tests := []TestCase{
		{
			Name: "sort",
			Script: `
				const ana = require("@jsh/analysis")
				console.log(ana.sort([3, 1, 2]))
				console.log(ana.sort([1.3, 1.2, 1.1]))
			`,
			Expect: []string{
				"[1, 2, 3]",
				"[1.1, 1.2, 1.3]",
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

func TestSum(t *testing.T) {
	tests := []TestCase{
		{
			Name: "sum",
			Script: `
				const ana = require("@jsh/analysis")
				console.log(ana.sum([3, 1, 2]))
				console.log(ana.sum([1.3, 1.2, 1.1]))
			`,
			Expect: []string{
				"6",
				"3.6",
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

func TestCdf(t *testing.T) {
	tests := []TestCase{
		{
			Name: "cdf",
			Script: `
				const ana = require("@jsh/analysis")
				x = [];
				for( i=1; i<=100; i++) {
				x.push(i);
				}
				console.log(ana.cdf(1.0, x))
			`,
			Expect: []string{
				"0.01",
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

func TestCircularMean(t *testing.T) {
	tests := []TestCase{
		{
			Name: "circularMean",
			Script: `
				const ana = require("@jsh/analysis")
				x = [0, 0.25 * Math.PI, 0.75 * Math.PI];
				w = [1, 2, 2.5];
				console.log(ana.circularMean(x).toFixed(4))
				console.log(ana.circularMean(x, w).toFixed(4))
			`,
			Expect: []string{
				"0.9553",
				"1.3704",
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

func TestCorrelation(t *testing.T) {
	tests := []TestCase{
		{
			Name: "correlation",
			Script: `
				const ana = require("@jsh/analysis")
				x = [8, -3, 7, 8, -4];
				y = [10, 5, 6, 3, -1];
				w = [2, 1.5, 3, 3, 2];
				console.log(ana.correlation(x, y).toFixed(5))
				console.log(ana.correlation(x, y, w).toFixed(5))
			`,
			Expect: []string{
				"0.61922",
				"0.59915",
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

func TestCovariance(t *testing.T) {
	tests := []TestCase{
		{
			Name: "covariance",
			Script: `
				const ana = require("@jsh/analysis")
				x = [8, -3, 7, 8, -4];
				y1 = [10, 2, 2, 4, 1];
				y2 = [12, 1, 11, 12, 0];
				console.log(ana.covariance(x, y1).toFixed(4))
				console.log(ana.covariance(x, y2).toFixed(4))
				console.log(ana.variance(x).toFixed(4))
			`,
			Expect: []string{"13.8000", "37.7000", "37.7000", ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestEntropy(t *testing.T) {
	tests := []TestCase{
		{
			Name: "entropy",
			Script: `
				const ana = require("@jsh/analysis")
				console.log(ana.entropy([0.05, 0.1, 0.9, 0.05]).toFixed(4));
				console.log(ana.entropy([0.2, 0.4, 0.25, 0.15]).toFixed(4));
				console.log(ana.entropy([0.2, 0, 0, 0.5, 0, 0.2, 0.1, 0, 0, 0]).toFixed(4));
				console.log(ana.entropy([0, 0, 1, 0]).toFixed(4));
			`,
			Expect: []string{"0.6247", "1.3195", "1.2206", "0.0000", ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestGeometricMean(t *testing.T) {
	tests := []TestCase{
		{
			Name: "geometricMean",
			Script: `
				const ana = require("@jsh/analysis")
				x = [8, 2, 9, 15, 4];
				w = [2, 2, 6, 7, 1];
				console.log(ana.mean(x, w).toFixed(4))
				console.log(ana.geometricMean(x, w).toFixed(4))
				log_x = [];
				for( v of x ) {
					log_x.push(Math.log(v));
				}
				console.log(Math.exp(ana.mean(log_x, w)).toFixed(4));
			`,
			Expect: []string{"10.1667", "8.7637", "8.7637", ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestHarmonicMean(t *testing.T) {
	tests := []TestCase{
		{
			Name: "harmonicMean",
			Script: `
				const ana = require("@jsh/analysis")
				x = [8, 2, 9, 15, 4];
				w = [2, 2, 6, 7, 1];
				console.log(ana.mean(x, w).toFixed(4))
				console.log(ana.harmonicMean(x, w).toFixed(4))
			`,
			Expect: []string{"10.1667", "6.8354", ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestMedian(t *testing.T) {
	tests := []TestCase{
		{
			Name: "median",
			Script: `
				const ana = require("@jsh/analysis")
				x = [];
				for( i=1; i<=100; i++) {
					x.push(i);
				}
				console.log(ana.median(x))
			`,
			Expect: []string{"50", ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestQuantile(t *testing.T) {
	tests := []TestCase{
		{
			Name: "quantile",
			Script: `
				const ana = require("@jsh/analysis")
				x = [];
				for( i=1; i<=100; i++) {
					x.push(i);
				}
				console.log(ana.quantile(0.25, x))
				console.log(ana.quantile(0.90, x))
			`,
			Expect: []string{"25", "90", ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestMean(t *testing.T) {
	tests := []TestCase{
		{
			Name: "mean",
			Script: `
				const ana = require("@jsh/analysis")
				x = [];
				for( i=1; i<=100; i++) {
					x.push(i);
				}
				console.log(ana.mean(x))
			`,
			Expect: []string{"50.5", ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestStdDev(t *testing.T) {
	tests := []TestCase{
		{
			Name: "stdDev",
			Script: `
				const ana = require("@jsh/analysis")
				x = [8, 2, -9, 15, 4];
				w = [2, 2, 6, 7, 1];
				console.log(ana.stdDev(x).toFixed(4))
				console.log(ana.stdDev(x, w).toFixed(4))
			`,
			Expect: []string{"8.8034", "10.5733", ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestStdErr(t *testing.T) {
	tests := []TestCase{
		{
			Name: "stdErr",
			Script: `
				const ana = require("@jsh/analysis")
				x = [8, 2, -9, 15, 4];
				w = [2, 2, 6, 7, 1];

				mean = ana.mean(x, w);
				stddev = ana.stdDev(x, w);
				nSamples = ana.sum(w);
				stdErr = ana.stdErr(stddev, nSamples);

				console.log("stddev", stddev.toFixed(4));
				console.log("nSamples", nSamples.toFixed(4));
				console.log("mean", mean.toFixed(4));
				console.log("stderr", stdErr.toFixed(4));
			`,
			Expect: []string{
				"stddev 10.5733",
				"nSamples 18.0000",
				"mean 4.1667",
				"stderr 2.4921",
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

func TestInterp(t *testing.T) {
	tests := []TestCase{
		{
			Name: "interp",
			Script: `
				const {Simplex} = require("@jsh/generator")
				const {simplex} = new Simplex(123);
				m = require("@jsh/analysis");

				xs = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11];
				ys = [0, 0.001, 0.002, 0.1, 1, 2, 2.5, -10, -10.01, 2.49, 2.53, 2.55];
				pc = m.interpPiecewiseConstant(xs, ys);
				pl = m.interpPiecewiseLinear(xs, ys);
				as = m.interpAkimaSpline(xs, ys);
				fb = m.interpFritschButland(xs, ys);

				n = xs.length;
				dx = 0.25;
				nPts = Math.round((n-1)/dx)+1;
				for( i = 0; i < nPts; i++ ) {
					x = xs[0] + i * dx;
					console.log(` + "`${x.toFixed(2)},${pc.predict(x).toFixed(2)},${pl.predict(x).toFixed(2)},${as.predict(x).toFixed(2)},${fb.predict(x).toFixed(2)}`);" +
				`}`,
			Expect: []string{
				"0.00,0.00,0.00,0.00,0.00",
				"0.25,0.00,0.00,0.00,0.00",
				"0.50,0.00,0.00,0.00,0.00",
				"0.75,0.00,0.00,0.00,0.00",
				"1.00,0.00,0.00,0.00,0.00",
				"1.25,0.00,0.00,0.00,0.00",
				"1.50,0.00,0.00,0.00,0.00",
				"1.75,0.00,0.00,0.00,0.00",
				"2.00,0.00,0.00,0.00,0.00",
				"2.25,0.10,0.03,-0.01,0.01",
				"2.50,0.10,0.05,-0.01,0.03",
				"2.75,0.10,0.08,0.02,0.06",
				"3.00,0.10,0.10,0.10,0.10",
				"3.25,1.00,0.33,0.26,0.22",
				"3.50,1.00,0.55,0.49,0.45",
				"3.75,1.00,0.78,0.75,0.73",
				"4.00,1.00,1.00,1.00,1.00",
				"4.25,2.00,1.25,1.24,1.26",
				"4.50,2.00,1.50,1.50,1.54",
				"4.75,2.00,1.75,1.75,1.79",
				"5.00,2.00,2.00,2.00,2.00",
				"5.25,2.50,2.13,2.22,2.17",
				"5.50,2.50,2.25,2.37,2.33",
				"5.75,2.50,2.38,2.47,2.45",
				"6.00,2.50,2.50,2.50,2.50",
				"6.25,-10.00,-0.63,0.83,0.55",
				"6.50,-10.00,-3.75,-2.98,-3.75",
				"6.75,-10.00,-6.88,-7.18,-8.04",
				"7.00,-10.00,-10.00,-10.00,-10.00",
				"7.25,-10.01,-10.00,-11.16,-10.00",
				"7.50,-10.01,-10.00,-11.55,-10.01",
				"7.75,-10.01,-10.01,-11.18,-10.01",
				"8.00,-10.01,-10.01,-10.01,-10.01",
				"8.25,2.49,-6.88,-7.18,-8.06",
				"8.50,2.49,-3.76,-2.99,-3.77",
				"8.75,2.49,-0.63,0.82,0.53",
				"9.00,2.49,2.49,2.49,2.49",
				"9.25,2.53,2.50,2.50,2.51",
				"9.50,2.53,2.51,2.51,2.52",
				"9.75,2.53,2.52,2.52,2.52",
				"10.00,2.53,2.53,2.53,2.53",
				"10.25,2.55,2.53,2.54,2.54",
				"10.50,2.55,2.54,2.54,2.54",
				"10.75,2.55,2.54,2.55,2.55",
				"11.00,2.55,2.55,2.55,2.55",
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
