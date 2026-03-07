package mat_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestFormat(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "format",
			Script: `const m = require("mathx/mat")
				A = new m.Dense(100, 100)
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
