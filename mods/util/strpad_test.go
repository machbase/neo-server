package util_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/stretchr/testify/require"
)

func TestStrPad(t *testing.T) {
	input := "Codes"
	ret := util.StrPad(input, 10, " ", "RIGHT")
	require.Equal(t, "Codes     ", ret)
	ret = util.StrPad(input, 10, "-=", "LEFT")
	require.Equal(t, "=-=-=Codes", ret)
	ret = util.StrPad(input, 10, "_", "BOTH")
	require.Equal(t, "__Codes___", ret)
	ret = util.StrPad(input, 6, "___", "RIGHT")
	require.Equal(t, "Codes_", ret)
	ret = util.StrPad(input, 3, "*", "RIGHT")
	require.Equal(t, "Codes", ret)

}
