package fsink

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream/spec"
)

type Context struct {
	Output spec.OutputStream
	Params map[string][]string
}

func (ctx *Context) Get(name string) (any, error) {
	if name == "outstream" {
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

var functions = map[string]expression.Function{
	"heading":         sinkf_heading,
	"rownum":          sinkf_rownum,
	"timeformat":      sinkf_timeformat,
	"precision":       sinkf_precision,
	"size":            sinkf_size,
	"theme":           sinkf_theme,
	"title":           sinkf_title,
	"subtitle":        sinkf_subtitle,
	"series":          sinkf_series,
	"OUTPUT":          OUTPUT,
	"CSV":             CSV,
	"JSON":            JSON,
	"CHART_LINE":      CHART_LINE,
	"CHART_LINE3D":    CHART_LINE3D,
	"CHART_BAR3D":     CHART_BAR3D,
	"CHART_SURFACE3D": CHART_SURFACE3D,
	"CHART_SCATTER3D": CHART_SCATTER3D,
}

func sinkf_timeformat(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(timeformat) invalid arg `timeformat(string)`")
	}
	if timeformat, ok := args[0].(string); !ok {
		return nil, fmt.Errorf("f(timeformat) invalid arg `timeformat(string)`")
	} else {
		return codec.Timeformat(timeformat), nil
	}
}

func sinkf_heading(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(heading) invalid arg `heading(bool)`")
	}
	if flag, ok := args[0].(bool); !ok {
		return nil, fmt.Errorf("f(heading) invalid arg `heading(bool)`")
	} else {
		return codec.Heading(flag), nil
	}
}

func sinkf_rownum(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(rownum) invalid arg `rownum(bool)`")
	}
	if flag, ok := args[0].(bool); !ok {
		return nil, fmt.Errorf("f(rownum) invalid arg `rownum(bool)`")
	} else {
		return codec.Rownum(flag), nil
	}
}

func sinkf_precision(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(precision) invalid arg `precision(int)`")
	}
	if prec, ok := args[0].(float64); !ok {
		return nil, fmt.Errorf("f(precision) invalid arg `precision(int)`")
	} else {
		return codec.Precision(int(prec)), nil
	}
}
func sinkf_size(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("f(size) invalid arg `size(width string, height string)`")
	}
	width, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("f(size) invalid width, should be string, but %T`", args[0])
	}
	height, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("f(size) invalid height, should be string, but %T`", args[1])
	}

	return codec.Size(width, height), nil
}

func sinkf_title(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(title) invalid arg `title(string)`")
	}
	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("f(title) invalid title, should be string, but %T`", args[0])
	}
	return codec.Title(str), nil
}

func sinkf_subtitle(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(subtitle) invalid arg `subtitle(string)`")
	}
	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("f(subtitle) invalid title, should be string, but %T`", args[0])
	}
	return codec.Subtitle(str), nil
}

func sinkf_theme(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(theme) invalid arg `theme(string)`")
	}
	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("f(theme) invalid theme, should be string, but %T`", args[0])
	}
	return codec.Theme(str), nil
}

// `series(1, 'rms-value')`
func sinkf_series(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("f(series) invalid arg `series(idx, label)`")
	}
	idx, ok := args[0].(float64)
	if !ok {
		return nil, fmt.Errorf("f(series) invalid index, should be int, but %T`", args[0])
	}
	label, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("f(series) invalid label, should be string, but %T`", args[1])
	}
	return codec.Series(int(idx), label), nil
}

type Encoder struct {
	format string
	opts   []codec.Option
}

func newEncoder(format string, args ...any) (*Encoder, error) {
	ret := &Encoder{
		format: format,
	}
	for _, arg := range args {
		if opt, ok := arg.(codec.Option); ok {
			ret.opts = append(ret.opts, opt)
		}
	}
	return ret, nil
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

func CHART_BOX(args ...any) (any, error) {
	return newEncoder("echart.box", args...)
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
		return nil, fmt.Errorf("f(PRINT) invalid number of args (n:%d)", len(args))
	}
	outstream, ok := args[0].(spec.OutputStream)
	if !ok {
		return nil, fmt.Errorf("f(PRINT) invalid output stream, but %T", args[0])
	}

	encoder, ok := args[1].(*Encoder)
	if !ok {
		return nil, fmt.Errorf("f(PRINT) 1st arg must be Encoder in string, but %T", args[1])
	}

	opts := append(encoder.opts, codec.OutputStream(outstream))
	for i, arg := range args[2:] {
		if op, ok := arg.(codec.Option); !ok {
			return nil, fmt.Errorf("f(PRINT) invalid option %d %T", i, arg)
		} else {
			opts = append(opts, op)
		}
	}
	ret := codec.NewEncoder(encoder.format, opts...)
	return ret, nil
}
