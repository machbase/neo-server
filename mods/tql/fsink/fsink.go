package fsink

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql/fx"
	spi "github.com/machbase/neo-spi"
)

type Context struct {
	Output spec.OutputStream
	Params map[string][]string
}

func (ctx *Context) Get(name string) (any, error) {
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

func Parse(text string) (*expression.Expression, error) {
	text = strings.ReplaceAll(text, "OUTPUT(", "OUTPUT(outstream,")
	text = strings.ReplaceAll(text, "outputstream,)", "outputstream)")
	return expression.NewWithFunctions(text, functions)
}

type Output interface {
	Open(db spi.Database) error
	Close()
	ContentType() string
	ContentEncoding() string
	SetHeader(spi.Columns)
	AddRow([]any) error
	IsChart() bool
}

func Compile(code string, params map[string][]string, writer io.Writer, toJsonOutput bool) (Output, error) {
	expr, err := Parse(code)
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
	result, err := expr.Eval(&Context{Output: outputStream, Params: params})
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
	case dbSink:
		ret.dbSink = v
	default:
		return nil, fmt.Errorf("invalid sink type: %T", result)
	}
	return ret, nil
}

type output struct {
	encoder codec.RowsEncoder
	dbSink  dbSink
	isChart bool
}

var _ Output = &output{}

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

var functions = map[string]expression.Function{
	// sink functions
	"OUTPUT":          OUTPUT,
	"CSV":             CSV,
	"JSON":            JSON,
	"MARKDOWN":        MARKDOWN,
	"INSERT":          INSERT,
	"APPEND":          APPEND,
	"CHART_LINE":      CHART_LINE,
	"CHART_SCATTER":   CHART_SCATTER,
	"CHART_BAR":       CHART_BAR,
	"CHART_LINE3D":    CHART_LINE3D,
	"CHART_BAR3D":     CHART_BAR3D,
	"CHART_SURFACE3D": CHART_SURFACE3D,
	"CHART_SCATTER3D": CHART_SCATTER3D,
}

func init() {
	for k, v := range fx.GenFunctions {
		functions[k] = v
	}
}

func Functions() []string {
	ret := []string{}
	for k := range functions {
		ret = append(ret, k)
	}
	return ret
}

type Encoder struct {
	format string
	opts   []opts.Option
}

func newEncoder(format string, args ...any) (*Encoder, error) {
	ret := &Encoder{
		format: format,
	}
	for _, arg := range args {
		if opt, ok := arg.(opts.Option); ok {
			ret.opts = append(ret.opts, opt)
		}
	}
	return ret, nil
}

func MARKDOWN(args ...any) (any, error) {
	return newEncoder("markdown", args...)
}

func CSV(args ...any) (any, error) {
	return newEncoder("csv", args...)
}

func JSON(args ...any) (any, error) {
	return newEncoder("json", args...)
}

func CHART_LINE(args ...any) (any, error) {
	return newEncoder("echart.line", args...)
}

func CHART_SCATTER(args ...any) (any, error) {
	return newEncoder("echart.scatter", args...)
}

func CHART_BAR(args ...any) (any, error) {
	return newEncoder("echart.bar", args...)
}

func CHART_LINE3D(args ...any) (any, error) {
	return newEncoder("echart.line3d", args...)
}

func CHART_BAR3D(args ...any) (any, error) {
	return newEncoder("echart.bar3d", args...)
}

func CHART_SURFACE3D(args ...any) (any, error) {
	return newEncoder("echart.surface3d", args...)
}

func CHART_SCATTER3D(args ...any) (any, error) {
	return newEncoder("echart.scatter3d", args...)
}

// `sink=OUTPUT(encoder)`
func OUTPUT(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("f(OUTPUT) invalid number of args (n:%d)", len(args))
	}
	outstream, ok := args[0].(spec.OutputStream)
	if !ok {
		return nil, fmt.Errorf("f(OUTPUT) invalid output stream, but %T", args[0])
	}

	switch sink := args[1].(type) {
	case *Encoder:
		codecOpts := []opts.Option{
			opts.AssetHost("/web/echarts/"),
			opts.OutputStream(outstream),
		}
		codecOpts = append(codecOpts, sink.opts...)
		for i, arg := range args[2:] {
			if op, ok := arg.(opts.Option); !ok {
				return nil, fmt.Errorf("f(OUTPUT) invalid option %d %T", i, arg)
			} else {
				codecOpts = append(codecOpts, op)
			}
		}
		ret := codec.NewEncoder(sink.format, codecOpts...)
		return ret, nil
	case dbSink:
		sink.SetOutputStream(outstream)
		return sink, nil
	default:
		return nil, fmt.Errorf("f(OUTPUT) 1st arg must be Encoder in string, but %T", args[1])
	}
}
