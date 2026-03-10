package interp_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestInterp(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "interp",
			Script: `
				m = require("mathx/interp");

				xs = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11];
				ys = [0, 0.001, 0.002, 0.1, 1, 2, 2.5, -10, -10.01, 2.49, 2.53, 2.55];
				pc = new m.PiecewiseConstant();
				pc.fit(xs, ys);
				pl = new m.PiecewiseLinear();
				pl.fit(xs, ys);
				as = new m.AkimaSpline();
				as.fit(xs, ys);
				fb = new m.FritschButland();
				fb.fit(xs, ys);
				lr = new m.LinearRegression();
				lr.fit(xs, ys);

				n = xs.length;
				dx = 0.25;
				nPts = Math.round((n-1)/dx)+1;
				for( i = 0; i < nPts; i++ ) {
					x = xs[0] + i * dx;
					console.println(` + "`${x.toFixed(2)},${pc.predict(x).toFixed(2)},${pl.predict(x).toFixed(2)},${as.predict(x).toFixed(2)},${fb.predict(x).toFixed(2)},${lr.predict(x).toFixed(2)}`);" +
				`}`,
			Output: []string{
				"0.00,0.00,0.00,0.00,0.00,-0.28",
				"0.25,0.00,0.00,0.00,0.00,-0.30",
				"0.50,0.00,0.00,0.00,0.00,-0.31",
				"0.75,0.00,0.00,0.00,0.00,-0.32",
				"1.00,0.00,0.00,0.00,0.00,-0.34",
				"1.25,0.00,0.00,0.00,0.00,-0.35",
				"1.50,0.00,0.00,0.00,0.00,-0.36",
				"1.75,0.00,0.00,0.00,0.00,-0.38",
				"2.00,0.00,0.00,0.00,0.00,-0.39",
				"2.25,0.10,0.03,-0.01,0.01,-0.40",
				"2.50,0.10,0.05,-0.01,0.03,-0.41",
				"2.75,0.10,0.08,0.02,0.06,-0.43",
				"3.00,0.10,0.10,0.10,0.10,-0.44",
				"3.25,1.00,0.33,0.26,0.22,-0.45",
				"3.50,1.00,0.55,0.49,0.45,-0.47",
				"3.75,1.00,0.78,0.75,0.73,-0.48",
				"4.00,1.00,1.00,1.00,1.00,-0.49",
				"4.25,2.00,1.25,1.24,1.26,-0.50",
				"4.50,2.00,1.50,1.50,1.54,-0.52",
				"4.75,2.00,1.75,1.75,1.79,-0.53",
				"5.00,2.00,2.00,2.00,2.00,-0.54",
				"5.25,2.50,2.13,2.22,2.17,-0.56",
				"5.50,2.50,2.25,2.37,2.33,-0.57",
				"5.75,2.50,2.38,2.47,2.45,-0.58",
				"6.00,2.50,2.50,2.50,2.50,-0.60",
				"6.25,-10.00,-0.63,0.83,0.55,-0.61",
				"6.50,-10.00,-3.75,-2.98,-3.75,-0.62",
				"6.75,-10.00,-6.88,-7.18,-8.04,-0.63",
				"7.00,-10.00,-10.00,-10.00,-10.00,-0.65",
				"7.25,-10.01,-10.00,-11.16,-10.00,-0.66",
				"7.50,-10.01,-10.00,-11.55,-10.01,-0.67",
				"7.75,-10.01,-10.01,-11.18,-10.01,-0.69",
				"8.00,-10.01,-10.01,-10.01,-10.01,-0.70",
				"8.25,2.49,-6.88,-7.18,-8.06,-0.71",
				"8.50,2.49,-3.76,-2.99,-3.77,-0.73",
				"8.75,2.49,-0.63,0.82,0.53,-0.74",
				"9.00,2.49,2.49,2.49,2.49,-0.75",
				"9.25,2.53,2.50,2.50,2.51,-0.76",
				"9.50,2.53,2.51,2.51,2.52,-0.78",
				"9.75,2.53,2.52,2.52,2.52,-0.79",
				"10.00,2.53,2.53,2.53,2.53,-0.80",
				"10.25,2.55,2.53,2.54,2.54,-0.82",
				"10.50,2.55,2.54,2.54,2.54,-0.83",
				"10.75,2.55,2.54,2.55,2.55,-0.84",
				"11.00,2.55,2.55,2.55,2.55,-0.85",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestInterpCubic(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "interp-cubic",
			Script: `
				m = require("mathx/interp");

				xs = [0, 1, 2, 3, 4];
				ys = [0, 10, 20, 30, 40];
				cc = new m.ClampedCubic();
				cc.fit(xs, ys);
				nc = new m.NaturalCubic();
				nc.fit(xs, ys);
				kn = new m.NotAKnotCubic();
				kn.fit(xs, ys);

				for( x of [0.5, 1.5, 2.5, 3.5] ) {
					cp = cc.predict(x);
					np = nc.predict(x);
					nd = nc.predictDerivative(x);
					kp = kn.predict(x);
					console.println(` + "`${x.toFixed(2)},${cp.toFixed(2)},${np.toFixed(2)},${nd.toFixed(2)},${kp.toFixed(2)}`);" +
				`}`,
			Output: []string{
				"0.50,3.39,5.00,10.00,5.00",
				"1.50,15.54,15.00,10.00,15.00",
				"2.50,24.46,25.00,10.00,25.00",
				"3.50,36.61,35.00,10.00,35.00",
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
			Name: "fit-missing-args",
			Script: `
				const m = require("mathx/interp");
				const pc = new m.PiecewiseConstant();
				try {
					pc.fit([0, 1, 2]);
				} catch (e) {
					console.println(e.message);
				}
			`,
			Output: []string{
				"fit: x and y are required",
			},
		},
		{
			Name: "fit-length-mismatch",
			Script: `
				const m = require("mathx/interp");
				const pc = new m.PiecewiseConstant();
				try {
					pc.fit([0, 1, 2], [0, 1]);
				} catch (e) {
					console.println(e.message);
				}
			`,
			Output: []string{
				"fit: x and y should be the same length",
			},
		},
		{
			Name: "predict-missing-arg",
			Script: `
				const m = require("mathx/interp");
				const pc = new m.PiecewiseConstant();
				pc.fit([0, 1], [0, 1]);
				try {
					pc.predict();
				} catch (e) {
					console.println(e.message);
				}
			`,
			Output: []string{
				"predict: x is required",
			},
		},
		{
			Name: "predict-derivative-not-supported",
			Script: `
				const m = require("mathx/interp");
				const lr = new m.LinearRegression();
				lr.fit([0, 1], [0, 1]);
				if (typeof lr.predictDerivative === 'function') {
					console.println("predictDerivative is supported");
				} else {
					console.println("predictDerivative is not supported");
				}
			`,
			Output: []string{
				"predictDerivative is not supported",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}
