package tql

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Task struct {
	ctx          context.Context
	functions    map[string]expression.Function
	params       map[string][]string
	inputReader  io.Reader
	outputWriter io.Writer
	outputStream spec.OutputStream
	toJsonOutput bool

	// comments start with plus(+) symbold and sperated by comma.
	// ex) => `// +brief, markdown`
	pragma []string

	// compiled result
	compiled   bool
	compileErr error
	input      *input
	output     *output
	mapExprs   []string
}

var _ expression.Parameters = &Task{}

func NewTaskContext(ctx context.Context) *Task {
	ret := NewTask()
	ret.ctx = ctx
	return ret
}

func (x *Task) Context() context.Context {
	return x.ctx
}

func (x *Task) Function(name string) expression.Function {
	return x.functions[name]
}

func (x *Task) SetInputReader(r io.Reader) {
	x.inputReader = r
}

func (x *Task) InputReader() io.Reader {
	return x.inputReader
}

func (x *Task) SetOutputWriter(w io.Writer) error {
	var err error
	x.outputWriter = w
	if w == nil {
		x.outputStream, err = stream.NewOutputStream("-")
		if err != nil {
			return err
		}
	} else {
		x.outputStream = &stream.WriterOutputStream{Writer: w}
	}
	return nil
}

func (x *Task) OutputWriter() io.Writer {
	return x.outputWriter
}

func (x *Task) SetOutputStream(o spec.OutputStream) {
	x.outputStream = o
	x.outputWriter = o
}

func (x *Task) OutputStream() spec.OutputStream {
	return x.outputStream
}

func (x *Task) SetJsonOutput(flag bool) {
	x.toJsonOutput = flag
}

func (x *Task) ShouldJsonOutput() bool {
	return x.toJsonOutput
}

func (x *Task) SetParams(p map[string][]string) {
	if x.params == nil {
		x.params = map[string][]string{}
	}
	for k, v := range p {
		x.params[k] = v
	}
}

func (x *Task) Params() map[string][]string {
	return x.params
}

func (x *Task) Get(name string) (any, error) {
	if strings.HasPrefix(name, "$") {
		if p, ok := x.params[strings.TrimPrefix(name, "$")]; ok {
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
			return x, nil
		case "PI":
			return math.Pi, nil
		case "outputstream":
			return x.outputStream, nil
		case "nil":
			return nil, nil
		}
	}
}

func (x *Task) AddPragma(p string) {
	x.pragma = append(x.pragma, p)
}

func (x *Task) CompileScript(sc *Script) error {
	file, err := os.Open(sc.path)
	if err != nil {
		return err
	}
	defer file.Close()
	return x.Compile(file)
}

func (x *Task) CompileString(code string) error {
	return x.Compile(bytes.NewBufferString(code))
}

func (x *Task) Compile(codeReader io.Reader) error {
	x.compiled = true
	lines, err := readLines(x, codeReader)
	if err != nil {
		x.compileErr = err
		return err
	}
	if len(lines) == 0 {
		x.compileErr = errors.New("empty expressions")
		return x.compileErr
	}

	var exprs []*Line
	for _, line := range lines {
		if line.isComment {
			if strings.HasPrefix(line.text, "+") {
				toks := strings.Split(line.text[1:], ",")
				for _, t := range toks {
					x.AddPragma(strings.TrimSpace(t))
				}
			}
		} else {
			exprs = append(exprs, line)
		}
	}

	// src
	if len(exprs) >= 1 {
		srcLine := exprs[0]
		src, err := x.compileSource(srcLine.text)
		if err != nil {
			x.compileErr = errors.Wrapf(err, "at line %d", srcLine.line)
			return x.compileErr
		}
		x.input = src
	}

	// sink
	if len(exprs) >= 2 {
		sinkLine := exprs[len(exprs)-1]
		// validates the syntax
		sink, err := x.compileSink(sinkLine.text)
		if err != nil {
			x.compileErr = errors.Wrapf(err, "at line %d", sinkLine.line)
			return x.compileErr
		}
		x.output = sink
	}

	// map
	if len(exprs) >= 3 {
		exprs = exprs[1 : len(exprs)-1]
		for _, mapLine := range exprs {
			// validates the syntax
			_, err := x.Parse(mapLine.text)
			if err != nil {
				x.compileErr = errors.Wrapf(err, "at line %d", mapLine.line)
				return x.compileErr
			}
			x.mapExprs = append(x.mapExprs, mapLine.text)
		}
	}
	return nil
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

func (x *Task) Parse(text string) (*expression.Expression, error) {
	for _, f := range mapFunctionsMacro {
		text = strings.ReplaceAll(text, f[0], f[1])
	}
	text = strings.ReplaceAll(text, ",V,)", ",V)")
	text = strings.ReplaceAll(text, "K,V,K,V", "K,V")
	return expression.NewWithFunctions(text, x.functions)
}

func (x *Task) parseSource(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, x.functions)
}

