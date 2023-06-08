package tql

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql/fmap"
	"github.com/machbase/neo-server/mods/tql/fsink"
	"github.com/machbase/neo-server/mods/tql/fsrc"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Tql interface {
	Execute(ctx context.Context, db spi.Database, output spec.OutputStream) error
	ExecuteEncoder(ctxCtx context.Context, db spi.Database, encoder codec.RowsEncoder) error
	ExecuteHandler(ctx context.Context, db spi.Database, w http.ResponseWriter) error
}

type tagQL struct {
	input    fsrc.Input
	mapExprs []string
	sinkExpr string
	params   map[string][]string
}

var tqlFunctions = map[string]expression.Function{}

func init() {
	for _, f := range fsrc.Functions() {
		tqlFunctions[f] = nil
	}
	for _, f := range fsink.Functions() {
		tqlFunctions[f] = nil
	}
	for _, f := range fmap.Functions() {
		tqlFunctions[f] = nil
	}
}

func Parse(in io.Reader) (Tql, error) {
	return ParseWithParams(in, nil)
}

func ParseWithParams(in io.Reader, params map[string][]string) (Tql, error) {
	reader := bufio.NewReader(in)
	parts := []byte{}
	stmt := []string{}
	expressions := []Line{}
	lineNo := 0
	for {
		b, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				if len(stmt) > 0 {
					line := Line{
						text: strings.Join(stmt, ""),
						line: lineNo,
					}
					if len(strings.TrimSpace(line.text)) > 0 {
						expressions = append(expressions, line)
					}
				}
				break
			}
			return nil, err
		}
		parts = append(parts, b...)
		if isPrefix {
			continue
		}
		lineNo++

		lineText := string(parts)
		parts = parts[:0]

		if strings.TrimSpace(lineText) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lineText), "#") {
			continue
		}

		aStmt := strings.Join(append(stmt, lineText), "")
		_, err = expression.ParseTokens(aStmt, tqlFunctions)
		if err != nil && err.Error() == "unbalanced parenthesis" {
			stmt = append(stmt, lineText)
			continue
		} else if err != nil {
			return nil, err
		} else {
			stmt = append(stmt, lineText)
			line := Line{
				text: strings.Join(stmt, ""),
				line: lineNo,
			}
			if len(strings.TrimSpace(line.text)) > 0 {
				expressions = append(expressions, line)
			}
			stmt = stmt[:0]
		}
	}

	tq, err := parseExpressions(expressions, params)
	if err != nil {
		return nil, err
	}
	if tagql, ok := tq.(*tagQL); ok {
		tagql.params = params
	}
	return tq, nil
}

type Line struct {
	text string
	line int
}

func parseExpressions(exprs []Line, params map[string][]string) (Tql, error) {
	if len(exprs) == 0 {
		return nil, errors.New("empty expressions")
	}

	tq := &tagQL{}

	// src
	if len(exprs) >= 1 {
		srcLine := exprs[0]
		src, err := fsrc.Compile(srcLine.text, params)
		if err != nil {
			return nil, errors.Wrapf(err, "at line %d", srcLine.line)
		}
		tq.input = src
	}

	// sink
	if len(exprs) >= 2 {
		sinkLine := exprs[len(exprs)-1]
		// validates the syntax
		_, err := fsink.Parse(sinkLine.text)
		if err != nil {
			return nil, errors.Wrapf(err, "at line %d", sinkLine.line)
		}
		tq.sinkExpr = sinkLine.text
	}

	// map
	if len(exprs) >= 3 {
		exprs = exprs[1 : len(exprs)-1]
		for _, mapLine := range exprs {
			// validates the syntax
			_, err := fmap.Parse(mapLine.text)
			if err != nil {
				return nil, errors.Wrapf(err, "at line %d", mapLine.line)
			}
			tq.mapExprs = append(tq.mapExprs, mapLine.text)
		}
	}
	return tq, nil
}

func ParseContext(ctx context.Context, params map[string][]string) (Tql, error) {
	tq := &tagQL{
		params: params,
	}

	tqls := params["_tq"]
	if len(tqls) < 2 {
		return nil, errors.New("tql require at leat two '_tq' params for source and sink")
	}

	var err error
	tq.input, err = fsrc.Compile(tqls[0], tq.params)
	if err != nil {
		return nil, err
	}

	for _, part := range tqls[1 : len(tqls)-1] {
		// validates the syntax
		_ /*expr */, err := fmap.Parse(part)
		if err != nil {
			return nil, err
		}
		tq.mapExprs = append(tq.mapExprs, part)
	}

	// validates the syntax
	_ /*expr*/, err = fsink.Parse(tqls[len(tqls)-1])
	if err != nil {
		return nil, err
	}
	tq.sinkExpr = tqls[len(tqls)-1]

	return tq, nil
}

func (tq *tagQL) ExecuteHandler(ctx context.Context, db spi.Database, w http.ResponseWriter) error {
	var compress = ""
	var output spec.OutputStream

	switch compress {
	case "gzip":
		output = &stream.WriterOutputStream{Writer: gzip.NewWriter(w)}
	default:
		compress = ""
		output = &stream.WriterOutputStream{Writer: w}
	}

	encoder, err := tq.buildEncoder(output, tq.params)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", encoder.ContentType())
	if len(compress) > 0 {
		w.Header().Set("Content-Encoding", compress)
	}
	return tq.ExecuteEncoder(ctx, db, encoder)
}

func (tq *tagQL) Execute(ctxCtx context.Context, db spi.Database, output spec.OutputStream) (err error) {
	if output == nil {
		output, err = stream.NewOutputStream("-")
		if err != nil {
			return err
		}
	}
	encoder, err := tq.buildEncoder(output, tq.params)
	if err != nil {
		return err
	}
	return tq.ExecuteEncoder(ctxCtx, db, encoder)
}

func (tq *tagQL) ExecuteEncoder(ctxCtx context.Context, db spi.Database, encoder codec.RowsEncoder) (err error) {
	exprs := []*expression.Expression{}
	for _, str := range tq.mapExprs {
		expr, err := fmap.Parse(str)
		if err != nil {
			return errors.Wrapf(err, "at %s", str)
		}
		exprs = append(exprs, expr)
	}

	chain, err := newExecutionChain(ctxCtx, db, tq.input, encoder, exprs, tq.params)
	if err != nil {
		return err
	}
	return chain.Run()
}

func (tq *tagQL) buildEncoder(output spec.OutputStream, params map[string][]string) (codec.RowsEncoder, error) {
	sinkExpr, err := fsink.Parse(tq.sinkExpr)
	if err != nil {
		return nil, err
	}
	sinkRet, err := sinkExpr.Eval(&fsink.Context{Output: output, Params: params})
	if err != nil {
		return nil, err
	}
	encoder, ok := sinkRet.(codec.RowsEncoder)
	if !ok {
		return nil, fmt.Errorf("invalid sink type: %T", sinkRet)
	}
	return encoder, nil
}
