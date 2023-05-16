package util_test

import (
	"testing"

	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

func TestSplitFields(t *testing.T) {
	testSplitFields(t, true,
		`--data "C:\Users\user\work\neo-download\neo 0.1.2\machbase_home"`,
		[]string{"--data", `C:\Users\user\work\neo-download\neo 0.1.2\machbase_home`})
	testSplitFields(t, false,
		`--data "C:\Users\user\work\neo-download\neo 0.1.2\machbase_home"`,
		[]string{"--data", `"C:\Users\user\work\neo-download\neo 0.1.2\machbase_home"`})
}

func testSplitFields(t *testing.T, stripQutes bool, args string, expects []string) {
	toks := util.SplitFields(args, stripQutes)
	require.Equal(t, len(expects), len(toks))
	for i, tok := range toks {
		require.Equal(t, expects[i], tok)
	}
}
