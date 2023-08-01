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
	"github.com/machbase/neo-server/mods/tql/maps"
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
	"OUTPUT": OUTPUT,
	"INSERT": INSERT,
	"APPEND": APPEND,
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
	case *maps.Encoder:
		ret := sink.New(outstream)
		return ret, nil
	case dbSink:
		sink.SetOutputStream(outstream)
		return sink, nil
	default:
		return nil, fmt.Errorf("f(OUTPUT) 1st arg must be Encoder in string, but %T", args[1])
	}
}
