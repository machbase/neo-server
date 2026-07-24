package geomapjs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapOptionsVarScript(t *testing.T) {
	t.Run("without tooltip", func(t *testing.T) {
		s := MapOptionsVarScript("mapA", false)
		require.Contains(t, s, "var mapA = {")
		require.Contains(t, s, "geojson:")
		require.Contains(t, s, "bindPopup")
		require.NotContains(t, s, "bindTooltip")
	})

	t.Run("with tooltip", func(t *testing.T) {
		s := MapOptionsVarScript("mapB", true)
		require.Contains(t, s, "var mapB = {")
		require.Contains(t, s, "geojson:")
		require.Contains(t, s, "bindPopup")
		require.Contains(t, s, "bindTooltip")
	})
}

func TestGeoJSONOptionsObjectLiteral(t *testing.T) {
	withoutTooltip := GeoJSONOptionsObjectLiteral(false)
	withTooltip := GeoJSONOptionsObjectLiteral(true)

	require.True(t, strings.HasPrefix(strings.TrimSpace(withoutTooltip), "{"))
	require.True(t, strings.HasPrefix(strings.TrimSpace(withTooltip), "{"))
	require.Contains(t, withoutTooltip, "pointToLayer")
	require.Contains(t, withTooltip, "pointToLayer")
	require.Contains(t, withoutTooltip, "bindPopup")
	require.Contains(t, withTooltip, "bindPopup")
	require.NotContains(t, withoutTooltip, "bindTooltip")
	require.Contains(t, withTooltip, "bindTooltip")
}
