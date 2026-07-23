package chartext

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFenceOptions(t *testing.T) {
	t.Run("parse chart options", func(t *testing.T) {
		opts, err := parseFenceOptions("chart {width=600px,height=400px,theme=dark,renderer=svg}")
		require.NoError(t, err)
		require.Equal(t, "600px", opts["width"])
		require.Equal(t, "400px", opts["height"])
		require.Equal(t, "dark", opts["theme"])
		require.Equal(t, "svg", opts["renderer"])
	})

	t.Run("old double brace syntax is ignored", func(t *testing.T) {
		opts, err := parseFenceOptions("chart {{opt1:true}}")
		require.NoError(t, err)
		require.Nil(t, opts)
	})

	t.Run("no meta", func(t *testing.T) {
		opts, err := parseFenceOptions("chart")
		require.NoError(t, err)
		require.Nil(t, opts)
	})

	t.Run("invalid meta", func(t *testing.T) {
		opts, err := parseFenceOptions("chart {width=100%")
		require.NoError(t, err)
		require.Nil(t, opts)
	})
}

func TestApplyFenceOptions(t *testing.T) {
	cfg := defaultRenderConfig(false)
	err := applyFenceOptions(&cfg, map[string]any{
		"width":    "600px",
		"height":   "320px",
		"theme":    "dark",
		"renderer": "svg",
	})
	require.NoError(t, err)
	require.Equal(t, "600px", cfg.Width)
	require.Equal(t, "320px", cfg.Height)
	require.Equal(t, "dark", cfg.Theme)
	require.Equal(t, "svg", cfg.Renderer)
}

func TestApplyFenceOptionsRejectsInvalidValue(t *testing.T) {
	cfg := defaultRenderConfig(false)
	err := applyFenceOptions(&cfg, map[string]any{"theme": "blue"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "light or dark")
}
