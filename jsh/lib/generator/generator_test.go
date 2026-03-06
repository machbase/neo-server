package generator_test

import (
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/stretchr/testify/require"
)

func TestSimplex(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "js-simplex",
			Script: `
				const {Simplex} = require("generator")
				simplex = new Simplex(123);
				for(i=0; i < 5; i++) {
					console.println(i, simplex.eval(i, i * 0.6).toFixed(3));
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

func TestUUID(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "js-uuid",
			Script: `
				const {UUID} = require("generator")
				gen = new UUID(1);
				for(i=0; i < 5; i++) {
					console.println(gen.eval());
				}
			`,
			ExpectFunc: func(t *testing.T, result string) {
				rows := strings.Split(strings.TrimSpace(result), "\n")
				require.Equal(t, 5, len(rows), result)
				for _, l := range rows {
					require.Equal(t, 36, len(l))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestMeshgrid(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "js-meshgrid",
			Script: `
				gen = require("generator").meshgrid([1, 2, 3], [4, 5]);
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
