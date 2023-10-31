package util_test

import (
	"testing"

	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

type Value struct {
	StrValue  string `default:"string"`
	FlagValue bool   `default:"true"`
	IntValue  int    `default:"123"`
}

func TestSetDefault(t *testing.T) {
	v := Value{}
	util.SetDefaultValue(&v)

	require.Equal(t, "string", v.StrValue)
	require.Equal(t, true, v.FlagValue)
	require.Equal(t, 123, v.IntValue)
}