func (x *Task) parseSink(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, x.functions)
}

// DumpSQL returns the generated SQL statement if the input source database source
func (x *Task) DumpSQL() string {
	if x.input == nil || x.input.dbSrc == nil {
		return ""
	}
	return x.input.dbSrc.ToSQL()
}

func (x *Task) ExecuteHandler(db spi.Database, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", x.output.ContentType())
	if contentEncoding := x.output.ContentEncoding(); len(contentEncoding) > 0 {
		w.Header().Set("Content-Encoding", contentEncoding)
	}
	if x.output.IsChart() {
		w.Header().Set("X-Chart-Type", "echarts")
	}
	return x.Execute(db)
}

func (x *Task) Execute(db spi.Database) (err error) {
	exprs := []*expression.Expression{}
	for _, str := range x.mapExprs {
		expr, err := x.Parse(str)
		if err != nil {
			return errors.Wrapf(err, "at %s", str)
		}
		if expr == nil {
			return fmt.Errorf("compile error at %s", str)
		}
		exprs = append(exprs, expr)
	}

	chain, err := newExecutionChain(x, db, x.input, x.output, exprs)
	if err != nil {
		return err
	}
	return chain.Run()
}

func (x *Task) compileSource(code string) (*input, error) {
	expr, err := x.parseSource(code)
	if err != nil {
		return nil, err
	}
	src, err := expr.Eval(x)
	if err != nil {
		return nil, err
	}
	var ret *input
	switch src := src.(type) {
	case DatabaseSource:
		ret = &input{dbSrc: src}
	case ChannelSource:
		ret = &input{chSrc: src}
	default:
		return nil, fmt.Errorf("%T is not applicable for INPUT", src)
	}
	return ret, nil
}

type input struct {
	dbSrc DatabaseSource
	chSrc ChannelSource
}

func (in *input) run(deligate InputDeligate) error {
	if in.dbSrc == nil && in.chSrc == nil {
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
				deligate.Feed([]any{&Record{eof: true}})
				deligate.Feed(nil)
			} else {
				deligate.Feed(nil)
			}
			return nil
		}
	} else if in.chSrc != nil {
		deligate.FeedHeader(in.chSrc.Header())
		for values := range in.chSrc.Gen() {
			deligate.Feed(values)
			if deligate.ShouldStop() {
				in.chSrc.Stop()
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

func (x *Task) compileSink(code string) (*output, error) {
	expr, err := x.parseSink(code)
	if err != nil {
		return nil, err
	}
	sink, err := expr.Eval(x)
	if err != nil {
		return nil, err
	}

	switch val := sink.(type) {
	case *Encoder:
		ret := &output{}
		ret.encoder = val.RowEncoder(
			opts.OutputStream(x.OutputStream()),
			opts.AssetHost("/web/echarts/"),
			opts.ChartJson(x.ShouldJsonOutput()),
		)
		if _, ok := ret.encoder.(opts.CanSetChartJson); ok {
			ret.isChart = true
		}
		return ret, nil
	case DatabaseSink:
		ret := &output{}
		ret.dbSink = val
		ret.dbSink.SetOutputStream(x.OutputStream())
		return ret, nil
	default:
		return nil, fmt.Errorf("%T is not applicable for OUTPUT", val)
	}
}

type output struct {
	encoder codec.RowsEncoder
	dbSink  DatabaseSink
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
