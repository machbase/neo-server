package simplex_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestSimplex(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "simplex",
			Script: `
				const simplex = require("mathx/simplex");
				const gen = new simplex.Simplex(123);
				for(i=0; i < 5; i++) {
					console.println(i, gen.noise(i, i * 0.6).toFixed(3));
				}
			`,
			Output: []string{
				"0 0.000",
				"1 0.349",
				"2 0.319",
				"3 0.038",
				"4 -0.364",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}
