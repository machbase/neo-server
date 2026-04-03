package util_test

import (
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/stretchr/testify/require"
)

func TestGetTimeformat(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "named format is case insensitive", input: "rfc3339", expect: "2006-01-02T15:04:05Z07:00"},
		{name: "punctuated default alias", input: "default.us", expect: "2006-01-02 15:04:05.000000"},
		{name: "unknown format passes through", input: "2006/01/02", expect: "2006/01/02"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expect, util.GetTimeformat(tc.input))
		})
	}
}

func TestHelpTimeformats(t *testing.T) {
	help := util.HelpTimeformats()

	require.True(t, strings.HasPrefix(help, "    epoch\n"))
	require.Contains(t, help, "RFC3339        2006-01-02T15:04:05Z07:00")
	require.Contains(t, help, "custom format")
	require.Contains(t, help, "second         05 or with sub-seconds '05.999999'")
}
