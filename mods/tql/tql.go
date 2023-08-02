package tql

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	tqlcontext "github.com/machbase/neo-server/mods/tql/context"
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
	input    *input
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
		src, err := CompileSource(srcLine.text, dataReader, params)
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

func ParseSource(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, fx.GenFunctions)
}
func ParseSink(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, fx.GenFunctions)
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

func CompileSource(code string, dataReader io.Reader, params map[string][]string) (*input, error) {
	expr, err := ParseSource(code)
	if err != nil {
		return nil, err
	}
	src, err := expr.Eval(&inputContext{Body: dataReader, params: params})
	if err != nil {
		return nil, err
	}
	var ret *input
	switch src := src.(type) {
	case maps.DatabaseSource:
		ret = &input{dbSrc: src}
	case maps.FakeSource:
		ret = &input{fakeSrc: src}
	case maps.ReaderSource:
		ret = &input{readerSrc: src}
	default:
		return nil, fmt.Errorf("f(INPUT) unknown type of arg, %T", src)
	}
	return ret, nil
}

type inputContext struct {
	Body   io.Reader
	params map[string][]string
}

func (p *inputContext) Get(name string) (any, error) {
	if strings.HasPrefix(name, "$") {
		if p, ok := p.params[strings.TrimPrefix(name, "$")]; ok {
			if len(p) > 0 {
				return p[len(p)-1], nil
			}
		}
		return nil, nil
	} else {
		switch name {
		default:
			return nil, fmt.Errorf("undefined variable '%s'", name)
		case "CTX":
			return p, nil
		case "PI":
			return math.Pi, nil
		case "nil":
			return nil, nil
		}
	}
}

type input struct {
	dbSrc     maps.DatabaseSource
	fakeSrc   maps.FakeSource
	readerSrc maps.ReaderSource
}

// for test and debug purpose
func (in *input) ToSQL() string {
	if in.dbSrc == nil {
		return ""
	}
	return in.dbSrc.ToSQL()
}

func (in *input) Run(deligate InputDeligate) error {
	if in.dbSrc == nil && in.fakeSrc == nil && in.readerSrc == nil {
		return errors.New("nil source")
	}
	if deligate == nil {
		return errors.New("nil deligate")
	}

	fetched := 0
	executed := false
	if in.dbSrc != nil {
		queryCtx := &do.QueryContext{
			DB: deligate.Database(),
			OnFetchStart: func(c spi.Columns) {
				deligate.FeedHeader(c)
			},
			OnFetch: func(nrow int64, values []any) bool {
				fetched++
				if deligate.ShouldStop() {
					return false
				} else {
					deligate.Feed(values)
					return true
				}
			},
			OnFetchEnd: func() {},
			OnExecuted: func(usermsg string, rowsAffected int64) {
				executed = true
			},
		}
		if msg, err := do.Query(queryCtx, in.dbSrc.ToSQL()); err != nil {
			deligate.Feed(nil)
			return err
		} else {
			if executed {
				deligate.FeedHeader(spi.Columns{{Name: "message", Type: "string"}})
				deligate.Feed([]any{msg})
				deligate.Feed(nil)
			} else if fetched == 0 {
				deligate.Feed([]any{tqlcontext.ExecutionEOF})
				deligate.Feed(nil)
			} else {
				deligate.Feed(nil)
			}
			return nil
		}
	} else if in.fakeSrc != nil {
		deligate.FeedHeader(in.fakeSrc.Header())
		for values := range in.fakeSrc.Gen() {
			deligate.Feed(values)
			if deligate.ShouldStop() {
				in.fakeSrc.Stop()
				break
			}
		}
		deligate.Feed(nil)
		return nil
	} else if in.readerSrc != nil {
		deligate.FeedHeader(in.readerSrc.Header())
		for values := range in.readerSrc.Gen() {
			deligate.Feed(values)
			if deligate.ShouldStop() {
				in.readerSrc.Stop()
				break
			}
		}
		deligate.Feed(nil)
		return nil
	} else {
		return errors.New("no source")
	}
}

type InputDeligate interface {
	Database() spi.Database
	ShouldStop() bool
	FeedHeader(spi.Columns)
	Feed([]any)
}

type InputDelegateWrapper struct {
	DatabaseFunc   func() spi.Database
	ShouldStopFunc func() bool
	FeedHeaderFunc func(spi.Columns)
	FeedFunc       func([]any)
}

func (w *InputDelegateWrapper) Database() spi.Database {
	if w.DatabaseFunc == nil {
		return nil
	}
	return w.DatabaseFunc()
}

func (w *InputDelegateWrapper) ShouldStop() bool {
	if w.ShouldStopFunc == nil {
		return false
	}
	return w.ShouldStopFunc()
}

func (w *InputDelegateWrapper) FeedHeader(c spi.Columns) {
	if w.FeedHeaderFunc != nil {
		w.FeedHeaderFunc(c)
	}
}

func (w *InputDelegateWrapper) Feed(v []any) {
	if w.FeedFunc != nil {
		w.FeedFunc(v)
	}
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
	sink, err := expr.Eval(&OutputContext{Output: outputStream, Params: params})
	if err != nil {
		return nil, err
	}

	switch val := sink.(type) {
	case *maps.Encoder:
		ret := &output{}
		ret.encoder = val.RowEncoder(
			opts.OutputStream(outputStream),
			opts.AssetHost("/web/echarts/"),
			opts.ChartJson(toJsonOutput),
		)
		if _, ok := ret.encoder.(opts.CanSetChartJson); ok {
			ret.isChart = true
		}
		return ret, nil
	case maps.DatabaseSink:
		ret := &output{}
		ret.dbSink = val
		ret.dbSink.SetOutputStream(outputStream)
		return ret, nil
	default:
		return nil, fmt.Errorf("invalid sink type: %T", val)
	}
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
