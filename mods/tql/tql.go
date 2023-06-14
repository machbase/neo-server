package tql

import (
	"context"
	"io"
	"net/http"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/fmap"
	"github.com/machbase/neo-server/mods/tql/fsink"
	"github.com/machbase/neo-server/mods/tql/fsrc"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Tql interface {
	Execute(ctx context.Context, db spi.Database) error
	ExecuteHandler(ctx context.Context, db spi.Database, w http.ResponseWriter) error
}

type tagQL struct {
	input    fsrc.Input
	output   fsink.Output
	mapExprs []string
	params   map[string][]string
}

func Parse(codeReader io.Reader, dataReader io.Reader, params map[string][]string, dataWriter io.Writer) (Tql, error) {
	exprs, err := readLines(codeReader)
	if err != nil {
		return nil, err
	}
	if len(exprs) == 0 {
		return nil, errors.New("empty expressions")
	}

	tq := &tagQL{params: params}
	// src
	if len(exprs) >= 1 {
		srcLine := exprs[0]
		src, err := fsrc.Compile(srcLine.text, dataReader, params)
		if err != nil {
			return nil, errors.Wrapf(err, "at line %d", srcLine.line)
		}
		tq.input = src
	}

	// sink
	if len(exprs) >= 2 {
		sinkLine := exprs[len(exprs)-1]
		// validates the syntax
		sink, err := fsink.Compile(sinkLine.text, params, dataWriter)
		if err != nil {
			return nil, errors.Wrapf(err, "at line %d", sinkLine.line)
		}
		tq.output = sink
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

func (tq *tagQL) ExecuteHandler(ctx context.Context, db spi.Database, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", tq.output.ContentType())
	if contentEncoding := tq.output.ContentEncoding(); len(contentEncoding) > 0 {
		w.Header().Set("Content-Encoding", contentEncoding)
	}
	return tq.Execute(ctx, db)
}

func (tq *tagQL) Execute(ctx context.Context, db spi.Database) (err error) {
	exprs := []*expression.Expression{}
	for _, str := range tq.mapExprs {
		expr, err := fmap.Parse(str)
		if err != nil {
			return errors.Wrapf(err, "at %s", str)
		}
		exprs = append(exprs, expr)
	}

	chain, err := newExecutionChain(ctx, db, tq.input, tq.output, exprs, tq.params)
	if err != nil {
		return err
	}
	return chain.Run()
}