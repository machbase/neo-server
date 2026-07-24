package geomapext

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFenceOptions(t *testing.T) {
	t.Run("parse geomap options", func(t *testing.T) {
		opts, err := parseFenceOptions("geomap {width=600px,height=420px,tile=default,fit=bounds,center=[37.5,127.0],zoom=11,grayscale=0.4,loader=auto}")
		require.NoError(t, err)
		require.Equal(t, "600px", opts["width"])
		require.Equal(t, "420px", opts["height"])
		require.Equal(t, "default", opts["tile"])
		require.Equal(t, "bounds", opts["fit"])
		require.Equal(t, int64(11), opts["zoom"])
		require.Equal(t, 0.4, opts["grayscale"])
	})

	t.Run("old double brace syntax is ignored", func(t *testing.T) {
		opts, err := parseFenceOptions("geomap {{opt:true}}")
		require.NoError(t, err)
		require.Nil(t, opts)
	})

	t.Run("no meta", func(t *testing.T) {
		opts, err := parseFenceOptions("geomap")
		require.NoError(t, err)
		require.Nil(t, opts)
	})
}

func TestApplyFenceOptions(t *testing.T) {
	cfg := defaultRenderConfig(false)
	err := applyFenceOptions(&cfg, map[string]any{
		"width":     "640px",
		"height":    "360px",
		"tile":      "https://{s}.example.com/tiles/{z}/{x}/{y}.png",
		"fit":       "center",
		"center":    []any{37.5, 127.0},
		"zoom":      int64(10),
		"grayscale": 0.5,
		"loader":    "local",
	})
	require.NoError(t, err)
	require.Equal(t, "640px", cfg.Width)
	require.Equal(t, "360px", cfg.Height)
	require.Equal(t, "https://{s}.example.com/tiles/{z}/{x}/{y}.png", cfg.Tile)
	require.Equal(t, "center", cfg.Fit)
	require.Equal(t, [2]float64{37.5, 127.0}, cfg.Center)
	require.Equal(t, 10, cfg.Zoom)
	require.Equal(t, 0.5, cfg.Grayscale)
	require.Equal(t, "local", cfg.Loader)
}

func TestApplyFenceOptionsRejectsInvalidTile(t *testing.T) {
	cfg := defaultRenderConfig(false)
	err := applyFenceOptions(&cfg, map[string]any{"tile": "https://example.com/no-placeholders"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "default or a URL template")
}

func TestApplyFenceOptionsRejectsInvalidPayloadOption(t *testing.T) {
	cfg := defaultRenderConfig(false)
	err := applyFenceOptions(&cfg, map[string]any{"center": []any{37.5}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "center")
}
