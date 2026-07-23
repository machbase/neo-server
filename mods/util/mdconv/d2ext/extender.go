package d2ext

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"oss.terrastruct.com/d2/d2graph"
)

type Extender struct {
	Layout          d2graph.LayoutGraph
	ThemeID         *int64
	Sketch          bool
	OptionApplierFn FenceOptionApplier
}

func (e *Extender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&Transformer{}, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&HTMLRenderer{
			Layout:          e.Layout,
			ThemeID:         e.ThemeID,
			Sketch:          e.Sketch,
			OptionApplierFn: e.OptionApplierFn,
		}, 0),
	))
}
