package uuid_test

import (
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/stretchr/testify/require"
)

func TestSimplex(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "uuid",
			Script: `
				const {UUID} = require("uuid")
				uuid = new UUID();
				for(i=0; i < 5; i++) {
					console.println(uuid.newV1());
				}
			`,
			ExpectFunc: func(t *testing.T, result string) {
				rows := strings.Split(strings.TrimSpace(result), "\n")
				require.Equal(t, 5, len(rows), result)
				for _, l := range rows {
					require.Equal(t, 36, len(l), l)
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
