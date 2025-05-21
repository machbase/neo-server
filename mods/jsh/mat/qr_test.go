package mat_test

import "testing"

func TestQR(t *testing.T) {
	tests := []TestCase{
		{
			Name: "qr_factorize",
			Script: `const m = require("@jsh/mat")
				a = new m.Dense(4, 2, [0, 1, 1, 1, 1, 1, 2, 1])
				b = new m.Dense(4, 1, [1, 0, 2, 1])

				qr = new m.QR()
				qr.factorize(a)

				q = new m.Dense()
				qr.QTo(q)
				console.log(m.format(q, { format: "Q = %.3f", prefix: "    " }))

				qt = q.T()
				console.log(m.format(qt, { format: "Q^T  = %.3f", prefix: "       " }))
				qi = new m.Dense()
				qi.inverse(q)
				console.log(m.format(qi, { format: "Q^-1 = %.3f", prefix: "       " }))

				r = new m.Dense()
				qr.RTo(r)
				console.log(m.format(r, { format: "R = %.3f", prefix: "    " }))

				A = new m.Dense()
				A.mul(q, r)
				console.log(m.format(A, { format: "A = %.3f", prefix: "    " }))

				x = new m.Dense()
				qr.solveTo(x, false, b)
				console.log(m.format(x, { format: "x = %.3f", prefix: "    " }))
			`,
			Expect: []string{
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
