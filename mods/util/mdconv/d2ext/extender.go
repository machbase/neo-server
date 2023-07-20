package d2ext

import (
	"context"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"oss.terrastruct.com/d2/d2graph"
)

type Extender struct {
	Layout  func(context.Context, *d2graph.Graph) error
	ThemeID int64
	Sketch  bool
}

func (e *Extender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&Transformer{}, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&HTMLRenderer{
			Layout:  e.Layout,
			ThemeID: e.ThemeID,
			Sketch:  e.Sketch,
		}, 0),
	))
}
