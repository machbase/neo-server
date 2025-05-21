package mat_test

import "testing"

func TestVecDense(t *testing.T) {
	tests := []TestCase{
		{
			Name: "vector_dim_cap_len",
			Script: `
				const m = require("@jsh/mat")
				v = new m.VecDense(3, [4, 5, 6]) 
				console.log("dim:", v.dims().rows, v.dims().cols)
				console.log("cap:", v.cap().rows, v.cap().cols)
				console.log("len:", v.len())
				`,
			Expect: []string{
				"dim: 3 1",
				"cap: 3 1",
				"len: 3",
				"",
			},
		},
		{
			Name: "vector_at_set",
			Script: `
				const m = require("@jsh/mat")
				v = new m.VecDense(3, [4, 5, 6])
				at = v.at(2, 0)
				console.log("at(2,0):", at)
				atVec = v.atVec(0)
				console.log("atVec(0):", atVec)
				v.setVec(1, 2.0)
				console.log(m.format(v, {format: "v = %g", prefix: "    "}))
				T = v.T()
				console.log("T =", T.toString())
			`,
			Expect: []string{
				"at(2,0): 6",
				"atVec(0): 4",
				"v = ⎡4⎤",
				"    ⎢2⎥",
				"    ⎣6⎦",
				"T = [4  2  6]",
				"",
			},
		},
		{
			Name: "vec",
			Script: `
				const m = require("@jsh/mat")
				v = new m.VecDense(3, [1.0, 2.0, 3.0])
				console.log("dim:", v.dims().rows, v.dims().cols)
				console.log("cap:", v.cap().rows, v.cap().cols)
				console.log(m.format(v, {format: "v = %.2f", prefix: "    "}))
				v.scaleVec(2.0, v)
				console.log(m.format(v, {format: "v = %.2f", prefix: "    "}))
				v.setVec(0, -1.0)
				console.log(m.format(v, {format: "v = %.2f", prefix: "    "}))
			`,
			Expect: []string{
				"dim: 3 1",
				"cap: 3 1",
				"v = ⎡1.00⎤",
				"    ⎢2.00⎥",
				"    ⎣3.00⎦",
				"v = ⎡2.00⎤",
				"    ⎢4.00⎥",
				"    ⎣6.00⎦",
				"v = ⎡-1.00⎤",
				"    ⎢ 4.00⎥",
				"    ⎣ 6.00⎦",
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

func TestDenseMat(t *testing.T) {
	tests := []TestCase{
		{
			Name: "mat",
			Script: `
				const m = require("@jsh/mat")
				A = new m.Dense(4, 3, [
					5.0e4, 1.0e4, 6.0e4,
					2.0e4, 2.5e4, 1.0e3,
					5.0e4, 5.0e4, 1.5e3,
					1.5e5, 1.5e4, 0.0,
				])
				b = new m.VecDense(4, [1.1e6, 2.0e5, 5.0e5, 3.5e5])
				console.log(A.dims().rows, A.dims().cols)
				console.log(m.format(A, {
					format: "A = %.2f\n", prefix: "    ",
				}))
				console.log(b.dims().rows, b.dims().cols)
				console.log(m.format(b, {
					format: "b = %.2f\n", prefix: "    ",
				}))
				x = new m.VecDense()
				x.solveVec(A, b)
				console.log(m.format(x, {format: "x = %.2f\n", prefix: "    "}))

				prod = new m.VecDense()
				prod.mulVec(A, x)
				console.log(m.format(prod, {format: "A*b = %.2f", prefix: "      "}))
			`,
			Expect: []string{
				"4 3",
				"A = ⎡ 50000.00   10000.00   60000.00⎤",
				"    ⎢ 20000.00   25000.00    1000.00⎥",
				"    ⎢ 50000.00   50000.00    1500.00⎥",
				"    ⎣150000.00   15000.00       0.00⎦",
				"",
				"4 1",
				"b = ⎡1100000.00⎤",
				"    ⎢ 200000.00⎥",
				"    ⎢ 500000.00⎥",
				"    ⎣ 350000.00⎦",
				"",
				"x = ⎡ 1.59⎤",
				"    ⎢ 7.57⎥",
				"    ⎣15.75⎦",
				"",
				"A*b = ⎡1099857.08⎤",
				"      ⎢ 236649.35⎥",
				"      ⎢ 481283.99⎥",
				"      ⎣ 351399.73⎦",
				"",
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
