package tql

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql/fsrc"
	"github.com/machbase/neo-server/mods/tql/fx"
	"github.com/machbase/neo-server/mods/tql/maps"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Tql interface {
	Execute(ctx context.Context, db spi.Database) error
	ExecuteHandler(ctx context.Context, db spi.Database, w http.ResponseWriter) error
}

type tagQL struct {
	input    fsrc.Input
	output   *output
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
		sink, err := CompileSink(sinkLine.text, params, dataWriter, toJsonOutput)
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

func ParseSink(text string) (*expression.Expression, error) {
	text = strings.ReplaceAll(text, "OUTPUT(", "OUTPUT(outstream,")
	text = strings.ReplaceAll(text, "outputstream,)", "outputstream)")
	return expression.NewWithFunctions(text, fx.GenFunctions)
}

func CompileSink(code string, params map[string][]string, writer io.Writer, toJsonOutput bool) (*output, error) {
	expr, err := ParseSink(code)
	if err != nil {
		return nil, err
	}
	var outputStream spec.OutputStream
	if writer == nil {
		outputStream, err = stream.NewOutputStream("-")
		if err != nil {
			return nil, err
		}
	} else {
		outputStream = &stream.WriterOutputStream{Writer: writer}
	}
	result, err := expr.Eval(&OutputContext{Output: outputStream, Params: params})
	if err != nil {
		return nil, err
	}

	ret := &output{}
	switch v := result.(type) {
	case codec.RowsEncoder:
		if o, ok := v.(opts.CanSetChartJson); ok {
			o.SetChartJson(toJsonOutput)
			ret.isChart = true
		}
		ret.encoder = v
	case maps.DatabaseSink:
		ret.dbSink = v
	default:
		return nil, fmt.Errorf("invalid sink type: %T", result)
	}
	return ret, nil
}

type OutputContext struct {
	Output spec.OutputStream
	Params map[string][]string
}

func (ctx *OutputContext) Get(name string) (any, error) {
	if name == "CTX" {
		return ctx, nil
	} else if name == "outstream" {
		return ctx.Output, nil
	} else if name == "nil" {
		return nil, nil
	} else if strings.HasPrefix(name, "$") {
		if p, ok := ctx.Params[strings.TrimPrefix(name, "$")]; ok {
			if len(p) > 0 {
				return p[len(p)-1], nil
			}
		}
		return nil, nil
	}
	return nil, fmt.Errorf("undefined variable '%s'", name)
}

type output struct {
	encoder codec.RowsEncoder
	dbSink  maps.DatabaseSink
	isChart bool
}

func (out *output) SetHeader(cols spi.Columns) {
	if out.encoder != nil {
		codec.SetEncoderColumns(out.encoder, cols)
	}
}

func (out *output) ContentType() string {
	if out.encoder != nil {
		return out.encoder.ContentType()
	}
	return "application/octet-stream"
}

func (out *output) IsChart() bool {
	return out.isChart
}

func (out *output) ContentEncoding() string {
	//ex: return "gzip" for  Content-Encoding: gzip
	return ""
}

func (out *output) AddRow(vals []any) error {
	if out.encoder != nil {
		return out.encoder.AddRow(vals)
	} else if out.dbSink != nil {
		return out.dbSink.AddRow(vals)
	}
	return errors.New("no output encoder")
}

func (out *output) Open(db spi.Database) error {
	if out.encoder != nil {
		return out.encoder.Open()
	} else if out.dbSink != nil {
		return out.dbSink.Open(db)
	}
	return errors.New("no output encoder")
}

func (out *output) Close() {
	if out.encoder != nil {
		out.encoder.Close()
	} else if out.dbSink != nil {
		out.dbSink.Close()
	}
}
