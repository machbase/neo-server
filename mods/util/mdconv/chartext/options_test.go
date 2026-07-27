package chartext

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFenceOptions(t *testing.T) {
	t.Run("parse chart options", func(t *testing.T) {
		opts, err := parseFenceOptions("chart {width=600px,height=400px,theme=dark,renderer=svg,loader=auto,plugins=gl,echartsSrc=/web/echarts/echarts.min.js}")
		require.NoError(t, err)
		require.Equal(t, "600px", opts["width"])
		require.Equal(t, "400px", opts["height"])
		require.Equal(t, "dark", opts["theme"])
		require.Equal(t, "svg", opts["renderer"])
		require.Equal(t, "auto", opts["loader"])
		require.Equal(t, "gl", opts["plugins"])
		require.Equal(t, "/web/echarts/echarts.min.js", opts["echartsSrc"])
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
		"theme":    "roma",
		"renderer": "svg",
		"loader":   "local",
		"plugins":  []any{"gl", "wordcloud", "gl"},
	})
	require.NoError(t, err)
	require.Equal(t, "600px", cfg.Width)
	require.Equal(t, "320px", cfg.Height)
	require.Equal(t, "roma", cfg.Theme)
	require.Equal(t, []string{"/web/echarts/themes/roma.js"}, cfg.ThemeSrcs)
	require.Equal(t, "svg", cfg.Renderer)
	require.Equal(t, "local", cfg.Loader)
	require.Equal(t, []string{"/web/echarts/echarts-gl.min.js", "/web/echarts/echarts-wordcloud.min.js"}, cfg.PluginSrcs)
}

func TestApplyFenceOptionsAcceptsImplicitStringConversion(t *testing.T) {
	cfg := defaultRenderConfig(false)
	err := applyFenceOptions(&cfg, map[string]any{
		"width":   int64(400),
		"height":  400,
		"plugins": []any{"gl", int64(123)},
	})
	require.NoError(t, err)
	require.Equal(t, "400", cfg.Width)
	require.Equal(t, "400", cfg.Height)
	require.Equal(t, []string{"/web/echarts/echarts-gl.min.js", "123"}, cfg.PluginSrcs)
}

func TestApplyFenceOptionsRejectsInvalidValue(t *testing.T) {
	cfg := defaultRenderConfig(false)
	err := applyFenceOptions(&cfg, map[string]any{"theme": "blue"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "chart theme must be one of")
}

func TestApplyFenceOptionsAcceptsLightThemeAlias(t *testing.T) {
	cfg := defaultRenderConfig(false)
	err := applyFenceOptions(&cfg, map[string]any{"theme": "light"})
	require.NoError(t, err)
	require.Equal(t, "white", cfg.Theme)
	require.Empty(t, cfg.ThemeSrcs)
}

func TestApplyFenceOptionsRejectsInvalidLoader(t *testing.T) {
	cfg := defaultRenderConfig(false)
	err := applyFenceOptions(&cfg, map[string]any{"loader": "bad"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "none, local or auto")
}
