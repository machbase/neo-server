package sqlext

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Transformer struct{}

var sqlLang = []byte("sql")

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
		if !bytes.Equal(lang, sqlLang) {
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
		var out strings.Builder
		if !opts.Execute {
			continue
		}
		start := time.Now()
		if err := runSQL(code, opts, &out); err != nil {
			out.WriteString(err.Error())
		}
		elapsed := time.Since(start)
		node := &Block{Source: previewSource(code, opts), Options: opts, Output: out.String(), TimedOut: false}
		if opts.Timeout > 0 && elapsed > opts.Timeout {
			node.TimedOut = true
			node.Output = fmt.Sprintf("timed out after %s", opts.Timeout)
		}
		parent := cb.Parent()
		if parent != nil {
			parent.ReplaceChild(parent, cb, node)
		}
	}
}

func previewSource(code string, opts Options) string {
	if opts.Source == "" || strings.EqualFold(opts.Source, "hide") {
		return ""
	}
	if strings.EqualFold(opts.Source, "all") {
		return code
	}
	return SelectSourceLines(code, opts.Source)
}

func runSQL(code string, opts Options, out *strings.Builder) error {
	req := NewQueryRequest()
	req.SqlText = code
	req.Params = opts.Params
	req.Output = opts.Format
	req.Preview = opts.Preview
	req.TimeFormat = opts.Timeformat
	req.Timezone = opts.TimeLocation
	req.BinaryFormat = opts.BinaryFormat
	req.Header = opts.Header == "skip"
	req.Delimiter = opts.Delimiter
	ctx := context.Background()
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	return req.HandleQuery(out, nil)
}
