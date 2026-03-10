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
				a = m.Dense(2, 2)
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
				a = m.Dense(2, 2, [
					1, 0,
					1, 0,
				])
				b = m.Dense(2, 2, [
					0, 1,
					0, 1,
				])
				c = m.Dense(2, 2)
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
				a = m.Dense(2, 2, [
					1, 1,
					1, 1,
				])
				b = m.Dense(2, 2, [
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
				a = m.Dense(2, 2, [
					1, 2,
					3, 4,
				])
				b = m.Dense(2, 2, [
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
				a = m.Dense(2, 2, [
					4, 0,
					0, 4,
				])
				b = m.Dense(2, 3, [
					4, 0, 0,
					0, 0, 4,
				])
				c = m.Dense()
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
				A = m.Dense(2, 2, [
					1, 2,
					3, 4,
				])
				b = new m.VecDense(2, [
					2,
					2,
				])
				C = m.Dense()
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
				a = m.Dense(2, 2, [
					5, 10,
					15, 20,
				])
				b = m.Dense(2, 2, [
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
				a = m.Dense(2, 2, [
					1, 0,
					0, 1,
				])
				b = m.Dense()
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
				a = m.Dense(2, 2, [
					4, 4,
					4, 4,
				])
				b = m.Dense()
				b.pow(a, 2)
				console.println(m.format(b, {
					format: "b = %v", prefix: "    ", squeeze: true,
				}))
				n = m.Dense()
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
				a = m.Dense(2, 2, [
					4, 4,
					4, 4,
				])
				b = m.Dense()
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
				a = m.Dense(2, 2, [
					2, 1,
					6, 4,
				])
				aInv = m.Dense()
				aInv.inverse(a)
				console.println(m.format(aInv, {
					format: "aInv = %.2g\n", prefix: "       ", squeeze: true,
				}))

				I = m.Dense();
				I.mul(a, aInv)
				console.println(m.format(I, {
					format: "I = %.2g\n", prefix: "    ", squeeze: true,
				}))

				b = m.Dense(2, 2, [
					2, 3,
					1, 2,
				])
				x = m.Dense()
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

func TestSymDenseSet(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dense_set",
			Script: `const m = require("mathx/mat")
				n = 5;
				a = new m.SymDense(n, null);
				count = 1.0;
				for(let i = 0; i < n; i++) {
					for(let j = i; j < n; j++) {
						a.setSym(i, j, count);
						count++;
					}
				}
				console.println(m.format(a, {
					format: "a = %v\n", prefix: "    ", squeeze: true,
				}))

				var sub = new m.SymDense();
				sub.subsetSym(a, [0, 2, 4]);
				console.println(m.format(sub, {
					format: "subset: [0, 2, 4]\n%v\n", squeeze: true,
				}))
				sub.subsetSym(a, [0, 0, 4]);
				console.println(m.format(sub, {
					format: "subset: [0, 0, 4]\n%v", squeeze: true,
				}))
			`,
			Output: []string{
				"a = ⎡1  2   3   4   5⎤",
				"    ⎢2  6   7   8   9⎥",
				"    ⎢3  7  10  11  12⎥",
				"    ⎢4  8  11  13  14⎥",
				"    ⎣5  9  12  14  15⎦",
				"",
				"subset: [0, 2, 4]",
				"⎡1   3   5⎤",
				"⎢3  10  12⎥",
				"⎣5  12  15⎦",
				"",
				"subset: [0, 0, 4]",
				"⎡1  1   5⎤",
				"⎢1  1   5⎥",
				"⎣5  5  15⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestVecDense(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "vector_dim_cap_len",
			Script: `
				const m = require("mathx/mat")
				v = m.VecDense(3, [4, 5, 6]) 
				console.println("dim:", v.dims())
				console.println("cap:", v.cap())
				console.println("len:", v.len())
				`,
			Output: []string{
				"dim: [3, 1]",
				"cap: 3",
				"len: 3",
			},
		},
		{
			Name: "vector_at_set",
			Script: `
				const m = require("mathx/mat")
				v = m.VecDense(3, [4, 5, 6])
				at = v.at(2, 0)
				console.println("at(2,0):", at)
				atVec = v.atVec(0)
				console.println("atVec(0):", atVec)
				v.setVec(1, 2.0)
				console.println(m.format(v, {format: "v = %g", prefix: "    "}))
				T = v.t()
				console.println(m.format(T, {format: "T = %g", prefix: "    "}))
			`,
			Output: []string{
				"at(2,0): 6",
				"atVec(0): 4",
				"v = ⎡4⎤",
				"    ⎢2⎥",
				"    ⎣6⎦",
				"T = [4  2  6]",
			},
		},
		{
			Name: "vec",
			Script: `
				const m = require("mathx/mat")
				v = m.VecDense(3, [1.0, 2.0, 3.0])
				console.println("dim:", v.dims())
				console.println("cap:", v.cap())
				console.println(m.format(v, {format: "v = %.2f", prefix: "    "}))
				v.scaleVec(2.0, v)
				console.println(m.format(v, {format: "v = %.2f", prefix: "    "}))
				v.setVec(0, -1.0)
				console.println(m.format(v, {format: "v = %.2f", prefix: "    "}))
			`,
			Output: []string{
				"dim: [3, 1]",
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
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestDenseMat(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "mat",
			Script: `
				const m = require("mathx/mat")
				A = m.Dense(4, 3, [
					5.0e4, 1.0e4, 6.0e4,
					2.0e4, 2.5e4, 1.0e3,
					5.0e4, 5.0e4, 1.5e3,
					1.5e5, 1.5e4, 0.0,
				])
				b = m.VecDense(4, [1.1e6, 2.0e5, 5.0e5, 3.5e5])
				console.println("dims:", A.dims())
				console.println(m.format(A, {
					format: "A = %.2f\n", prefix: "    ",
				}))
				console.println("dims:", b.dims())
				console.println(m.format(b, {
					format: "b = %.2f\n", prefix: "    ",
				}))
				x = m.VecDense()
				x.solveVec(A, b)
				console.println(m.format(x, {format: "x = %.2f\n", prefix: "    "}))

				prod = m.VecDense()
				prod.mulVec(A, x)
				console.println(m.format(prod, {format: "A*b = %.2f", prefix: "      "}))
			`,
			Output: []string{
				"dims: [4, 3]",
				"A = ⎡ 50000.00   10000.00   60000.00⎤",
				"    ⎢ 20000.00   25000.00    1000.00⎥",
				"    ⎢ 50000.00   50000.00    1500.00⎥",
				"    ⎣150000.00   15000.00       0.00⎦",
				"",
				"dims: [4, 1]",
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
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestQR(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "qr_factorize",
			Script: `const m = require("mathx/mat")
				a = m.Dense(4, 2, [0, 1, 1, 1, 1, 1, 2, 1])
				b = m.Dense(4, 1, [1, 0, 2, 1])

				qr = m.QR()
				qr.factorize(a)

				q = m.Dense()
				qr.qTo(q)
				console.println(m.format(q, { format: "Q = %.3f", prefix: "    " }))

				qt = q.t()
				console.println(m.format(qt, { format: "Q^T  = %.3f", prefix: "       " }))
				qi = m.Dense()
				qi.inverse(q)
				console.println(m.format(qi, { format: "Q^-1 = %.3f", prefix: "       " }))

				r = m.Dense()
				qr.rTo(r)
				console.println(m.format(r, { format: "R = %.3f", prefix: "    " }))

				A = m.Dense()
				A.mul(q, r)
				console.println(m.format(A, { format: "A = %.3f", prefix: "    " }))

				x = m.Dense()
				qr.solveTo(x, false, b)
				console.println(m.format(x, { format: "x = %.3f", prefix: "    " }))
			`,
			Output: []string{
				"Q = ⎡ 0.000   0.866  -0.331   0.375⎤",
				"    ⎢-0.408   0.289  -0.200  -0.843⎥",
				"    ⎢-0.408   0.289   0.861   0.092⎥",
				"    ⎣-0.816  -0.289  -0.331   0.375⎦",
				"Q^T  = ⎡ 0.000  -0.408  -0.408  -0.816⎤",
				"       ⎢ 0.866   0.289   0.289  -0.289⎥",
				"       ⎢-0.331  -0.200   0.861  -0.331⎥",
				"       ⎣ 0.375  -0.843   0.092   0.375⎦",
				"Q^-1 = ⎡ 0.000  -0.408  -0.408  -0.816⎤",
				"       ⎢ 0.866   0.289   0.289  -0.289⎥",
				"       ⎢-0.331  -0.200   0.861  -0.331⎥",
				"       ⎣ 0.375  -0.843   0.092   0.375⎦",
				"R = ⎡-2.449  -1.633⎤",
				"    ⎢ 0.000   1.155⎥",
				"    ⎢ 0.000   0.000⎥",
				"    ⎣ 0.000   0.000⎦",
				"A = ⎡0.000  1.000⎤",
				"    ⎢1.000  1.000⎥",
				"    ⎢1.000  1.000⎥",
				"    ⎣2.000  1.000⎦",
				"x = ⎡0.000⎤",
				"    ⎣1.000⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "format",
			Script: `const m = require("mathx/mat")
				A = m.Dense(100, 100)
				for (let i = 0; i < 100; i++) {
					for (let j = 0; j < 100; j++) {
						A.set(i, j, i + j)
					}
				}
				console.println(m.format(A, {
					format: "A = %v",
					prefix: "    ",
					squeeze: true,
					excerpt: 3,
				}))
			`,
			Output: []string{
				"A = Dims(100, 100)",
				"    ⎡ 0    1    2  ...  ...   97   98   99⎤",
				"    ⎢ 1    2    3             98   99  100⎥",
				"    ⎢ 2    3    4             99  100  101⎥",
				"     .",
				"     .",
				"     .",
				"    ⎢97   98   99            194  195  196⎥",
				"    ⎢98   99  100            195  196  197⎥",
				"    ⎣99  100  101  ...  ...  196  197  198⎦",
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
			Name: "dense-invalid-data",
			Script: `const m = require("mathx/mat")
				try {
					new m.Dense(2, 2, [1, "bad", 3, 4])
				} catch (e) {
					console.println(e.message)
				}`,
			Output: []string{
				"Dense: data should contain only numbers",
			},
		},
		{
			Name: "symdense-invalid-length",
			Script: `const m = require("mathx/mat")
				try {
					new m.SymDense(2, [1, 2, 3])
				} catch (e) {
					console.println(e.message)
				}`,
			Output: []string{
				"SymDense: data length should be 4",
			},
		},
		{
			Name: "vecdense-invalid-size",
			Script: `const m = require("mathx/mat")
				try {
					new m.VecDense(-1, [1])
				} catch (e) {
					console.println(e.message)
				}`,
			Output: []string{
				"VecDense: size should be non-negative",
			},
		},
		{
			Name: "format-invalid-options",
			Script: `const m = require("mathx/mat")
				A = new m.Dense(1, 1, [1])
				try {
					m.format(A, { excerpt: "bad" })
				} catch (e) {
					console.println(e.message)
				}`,
			Output: []string{
				"format: invalid options",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}
