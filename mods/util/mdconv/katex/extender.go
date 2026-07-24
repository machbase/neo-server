package katex

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type RenderOptions struct {
	ThrowOnError       bool
	InlineWrapperClass string
	BlockWrapperClass  string
	Output             string
	Leqno              bool
	Fleqn              bool
}

type Extender struct {
	Options RenderOptions
}

func (e *Extender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&Transformer{}, 100),
	))
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(&InlineParser{}, 150),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&HTMLRenderer{Options: e.Options}, 0),
	))
}
