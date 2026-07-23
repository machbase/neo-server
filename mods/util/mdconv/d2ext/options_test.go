package d2ext

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2plugin"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
)

func TestParseFenceOptions(t *testing.T) {
	t.Run("parse hugo style options", func(t *testing.T) {
		opts, err := parseFenceOptions("d2 {layout=elk,linenos=table,linenostart=1,hl_lines=[2,3,\"10-20\"],include_space=\"a b\"}")
		require.NoError(t, err)
		require.Equal(t, "elk", opts["layout"])
		require.Equal(t, "table", opts["linenos"])
		require.Equal(t, int64(1), opts["linenostart"])
		require.Equal(t, "a b", opts["include_space"])

		hl, ok := opts["hl_lines"].([]any)
		require.True(t, ok)
		require.Len(t, hl, 3)
		require.Equal(t, int64(2), hl[0])
		require.Equal(t, int64(3), hl[1])
		require.Equal(t, "10-20", hl[2])
	})

	t.Run("old double brace syntax is ignored", func(t *testing.T) {
		opts, err := parseFenceOptions("d2 {{opt1:true}}")
		require.NoError(t, err)
		require.Nil(t, opts)
	})

	t.Run("no meta", func(t *testing.T) {
		opts, err := parseFenceOptions("d2")
		require.NoError(t, err)
		require.Nil(t, opts)
	})

	t.Run("invalid meta", func(t *testing.T) {
		opts, err := parseFenceOptions("d2 {opt1=true")
		require.NoError(t, err)
		require.Nil(t, opts)
	})
}

func TestApplyDefaultFenceOptions(t *testing.T) {
	renderOpts := &d2svg.RenderOpts{}
	err := applyDefaultFenceOptions(map[string]any{
		"themeID": "7",
		"sketch":  true,
		"pad":     32,
		"scale":   1.5,
	}, nil, renderOpts)
	require.NoError(t, err)
	require.NotNil(t, renderOpts.ThemeID)
	require.Equal(t, int64(7), *renderOpts.ThemeID)
	require.NotNil(t, renderOpts.Sketch)
	require.Equal(t, true, *renderOpts.Sketch)
	require.NotNil(t, renderOpts.Pad)
	require.Equal(t, int64(32), *renderOpts.Pad)
	require.NotNil(t, renderOpts.Scale)
	require.Equal(t, 1.5, *renderOpts.Scale)
}

func TestApplyDefaultFenceOptionsWithThemeName(t *testing.T) {
	renderOpts := &d2svg.RenderOpts{}
	err := applyDefaultFenceOptions(map[string]any{
		"theme":  "Cool Classics",
		"sketch": false,
	}, nil, renderOpts)
	require.NoError(t, err)
	require.NotNil(t, renderOpts.ThemeID)
	require.Equal(t, d2themescatalog.CoolClassics.ID, *renderOpts.ThemeID)
}

func TestApplyDefaultFenceOptionsWithLayoutOverride(t *testing.T) {
	compileOpts := &d2lib.CompileOptions{}
	renderOpts := &d2svg.RenderOpts{}
	err := applyDefaultFenceOptionsWithCompile(map[string]any{
		"layout":  "dagre",
		"nodesep": int64(120),
		"edgesep": int64(45),
	}, compileOpts, renderOpts)
	require.NoError(t, err)
	require.NotNil(t, compileOpts.Layout)
	require.Equal(t, "dagre", *compileOpts.Layout)
	require.NotNil(t, compileOpts.LayoutResolver)

	layout, err := compileOpts.LayoutResolver("dagre")
	require.NoError(t, err)
	require.NotNil(t, layout)
}

func TestApplyDefaultFenceOptionsWithLayoutOnly(t *testing.T) {
	compileOpts := &d2lib.CompileOptions{}
	renderOpts := &d2svg.RenderOpts{}
	err := applyDefaultFenceOptionsWithCompile(map[string]any{
		"layout": "elk",
	}, compileOpts, renderOpts)
	require.NoError(t, err)
	require.NotNil(t, compileOpts.Layout)
	require.Equal(t, "elk", *compileOpts.Layout)
}

func TestApplyDefaultFenceOptionsRejectsNodeSepWithNonDagreLayout(t *testing.T) {
	compileOpts := &d2lib.CompileOptions{}
	renderOpts := &d2svg.RenderOpts{}
	err := applyDefaultFenceOptionsWithCompile(map[string]any{
		"layout":  "elk",
		"nodesep": int64(120),
	}, compileOpts, renderOpts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "only supported when layout is dagre")
}

func TestApplyDefaultFenceOptionsWithElkAlgorithm(t *testing.T) {
	compileOpts := &d2lib.CompileOptions{}
	renderOpts := &d2svg.RenderOpts{}
	err := applyDefaultFenceOptionsWithCompile(map[string]any{
		"layout":    "elk",
		"algorithm": "mrtree",
	}, compileOpts, renderOpts)
	require.NoError(t, err)
	require.NotNil(t, compileOpts.Layout)
	require.Equal(t, "elk", *compileOpts.Layout)
	require.NotNil(t, compileOpts.LayoutResolver)

	layout, err := compileOpts.LayoutResolver("elk")
	require.NoError(t, err)
	require.NotNil(t, layout)
}

func TestApplyDefaultFenceOptionsRejectsAlgorithmWithNonElkLayout(t *testing.T) {
	compileOpts := &d2lib.CompileOptions{}
	renderOpts := &d2svg.RenderOpts{}
	err := applyDefaultFenceOptionsWithCompile(map[string]any{
		"layout":    "dagre",
		"algorithm": "layered",
	}, compileOpts, renderOpts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "algorithm is only supported when layout is elk")
}

func TestApplyDefaultFenceOptionsRejectsUnsupportedElkAlgorithm(t *testing.T) {
	compileOpts := &d2lib.CompileOptions{}
	renderOpts := &d2svg.RenderOpts{}
	err := applyDefaultFenceOptionsWithCompile(map[string]any{
		"layout":    "elk",
		"algorithm": "spore",
	}, compileOpts, renderOpts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported elk algorithm: spore")
	require.Contains(t, err.Error(), "supported: layered, mrtree, random")
}

func TestDefaultLayoutResolverUnknownLayoutMessage(t *testing.T) {
	d2PluginListOnce = sync.Once{}
	d2PluginListErr = nil
	d2PluginList = []d2plugin.Plugin{&stubPlugin{name: "dagre"}, &stubPlugin{name: "elk"}}

	r := &HTMLRenderer{}
	_, err := r.defaultLayoutResolver("not-exists")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown layout engine: not-exists")
	require.Contains(t, err.Error(), "available: dagre, elk")
}

type stubPlugin struct {
	name string
}

func (p *stubPlugin) Info(context.Context) (*d2plugin.PluginInfo, error) {
	return &d2plugin.PluginInfo{Name: p.name}, nil
}

func (p *stubPlugin) Flags(context.Context) ([]d2plugin.PluginSpecificFlag, error) {
	return nil, nil
}

func (p *stubPlugin) HydrateOpts([]byte) error {
	return nil
}

func (p *stubPlugin) Layout(context.Context, *d2graph.Graph) error {
	return nil
}

func (p *stubPlugin) PostProcess(context.Context, []byte) ([]byte, error) {
	return nil, nil
}
