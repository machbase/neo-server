package d2ext

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Transformer struct {
}

var _d2 = []byte("d2")

func (s *Transformer) Transform(doc *ast.Document, reader text.Reader, pctx parser.Context) {
	var blocks []*ast.FencedCodeBlock

	// Collect all blocks to be replaced without modifying the tree.
	ast.Walk(doc, func(node ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}

		cb, ok := node.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}

		lang := cb.Language(reader.Source())
		if !bytes.Equal(lang, _d2) {
			return ast.WalkContinue, nil
		}

		blocks = append(blocks, cb)
		return ast.WalkContinue, nil
	})

	// Nothing to do.
	if len(blocks) == 0 {
		return
	}

	for _, cb := range blocks {
		b := new(Block)
		b.SetLines(cb.Lines())

		parent := cb.Parent()
		if parent != nil {
			parent.ReplaceChild(parent, cb, b)
		}
	}
}
