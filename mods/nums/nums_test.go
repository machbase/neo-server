package nums_test

import (
	"math"
	"testing"

	"github.com/machbase/neo-server/mods/nums"
	"github.com/stretchr/testify/require"
)

func TestGCD(t *testing.T) {
	ret := nums.GCD(8, 10)
	require.Equal(t, 2, ret)
}

func TestLCM(t *testing.T) {
	ret := nums.LCM(8, 10)
	require.Equal(t, 40, ret)
}

func TestRound(t *testing.T) {
	ret := nums.Round(12, 10)
	require.Equal(t, 10.0, ret)
	ret = nums.Round(12, 0)
	require.True(t, math.IsNaN(ret))
}

func TestMod(t *testing.T) {
	ret := nums.Mod(12.1, 3.0)
	mod := float64(12.1) / float64(3.0)
	require.Equal(t, 12.1-float64(int(mod)*3), ret)
}
