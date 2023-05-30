package tagql

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tagql/fmap"
	"github.com/machbase/neo-server/mods/tagql/fsink"
	"github.com/machbase/neo-server/mods/tagql/fsrc"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type TagQL interface {
	ToSQL() string
	Execute(ctx context.Context, db spi.Database, output spec.OutputStream) error
	ExecuteEncoder(ctxCtx context.Context, db spi.Database, encoder codec.RowsEncoder) error
	ExecuteHandler(ctx context.Context, db spi.Database, w http.ResponseWriter) error
}

type tagQL struct {
	input    fsrc.Input
	mapExprs []string
	sinkExpr string
}

var regexpTagQL = regexp.MustCompile(`([a-zA-Z0-9_-]+)\/(.+)`)
var regexpSpaceprefix = regexp.MustCompile(`^\s+(.*)`)

func Parse(in io.Reader) (TagQL, error) {
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

		if regexpSpaceprefix.MatchString(lineText) {
			// line starts with whitespace
			stmt = append(stmt, lineText)
			continue
		} else {
			line := Line{
				text: strings.Join(stmt, ""),
				line: lineNo,
			}
			if len(strings.TrimSpace(line.text)) > 0 {
				expressions = append(expressions, line)
			}
			stmt = stmt[:0]
			if len(strings.TrimSpace(lineText)) > 0 {
				stmt = append(stmt, lineText)
			}
		}

	}

	return ParseExpressions(expressions)
}

type Line struct {
	text string
	line int
}

func ParseExpressions(exprs []Line) (TagQL, error) {
	if len(exprs) == 0 {
		return nil, errors.New("empty expressions")
	}

	tq := &tagQL{}

	// src
	if len(exprs) >= 1 {
		srcLine := exprs[0]
		src, err := fsrc.Compile(srcLine.text)
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

func ParseURI(query string) (TagQL, error) {
	return ParseURIContext(context.Background(), query)
}

func ParseURIContext(ctx context.Context, query string) (TagQL, error) {
	subs := regexpTagQL.FindAllStringSubmatch(query, -1)
	if len(subs) != 1 || len(subs[0]) < 3 {
		return nil, errors.New("invalid syntax")
	}

	tq := &tagQL{}

	uri, err := url.Parse("tag:///" + query)
	if err != nil {
		return nil, err
	}

	queryPart := uri.RawQuery

	var params map[string][]string
	if queryPart != "" {
		urlParams, err := url.ParseQuery(queryPart)
		if err != nil {
			return nil, err
		}
		params = urlParams
	}

	srcParts := params["src"]
	if len(srcParts) == 0 {
		tq.input = fsrc.NewDefaultInput()
	} else {
		// only take the last one
		part := srcParts[len(srcParts)-1]
		in, err := fsrc.Compile(part)
		if err != nil {
			return nil, err
		}
		tq.input = in
	}
	mapParts := params["map"]
	for _, part := range mapParts {
		// validates the syntax
		_ /*expr */, err := fmap.Parse(part)
		if err != nil {
			return nil, err
		}
		tq.mapExprs = append(tq.mapExprs, part)
	}

	sinkParts := params["sink"]
	if len(sinkParts) > 0 {
		// only take the last one
		part := sinkParts[len(sinkParts)-1]
		// validates the syntax
		_ /*expr*/, err := fsink.Parse(part)
		if err != nil {
			return nil, err
		}
		tq.sinkExpr = part
	}

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

	encoder, err := tq.buildEncoder(output)
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
	encoder, err := tq.buildEncoder(output)
	if err != nil {
		return err
	}
	return tq.ExecuteEncoder(ctxCtx, db, encoder)
}

func (tq *tagQL) ExecuteEncoder(ctxCtx context.Context, db spi.Database, encoder codec.RowsEncoder) (err error) {
	deferHooks := []func(){}
	defer func() {
		for _, f := range deferHooks {
			f()
		}
	}()

	chain, err := NewExecutionChain(ctxCtx, tq.mapExprs)
	if err != nil {
		return err
	}

	var cols spi.Columns
	go chain.Start()
	go func() {
		open := false
		for arr := range chain.Sink() {
			if !open {
				if len(tq.mapExprs) > 0 {
					for i, v := range arr {
						cols = append(cols, &spi.Column{
							Name: fmt.Sprintf("C%02d", i),
							Type: fmt.Sprintf("%T", v)})
					}
				}
				codec.SetEncoderColumns(encoder, cols)
				encoder.Open()
				deferHooks = append(deferHooks, func() {
					// if close encoder right away without defer,
					// it will crash, because it could be earlier than all map() pipe to be closed
					encoder.Close()
				})
				open = true
			}
			encoder.AddRow(arr)
		}
	}()
	//_, err = do.Query(queryCtx, tq.ToSQL())

	deligate := &fsrc.InputDelegateWrapper{
		DatabaseFunc:   func() spi.Database { return db },
		ShouldStopFunc: func() bool { return chain.Error() != nil },
		FeedHeaderFunc: func(c spi.Columns) { cols = c },
		FeedFunc:       func(values []any) { chain.Source(values) },
	}

	err = tq.input.Run(deligate)

	chain.Wait()
	chain.Stop()

	if chain.Error() != nil {
		return chain.Error()
	}
	return err
}

func (tq *tagQL) buildEncoder(output spec.OutputStream) (codec.RowsEncoder, error) {
	sinkExpr, err := fsink.Parse(tq.sinkExpr)
	if err != nil {
		return nil, err
	}
	sinkRet, err := sinkExpr.Evaluate(map[string]any{"outstream": output})
	if err != nil {
		return nil, err
	}
	encoder, ok := sinkRet.(codec.RowsEncoder)
	if !ok {
		return nil, fmt.Errorf("invalid sink type: %T", sinkRet)
	}
	return encoder, nil
}

func (tq *tagQL) ToSQL() string {
	return tq.input.Source().ToSQL()
}
