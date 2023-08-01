package tql

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/fsink"
	"github.com/machbase/neo-server/mods/tql/fsrc"
	"github.com/machbase/neo-server/mods/tql/fx"
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

	// comments start with plus(+) symbold and sperated by comma.
	// ex) => `// +brief, markdown`
	pragma []string
}

func Parse(codeReader io.Reader, dataReader io.Reader, params map[string][]string, dataWriter io.Writer, toJsonOutput bool) (Tql, error) {
	lines, err := readLines(codeReader)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, errors.New("empty expressions")
	}

	var exprs []*Line
	var pragma []string
	for _, line := range lines {
		if line.isComment {
			if strings.HasPrefix(line.text, "+") {
				toks := strings.Split(line.text[1:], ",")
				for _, t := range toks {
					pragma = append(pragma, strings.TrimSpace(t))
				}
			}
		} else {
			exprs = append(exprs, line)
		}
	}

	tq := &tagQL{params: params, pragma: pragma}
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
		sink, err := fsink.Compile(sinkLine.text, params, dataWriter, toJsonOutput)
		if err != nil {
			return nil, errors.Wrapf(err, "at line %d", sinkLine.line)
		}
		tq.output = sink
	} else {
		return nil, errors.New("tql contains no output")
	}

	// map
	if len(exprs) >= 3 {
		exprs = exprs[1 : len(exprs)-1]
		for _, mapLine := range exprs {
			// validates the syntax
			_, err := ParseMap(mapLine.text)
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
	if tq.output.IsChart() {
		w.Header().Set("X-Chart-Type", "echarts")
	}
	return tq.Execute(ctx, db)
}

func (tq *tagQL) Execute(ctx context.Context, db spi.Database) (err error) {
	exprs := []*expression.Expression{}
	for _, str := range tq.mapExprs {
		expr, err := ParseMap(str)
		if err != nil {
			return errors.Wrapf(err, "at %s", str)
		}
		if expr == nil {
			return fmt.Errorf("compile error at %s", str)
		}
		exprs = append(exprs, expr)
	}

	chain, err := newExecutionChain(ctx, db, tq.input, tq.output, exprs, tq.params)
	if err != nil {
		return err
	}
	return chain.Run()
}

var mapFunctionsMacro = [][2]string{
	{"SCRIPT(", "SCRIPT(CTX,K,V,"},
	{"TAKE(", "TAKE(CTX,K,V,"},
	{"DROP(", "DROP(CTX,K,V,"},
	{"PUSHKEY(", "PUSHKEY(CTX,K,V,"},
	{"POPKEY(", "POPKEY(CTX,K,V,"},
	{"GROUPBYKEY(", "GROUPBYKEY(CTX,K,V,"},
	{"FLATTEN(", "FLATTEN(CTX,K,V,"},
	{"FILTER(", "FILTER(CTX,K,V,"},
	{"FFT(", "FFT(CTX,K,V,"},
}

func ParseMap(text string) (*expression.Expression, error) {
	for _, f := range mapFunctionsMacro {
		text = strings.ReplaceAll(text, f[0], f[1])
	}
	text = strings.ReplaceAll(text, ",V,)", ",V)")
	text = strings.ReplaceAll(text, "K,V,K,V", "K,V")
	return expression.NewWithFunctions(text, fx.GenFunctions)
}
