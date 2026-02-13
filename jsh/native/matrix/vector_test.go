package matrix

import "testing"

func TestVecDense(t *testing.T) {
	tests := []TestCase{
		{
			name: "vector_dim_cap_len",
			script: `
				const m = require("matrix")
				v = new m.VecDense(3, [4, 5, 6]) 
				console.println("dim:", v.dims().rows, v.dims().cols)
				console.println("cap:", v.cap())
				console.println("len:", v.len())
				`,
			output: []string{
				"dim: 3 1",
				"cap: 3",
				"len: 3",
			},
		},
		{
			name: "vector_at_set",
			script: `
				const m = require("matrix")
				v = new m.VecDense(3, [4, 5, 6])
				at = v.at(2, 0)
				console.println("at(2,0):", at)
				atVec = v.atVec(0)
				console.println("atVec(0):", atVec)
				v.setVec(1, 2.0)
				console.println(m.format(v, {format: "v = %g", prefix: "    "}))
				T = v.T()
				console.println("T =", T.toString())
			`,
			output: []string{
				"at(2,0): 6",
				"atVec(0): 4",
				"v = ⎡4⎤",
				"    ⎢2⎥",
				"    ⎣6⎦",
				"T = [4  2  6]",
			},
		},
		{
			name: "vec",
			script: `
				const m = require("matrix")
				v = new m.VecDense(3, [1.0, 2.0, 3.0])
				console.println("dim:", v.dims().rows, v.dims().cols)
				console.println("cap:", v.cap())
				console.println(m.format(v, {format: "v = %.2f", prefix: "    "}))
				v.scaleVec(2.0, v)
				console.println(m.format(v, {format: "v = %.2f", prefix: "    "}))
				v.setVec(0, -1.0)
				console.println(m.format(v, {format: "v = %.2f", prefix: "    "}))
			`,
			output: []string{
				"dim: 3 1",
				"cap: 3",
				"v = ⎡1.00⎤",
				"    ⎢2.00⎥",
				"    ⎣3.00⎦",
				"v = ⎡2.00⎤",
				"    ⎢4.00⎥",
				"    ⎣6.00⎦",
				"v = ⎡-1.00⎤",
				"    ⎢ 4.00⎥",
				"    ⎣ 6.00⎦",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}

func TestDenseMat(t *testing.T) {
	tests := []TestCase{
		{
			name: "mat",
			script: `
				const m = require("matrix")
				A = new m.Dense(4, 3, [
					5.0e4, 1.0e4, 6.0e4,
					2.0e4, 2.5e4, 1.0e3,
					5.0e4, 5.0e4, 1.5e3,
					1.5e5, 1.5e4, 0.0,
				])
				b = new m.VecDense(4, [1.1e6, 2.0e5, 5.0e5, 3.5e5])
				console.println(A.dims().rows, A.dims().cols)
				console.println(m.format(A, {
					format: "A = %.2f\n", prefix: "    ",
				}))
				console.println(b.dims().rows, b.dims().cols)
				console.println(m.format(b, {
					format: "b = %.2f\n", prefix: "    ",
				}))
				x = new m.VecDense()
				x.solveVec(A, b)
				console.println(m.format(x, {format: "x = %.2f\n", prefix: "    "}))

				prod = new m.VecDense()
				prod.mulVec(A, x)
				console.println(m.format(prod, {format: "A*b = %.2f", prefix: "      "}))
			`,
			output: []string{
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
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}
