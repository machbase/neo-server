package geomapext

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Transformer struct{}

var geomapLang = []byte("geomap")

func (s *Transformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	var blocks []*ast.FencedCodeBlock

	ast.Walk(doc, func(node ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}

		cb, ok := node.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}

		lang := cb.Language(reader.Source())
		if !bytes.Equal(lang, geomapLang) {
			return ast.WalkContinue, nil
		}

		blocks = append(blocks, cb)
		return ast.WalkContinue, nil
	})

	if len(blocks) == 0 {
		return
	}

	for _, cb := range blocks {
		b := new(Block)
		b.SetLines(cb.Lines())

		if cb.Info != nil {
			info := string(cb.Info.Segment.Value(reader.Source()))
			if opts, err := parseFenceOptions(info); err == nil && len(opts) > 0 {
				b.Options = opts
			}
		}

		parent := cb.Parent()
		if parent != nil {
			parent.ReplaceChild(parent, cb, b)
		}
	}
}

func parseFenceOptions(info string) (map[string]any, error) {
	trimmed := strings.TrimSpace(info)
	if trimmed == "" {
		return nil, nil
	}

	space := strings.IndexAny(trimmed, " \t")
	if space < 0 {
		return nil, nil
	}

	meta := strings.TrimSpace(trimmed[space+1:])
	if strings.HasPrefix(meta, "{{") && strings.HasSuffix(meta, "}}") {
		return nil, nil
	}
	if !strings.HasPrefix(meta, "{") || !strings.HasSuffix(meta, "}") {
		return nil, nil
	}

	body := strings.TrimSpace(meta[1 : len(meta)-1])
	if body == "" {
		return map[string]any{}, nil
	}

	return parseInlineOptions(body)
}
