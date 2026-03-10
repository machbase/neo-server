package mathx_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestMeshgrid(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "js-meshgrid",
			Script: `
				gen = require("mathx").meshgrid([1, 2, 3], [4, 5]);
				for(i=0; i < gen.length; i++) {
					console.println(JSON.stringify(gen[i]));
				}
			`,
			Output: []string{
				"[1,4]",
				"[1,5]",
				"[2,4]",
				"[2,5]",
				"[3,4]",
				"[3,5]",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestArrange(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "js-arrange",
			Script: `
				gen = require("mathx").arrange(1, 5, 1);
				console.println(JSON.stringify(gen));
			`,
			Output: []string{
				"[1,2,3,4,5]",
			},
		},
		{
			Name: "js-arrange-desc",
			Script: `
				gen = require("mathx").arrange(5, 1, -1);
				console.println(JSON.stringify(gen));
			`,
			Output: []string{
				"[5,4,3,2,1]",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestLinspace(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "js-linspace",
			Script: `
				gen = require("mathx").linspace(1, 5, 5);
				for(i=0; i < gen.length; i++) {
					console.println(gen[i]);
				}
			`,
			Output: []string{
				"1",
				"2",
				"3",
				"4",
				"5",
			},
		},
		{
			Name: "js-linspace-float",
			Script: `
				gen = require("mathx").linspace(0, 1, 5);
				for(i=0; i < gen.length; i++) {
					console.println(gen[i].toFixed(2));
				}
			`,
			Output: []string{
				"0.00",
				"0.25",
				"0.50",
				"0.75",
				"1.00",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestSort(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "sort",
			Script: `
				const mathx = require("mathx")
				console.println(mathx.sort([3, 1, 2]))
				console.println(mathx.sort([1.3, 1.2, 1.1]))
			`,
			Output: []string{
				"[1, 2, 3]",
				"[1.1, 1.2, 1.3]",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestSum(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "sum",
			Script: `
				const mathx = require("mathx")
				console.println(mathx.sum([3, 1, 2]))
				console.println(mathx.sum([1.3, 1.2, 1.1]))
			`,
			Output: []string{
				"6",
				"3.6",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestCdf(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "cdf",
			Script: `
				const mathx = require("mathx")
				x = [];
				for( i=1; i<=100; i++) {
					x.push(i);
				}
				console.println(mathx.cdf(1.0, x))
			`,
			Output: []string{
				"0.01",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestCircularMean(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "circularMean",
			Script: `
				const mathx = require("mathx")
				x = [0, 0.25 * Math.PI, 0.75 * Math.PI];
				w = [1, 2, 2.5];
				console.println(mathx.circularMean(x).toFixed(4))
				console.println(mathx.circularMean(x, w).toFixed(4))
			`,
			Output: []string{
				"0.9553",
				"1.3704",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestCorrelation(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "correlation",
			Script: `
				const mathx = require("mathx")
				x = [8, -3, 7, 8, -4];
				y = [10, 5, 6, 3, -1];
				w = [2, 1.5, 3, 3, 2];
				console.println(mathx.correlation(x, y).toFixed(5))
				console.println(mathx.correlation(x, y, w).toFixed(5))
			`,
			Output: []string{
				"0.61922",
				"0.59915",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestCovariance(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "covariance",
			Script: `
				const mathx = require("mathx")
				x = [8, -3, 7, 8, -4];
				y1 = [10, 2, 2, 4, 1];
				y2 = [12, 1, 11, 12, 0];
				console.println(mathx.covariance(x, y1).toFixed(4))
				console.println(mathx.covariance(x, y2).toFixed(4))
				console.println(mathx.variance(x).toFixed(4))
			`,
			Output: []string{
				"13.8000",
				"37.7000",
				"37.7000",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestEntropy(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "entropy",
			Script: `
				const mathx = require("mathx")
				console.println(mathx.entropy([0.05, 0.1, 0.9, 0.05]).toFixed(4));
				console.println(mathx.entropy([0.2, 0.4, 0.25, 0.15]).toFixed(4));
				console.println(mathx.entropy([0.2, 0, 0, 0.5, 0, 0.2, 0.1, 0, 0, 0]).toFixed(4));
				console.println(mathx.entropy([0, 0, 1, 0]).toFixed(4));
			`,
			Output: []string{
				"0.6247",
				"1.3195",
				"1.2206",
				"0.0000",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestGeometricMean(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "geometricMean",
			Script: `
				const mathx = require("mathx")
				x = [8, 2, 9, 15, 4];
				w = [2, 2, 6, 7, 1];
				console.println(mathx.mean(x, w).toFixed(4))
				console.println(mathx.geometricMean(x, w).toFixed(4))
				log_x = [];
				for( v of x ) {
					log_x.push(Math.log(v));
				}
				console.println(Math.exp(mathx.mean(log_x, w)).toFixed(4));
			`,
			Output: []string{
				"10.1667",
				"8.7637",
				"8.7637",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestHarmonicMean(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "harmonicMean",
			Script: `
				const mathx = require("mathx")
				x = [8, 2, 9, 15, 4];
				w = [2, 2, 6, 7, 1];
				console.println(mathx.mean(x, w).toFixed(4))
				console.println(mathx.harmonicMean(x, w).toFixed(4))
			`,
			Output: []string{
				"10.1667",
				"6.8354",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestMedian(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "median",
			Script: `
				const mathx = require("mathx")
				x = [];
				for( i=1; i<=100; i++) {
					x.push(i);
				}
				console.println(mathx.median(x))
			`,
			Output: []string{"50"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestQuantile(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "quantile",
			Script: `
				const mathx = require("mathx")
				x = [];
				for( i=1; i<=100; i++) {
					x.push(i);
				}
				console.println(mathx.quantile(0.25, x))
				console.println(mathx.quantile(0.90, x))
			`,
			Output: []string{"25", "90"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestMean(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "mean",
			Script: `
				const mathx = require("mathx")
				x = [];
				for( i=1; i<=100; i++) {
					x.push(i);
				}
				console.println(mathx.mean(x))
			`,
			Output: []string{"50.5"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestStdDev(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stdDev",
			Script: `
				const mathx = require("mathx")
				x = [8, 2, -9, 15, 4];
				w = [2, 2, 6, 7, 1];
				console.println(mathx.stdDev(x).toFixed(4))
				console.println(mathx.stdDev(x, w).toFixed(4))
			`,
			Output: []string{"8.8034", "10.5733"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestStdErr(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stdErr",
			Script: `
				const mathx = require("mathx")
				x = [8, 2, -9, 15, 4];
				w = [2, 2, 6, 7, 1];

				mean = mathx.mean(x, w);
				stddev = mathx.stdDev(x, w);
				nSamples = mathx.sum(w);
				stdErr = mathx.stdErr(stddev, nSamples);

				console.println("stddev", stddev.toFixed(4));
				console.println("nSamples", nSamples.toFixed(4));
				console.println("mean", mean.toFixed(4));
				console.println("stderr", stdErr.toFixed(4));
			`,
			Output: []string{
				"stddev 10.5733",
				"nSamples 18.0000",
				"mean 4.1667",
				"stderr 2.4921",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "mean-weight-length-check",
			Script: `
				const mathx = require("mathx")
				try {
					mathx.mean([1, 2, 3], [1, 2])
				} catch (e) {
					console.println(e.message)
				}
			`,
			Output: []string{
				"mean: x and weight should be the same length",
			},
		},
		{
			Name: "correlation-length-check",
			Script: `
				const mathx = require("mathx")
				try {
					mathx.correlation([1, 2, 3], [1, 2])
				} catch (e) {
					console.println(e.message)
				}
			`,
			Output: []string{
				"correlation: x and y should be the same length",
			},
		},
		{
			Name: "correlation-weight-length-check",
			Script: `
				const mathx = require("mathx")
				try {
					mathx.correlation([1, 2, 3], [4, 5, 6], [1, 2])
				} catch (e) {
					console.println(e.message)
				}
			`,
			Output: []string{
				"correlation: x, y and weight should be the same length",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}
