package tql

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/expression"
	tqlcontext "github.com/machbase/neo-server/mods/tql/context"
	"github.com/machbase/neo-server/mods/tql/fx"
	"github.com/machbase/neo-server/mods/tql/maps"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Tql interface {
	Execute(task fx.Task, db spi.Database) error
	ExecuteHandler(task fx.Task, db spi.Database, w http.ResponseWriter) error
}

type tagQL struct {
	input    *input
	output   *output
	mapExprs []string
}

func Parse(task fx.Task, codeReader io.Reader) (Tql, error) {
	lines, err := readLines(task, codeReader)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, errors.New("empty expressions")
	}

	var exprs []*Line
	for _, line := range lines {
		if line.isComment {
			if strings.HasPrefix(line.text, "+") {
				toks := strings.Split(line.text[1:], ",")
				for _, t := range toks {
					task.AddPragma(strings.TrimSpace(t))
				}
			}
		} else {
			exprs = append(exprs, line)
		}
	}

	tq := &tagQL{}
	// src
	if len(exprs) >= 1 {
		srcLine := exprs[0]
		src, err := CompileSource(task, srcLine.text)
		if err != nil {
			return nil, errors.Wrapf(err, "at line %d", srcLine.line)
		}
		tq.input = src
	}

	// sink
	if len(exprs) >= 2 {
		sinkLine := exprs[len(exprs)-1]
		// validates the syntax
		sink, err := CompileSink(task, sinkLine.text)
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
			_, err := ParseMap(task, mapLine.text)
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

func ParseMap(task fx.Task, text string) (*expression.Expression, error) {
	for _, f := range mapFunctionsMacro {
		text = strings.ReplaceAll(text, f[0], f[1])
	}
	text = strings.ReplaceAll(text, ",V,)", ",V)")
	text = strings.ReplaceAll(text, "K,V,K,V", "K,V")
	return expression.NewWithFunctions(text, task.Functions())
}

func ParseSource(task fx.Task, text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, task.Functions())
}

func ParseSink(task fx.Task, text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, task.Functions())
}

func (tq *tagQL) ExecuteHandler(task fx.Task, db spi.Database, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", tq.output.ContentType())
	if contentEncoding := tq.output.ContentEncoding(); len(contentEncoding) > 0 {
		w.Header().Set("Content-Encoding", contentEncoding)
	}
	if tq.output.IsChart() {
		w.Header().Set("X-Chart-Type", "echarts")
	}
	return tq.Execute(task, db)
}

func (tq *tagQL) Execute(task fx.Task, db spi.Database) (err error) {
	exprs := []*expression.Expression{}
	for _, str := range tq.mapExprs {
		expr, err := ParseMap(task, str)
		if err != nil {
			return errors.Wrapf(err, "at %s", str)
		}
		if expr == nil {
			return fmt.Errorf("compile error at %s", str)
		}
		exprs = append(exprs, expr)
	}

	chain, err := newExecutionChain(task, db, tq.input, tq.output, exprs)
	if err != nil {
		return err
	}
	return chain.Run()
}

func CompileSource(task fx.Task, code string) (*input, error) {
	expr, err := ParseSource(task, code)
	if err != nil {
		return nil, err
	}
	src, err := expr.Eval(task)
	if err != nil {
		return nil, err
	}
	var ret *input
	switch src := src.(type) {
	case maps.DatabaseSource:
		ret = &input{dbSrc: src}
	case maps.ChannelSource:
		ret = &input{chSrc: src}
	default:
		return nil, fmt.Errorf("%T is not applicable for INPUT", src)
	}
	return ret, nil
}

type input struct {
	dbSrc maps.DatabaseSource
	chSrc maps.ChannelSource
}

// for test and debug purpose
func (in *input) ToSQL() string {
	if in.dbSrc == nil {
		return ""
	}
	return in.dbSrc.ToSQL()
}

func (in *input) Run(deligate InputDeligate) error {
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
				deligate.Feed([]any{tqlcontext.ExecutionEOF})
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

func CompileSink(task fx.Task, code string) (*output, error) {
	expr, err := ParseSink(task, code)
	if err != nil {
		return nil, err
	}
	sink, err := expr.Eval(task)
	if err != nil {
		return nil, err
	}

	switch val := sink.(type) {
	case *maps.Encoder:
		ret := &output{}
		ret.encoder = val.RowEncoder(
			opts.OutputStream(task.OutputStream()),
			opts.AssetHost("/web/echarts/"),
			opts.ChartJson(task.ShouldJsonOutput()),
		)
		if _, ok := ret.encoder.(opts.CanSetChartJson); ok {
			ret.isChart = true
		}
		return ret, nil
	case maps.DatabaseSink:
		ret := &output{}
		ret.dbSink = val
		ret.dbSink.SetOutputStream(task.OutputStream())
		return ret, nil
	default:
		return nil, fmt.Errorf("%T is not applicable for OUTPUT", val)
	}
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
