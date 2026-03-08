package mat_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

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
