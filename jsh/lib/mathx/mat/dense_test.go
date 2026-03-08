package mat_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestDenseSet(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_set",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2)
				a.set(0, 0, 1)
				a.set(1, 1, 1)
				console.println(m.format(a, {
					format: "a = %v", prefix: "    ", squeeze: true,
				}))
			`,
			Output: []string{
				"a = ⎡1  0⎤",
				"    ⎣0  1⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDenseAdd(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_add",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2, [
					1, 0,
					1, 0,
				])
				b = new m.Dense(2, 2, [
					0, 1,
					0, 1,
				])
				c = new m.Dense(2, 2)
				c.add(a, b)
				console.println(m.format(c, {
					format: "c = %v", prefix: "    ",
				}))
			`,
			Output: []string{
				"c = ⎡1  1⎤",
				"    ⎣1  1⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDenseSub(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_sub",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2, [
					1, 1,
					1, 1,
				])
				b = new m.Dense(2, 2, [
					1, 0,
					0, 1,
				])
				a.sub(a, b)
				console.println(m.format(a, {
					format: "a = %v", prefix: "    ",
				}))
			`,
			Output: []string{
				"a = ⎡0  1⎤",
				"    ⎣1  0⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDenseMulElem(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_mul_elem",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2, [
					1, 2,
					3, 4,
				])
				b = new m.Dense(2, 2, [
					1, 2,
					3, 4,
				])
				a.mulElem(a, b)
				console.println(m.format(a, {
					format: "a = %v", prefix: "    ", squeeze: true,
				}))
			`,
			Output: []string{
				"a = ⎡1   4⎤",
				"    ⎣9  16⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDenseMul(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_mul",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2, [
					4, 0,
					0, 4,
				])
				b = new m.Dense(2, 3, [
					4, 0, 0,
					0, 0, 4,
				])
				c = new m.Dense()
				c.mul(a, b)
				console.println(m.format(c, {
					format: "c = %v", prefix: "    ", squeeze: true,
				}))
			`,
			Output: []string{
				"c = ⎡16  0   0⎤",
				"    ⎣ 0  0  16⎦",
			},
		},
		{
			Name: "dense_mul",
			Script: `const m = require("mathx/mat")
				A = new m.Dense(2, 2, [
					1, 2,
					3, 4,
				])
				b = new m.VecDense(2, [
					2,
					2,
				])
				C = new m.Dense()
				C.mul(A, b)
				console.println(m.format(C, {
					format: "C = %v", prefix: "    ", squeeze: true,
				}))
			`,
			Output: []string{
				"C = ⎡ 6⎤",
				"    ⎣14⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDenseDivElem(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_div_elem",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2, [
					5, 10,
					15, 20,
				])
				b = new m.Dense(2, 2, [
					5, 5,
					5, 5,
				])
				a.divElem(a, b)
				console.println(m.format(a, {
					format: "a = %v", prefix: "    ", squeeze: true,
				}))
			`,
			Output: []string{
				"a = ⎡1  2⎤",
				"    ⎣3  4⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDenseExp(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_exp",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2, [
					1, 0,
					0, 1,
				])
				b = new m.Dense()
				b.exp(a)
				console.println(m.format(b, {
					format: "b = %4.2f", prefix: "    ", squeeze: true,
				}))
			`,
			Output: []string{
				"b = ⎡2.72  0.00⎤",
				"    ⎣0.00  2.72⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDensePow(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_pow",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2, [
					4, 4,
					4, 4,
				])
				b = new m.Dense()
				b.pow(a, 2)
				console.println(m.format(b, {
					format: "b = %v", prefix: "    ", squeeze: true,
				}))
				n = new m.Dense()
				n.pow(a, 0)
				console.println(m.format(n, {
					format: "n = %v", prefix: "    ", squeeze: true,
				}))
			`,
			Output: []string{
				"b = ⎡32  32⎤",
				"    ⎣32  32⎦",
				"n = ⎡1  0⎤",
				"    ⎣0  1⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDenseScale(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_pow",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2, [
					4, 4,
					4, 4,
				])
				b = new m.Dense()
				b.scale(0.24, a)
				console.println(m.format(b, {
					format: "b = %2.1f", prefix: "    ", squeeze: true,
				}))
			`,
			Output: []string{
				"b = ⎡1.0  1.0⎤",
				"    ⎣1.0  1.0⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDenseInverse(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_inverse",
			Script: `const m = require("mathx/mat")
				a = new m.Dense(2, 2, [
					2, 1,
					6, 4,
				])
				aInv = new m.Dense()
				aInv.inverse(a)
				console.println(m.format(aInv, {
					format: "aInv = %.2g\n", prefix: "       ", squeeze: true,
				}))

				I = new m.Dense();
				I.mul(a, aInv)
				console.println(m.format(I, {
					format: "I = %.2g\n", prefix: "    ", squeeze: true,
				}))

				b = new m.Dense(2, 2, [
					2, 3,
					1, 2,
				])
				x = new m.Dense()
				x.solve(a, b)
				console.println(m.format(x, {
					format: "x = %.1f\n", prefix: "    ", squeeze: true,
				}))
			`,
			Output: []string{
				"aInv = ⎡ 2  -0.5⎤",
				"       ⎣-3     1⎦",
				"",
				"I = ⎡1  0⎤",
				"    ⎣0  1⎦",
				"",
				"x = ⎡ 3.5   5.0⎤",
				"    ⎣-5.0  -7.0⎦",
				"",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}
