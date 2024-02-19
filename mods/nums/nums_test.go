package nums_test

import (
	"math"
	"testing"
	"time"

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

	ret = nums.LCM(80, 10, 100)
	require.Equal(t, 400, ret)
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

func TestLinspace(t *testing.T) {
	var ret, expect []float64
	ret = nums.Linspace(-2, 2, 5)
	expect = []float64{-2, -1, 0, 1, 2}
	require.Equal(t, 5, len(ret))
	for i := range ret {
		require.Equal(t, expect[i], ret[i])
	}

	ret = nums.Linspace50(-2, 2)
	require.Equal(t, 50, len(ret))
	require.Equal(t, -2.0, ret[0])
	require.Equal(t, 2.0, ret[49])

	ret = nums.Linspace(0, 1, 1)
	require.Equal(t, 1, len(ret))
	require.Equal(t, 0.0, ret[0])

	ret = nums.Linspace(0, 1, 0)
	require.Equal(t, 0, len(ret))

	ret = nums.Linspace(0, 1, -1)
	require.Equal(t, 0, len(ret))

	ret = nums.Linspace(1, 1, 1)
	require.Equal(t, 1, len(ret))

	ret = nums.Linspace(1, 1, 3)
	require.Equal(t, 3, len(ret))
}

func TestArrange(t *testing.T) {
	var ret, expect []float64

	ret = nums.Arrange(-2, 2, 1)
	expect = []float64{-2, -1, 0, 1, 2}
	require.Equal(t, 5, len(ret))
	for i := range ret {
		require.Equal(t, expect[i], ret[i])
	}

	ret = nums.Arrange(0, 1, 1)
	require.Equal(t, 2, len(ret))
	require.Equal(t, 0.0, ret[0])
	require.Equal(t, 1.0, ret[1])

	ret = nums.Arrange(0, 1, 0)
	require.Equal(t, 0, len(ret))

	ret = nums.Arrange(0, 1, -1)
	require.Equal(t, 0, len(ret))

	ret = nums.Arrange(1, 0, -0.3)
	require.Equal(t, 4, len(ret))
	require.Equal(t, 1.0, ret[0])

	ret = nums.Arrange(1, 1, 1)
	require.Equal(t, 0, len(ret))
}

func TestMeshgrid(t *testing.T) {
	var ret, expect [][][]float64
	ret = nums.Meshgrid(nums.Linspace(-2, 2, 5), nums.Linspace(-2, 2, 5))
	expect = [][][]float64{
		{{-2.0, -2.0}, []float64{-2.0, -1.0}, []float64{-2.0, 0}, []float64{-2.0, 1.0}, []float64{-2.0, 2.0}},
		{{-1.0, -2.0}, []float64{-1.0, -1.0}, []float64{-1.0, 0}, []float64{-1.0, 1.0}, []float64{-1.0, 2.0}},
		{{0, -2.0}, {0, -1.0}, {0, 0}, {0, 1.0}, {0, 2.0}},
		{{1.0, -2.0}, {1.0, -1.0}, {1.0, 0}, {1.0, 1.0}, {1.0, 2.0}},
		{{2.0, -2.0}, {2.0, -1.0}, {2.0, 0}, {2.0, 1.0}, {2.0, 2.0}},
	}
	require.Equal(t, 5, len(ret))
	for i := range ret {
		require.Equal(t, 5, len(ret[i]))
		for j := range ret[i] {
			require.Equal(t, 2, len(ret[i][j]))
			require.Equal(t, expect[i][j][0], ret[i][j][0])
			require.Equal(t, expect[i][j][1], ret[i][j][1])
		}
	}

	ret = nums.Meshgrid(nums.Linspace(-1, 0, -1), nums.Linspace(-2, 2, 1))
	require.Equal(t, 0, len(ret))

	ret = nums.Meshgrid(nums.Linspace(-2, 2, 1), nums.Linspace(-1, 0, -1))
	require.Equal(t, 1, len(ret))
	require.Equal(t, 0, len(ret[0]))
}

func TestFakeGen(t *testing.T) {
	n := 0.0
	gen := nums.NewFakeGenerator(func(ts time.Time) float64 {
		n += 1.0
		return n
	}, 5)

	for i := 0; i < 5; i++ {
		v := <-gen.C
		require.Equal(t, float64(i+1), v.V)
	}
	gen.Stop()
}

func TestArray(t *testing.T) {
	count, err := nums.Count(1, 2, 3)
	require.Nil(t, err)
	require.Equal(t, 3.0, count)

	val, err := nums.Element([]any{1.0, 2.0, 3.0, 1}...)
	require.Nil(t, err)
	require.Equal(t, 2.0, val)
}

func TestWeightedFloat64(t *testing.T) {
	sf := nums.WeightedFloat64Slice{
		nums.WeightedFloat64ValueWeight(1.23, 1),
		nums.WeightedFloat64ValueWeight(2.34, 2),
		nums.WeightedFloat64ValueWeight(0.12, 3),
	}
	sf.Sort()

	require.Equal(t, 0.12, sf[0].Value())
	require.Equal(t, 3.0, sf[0].Weight())
	require.Equal(t, 1.23, sf[1].Value())
	require.Equal(t, 1.0, sf[1].Weight())
	require.Equal(t, 2.34, sf[2].Value())
	require.Equal(t, 2.0, sf[2].Weight())
	require.EqualValues(t, []float64{0.12, 1.23, 2.34}, sf.Values())
	require.EqualValues(t, []float64{3.0, 1.0, 2.0}, sf.Weights())
}
