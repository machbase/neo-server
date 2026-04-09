package nums

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpheroidAccessors(t *testing.T) {
	s := spheroid{a: 6378137, fi: 298.257222101}
	require.Equal(t, 6378137.0, s.A())
	require.Equal(t, 298.257222101, s.Fi())
}

func TestEPSG5186Options(t *testing.T) {
	sp, ok := EPSG5186.Options["spheroid"].(spheroid)
	require.True(t, ok)
	require.Equal(t, 6378137.0, sp.A())
	require.Equal(t, 298.257222101, sp.Fi())

	area, ok := EPSG5186.Options["area"].(func(lon, lat float64) bool)
	require.True(t, ok)

	require.True(t, area(127.0, 37.5))
	require.False(t, area(122.0, 37.5))
	require.False(t, area(127.0, 41.0))
}

func TestTransformer5186(t *testing.T) {
	transform := Transformer5186()
	lon, lat, h := transform(200000, 600000, 123)

	require.False(t, math.IsNaN(lon))
	require.False(t, math.IsNaN(lat))
	require.InDelta(t, 123.0, h, 0.001)
	require.InDelta(t, 127.0, lon, 0.05)
	require.InDelta(t, 38.0, lat, 0.05)
}

func TestTransformer5179(t *testing.T) {
	transform := Transformer5179()
	lon, lat, h := transform(1000000, 2000000, 7)

	require.False(t, math.IsNaN(lon))
	require.False(t, math.IsNaN(lat))
	require.InDelta(t, 7.0, h, 0.001)
	require.InDelta(t, 127.5, lon, 0.05)
	require.InDelta(t, 38.0, lat, 0.05)
}
