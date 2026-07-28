package jshext

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Transformer struct{}

var jshLang = []byte("jsh")
var jsLang = []byte("javascript")

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
		lang := cb.Language(reader.Source())
		if !bytes.Equal(lang, jshLang) && !bytes.Equal(lang, jsLang) {
			return ast.WalkContinue, nil
		}
		blocks = append(blocks, cb)
		return ast.WalkContinue, nil
	})

	for _, cb := range blocks {
		var buf bytes.Buffer
		lines := cb.Lines()
		for i := 0; i < lines.Len(); i++ {
			segment := lines.At(i)
			buf.Write(segment.Value(reader.Source()))
		}
		code := buf.String()
		info := ""
		if cb.Info != nil {
			segment := cb.Info.Segment
			info = string(segment.Value(reader.Source()))
		}
		opts := ParseOptions(info)
		if !opts.Execute {
			continue
		}
		res := runJSHCode(code, opts)
		out := formatExecOutput(res)
		switch opts.Result {
		case "json":
			out = renderExecResultJSON(res)
		case "none":
			out = ""
		}
		exitCode := res.ExitCode
		preview := ""
		if opts.Execute {
			if opts.Source == "" || strings.EqualFold(opts.Source, "hide") {
				preview = ""
			} else if strings.EqualFold(opts.Source, "all") {
				preview = code
			} else {
				preview = SelectSourceLines(code, opts.Source)
			}
		}
		if opts.Execute && preview == "" && !strings.EqualFold(opts.Source, "hide") && !strings.EqualFold(opts.Source, "") {
			preview = SelectSourceLines(code, opts.Source)
		}
		node := &Block{Source: preview, Options: opts, Output: out, ExitCode: exitCode, TimedOut: res.TimedOut}
		parent := cb.Parent()
		if parent != nil {
			parent.ReplaceChild(parent, cb, node)
		}
	}
}
