package semver_test

import (
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/stretchr/testify/require"
)

func TestSimplex(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "semver",
			Script: `
				const semver = require("semver")
				console.println(semver.satisfies("1.4.2", "1.2 - 1.4"));
				console.println(semver.satisfies("2.0.0", "1.2 - 1.4"));
				console.println(semver.maxSatisfying(["1.2.0", "1.4.2", "2.0.0"], "1.2 - 1.4"));
				console.println(semver.maxSatisfying(["1.0.0", "1.1.4", "1.2.0"], "~1.1"));
				console.println(semver.compare("1.1.0", "1.2.0"));
				console.println(semver.compare("1.2.0", "1.1.0"));
				console.println(semver.compare("1.2.0", "1.2.0"));
			`,
			ExpectFunc: func(t *testing.T, result string) {
				rows := strings.Split(strings.TrimSpace(result), "\n")
				require.Equal(t, []string{
					"true",
					"false",
					"1.4.2",
					"1.1.4",
					"-1",
					"1",
					"0",
				}, rows)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}
