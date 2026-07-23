package httpext

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Transformer struct{}

var httpClientLang = []byte("http")

func (t *Transformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	var blocks []*ast.FencedCodeBlock

	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		cb, ok := node.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}
		if !bytes.Equal(cb.Language(reader.Source()), httpClientLang) {
			return ast.WalkContinue, nil
		}
		blocks = append(blocks, cb)
		return ast.WalkContinue, nil
	})

	for _, cb := range blocks {
		b := bytes.Buffer{}
		lines := cb.Lines()
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			b.Write(line.Value(reader.Source()))
		}

		opts := FenceOptions{ShowRequest: true, ShowLineNumbers: false, IndentJSON: true, ClassStyles: map[string]string{}}
		if cb.Info != nil {
			opts = parseFenceOptions(string(cb.Info.Segment.Value(reader.Source())))
		}

		req, rsp, err := executeRawHTTPClient(b.String())
		node := &Block{ShowRequest: opts.ShowRequest, ShowLineNumbers: opts.ShowLineNumbers, IndentJSON: opts.IndentJSON, ClassStyles: opts.ClassStyles, Warnings: opts.Warnings}
		if err != nil {
			node.Request = req
			node.Response = rsp
			node.ExecuteError = err.Error()
			if rsp == "" {
				node.Response = err.Error()
			}
		} else {
			node.Request = req
			node.Response = rsp
		}

		parent := cb.Parent()
		if parent != nil {
			parent.ReplaceChild(parent, cb, node)
		}
	}
}
