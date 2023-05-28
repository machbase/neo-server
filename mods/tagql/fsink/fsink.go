package fsink

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream/spec"
)

var Functions = map[string]expression.Function{
	"heading":    sinkf_heading,
	"rownum":     sinkf_rownum,
	"timeformat": sinkf_timeformat,
	"precision":  sinkf_precision,
	"size":       sinkf_size,
	"theme":      sinkf_theme,
	"title":      sinkf_title,
	"subtitle":   sinkf_subtitle,
	"series":     sinkf_series,
	"OUTPUT":     sinkf_OUTPUT,
}

func NormalizeSinkFuncExpr(expr string) string {
	expr = strings.ReplaceAll(expr, "OUTPUT(", "OUTPUT(outstream,")
	expr = strings.ReplaceAll(expr, "outputstream,)", "outputstream)")
	return expr
}

func sinkf_timeformat(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(timeformat) invalid arg `timeformat(string)`")
	}
	if timeformat, ok := args[0].(string); !ok {
		return nil, fmt.Errorf("f(timeformat) invalid arg `timeformat(string)`")
	} else {
		return codec.TimeFormat(timeformat), nil
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

// `sink=OUTPUT(format, opts...)`
func sinkf_OUTPUT(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("f(OUTPUT) invalid number of args (n:%d)", len(args))
	}
	outstream, ok := args[0].(spec.OutputStream)
	if !ok {
		return nil, fmt.Errorf("f(OUTPUT) invalid output stream, but %T", args[0])
	}

	format, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("f(OUTPUT) 1st arg must be format in string, but %T", args[1])
	}

	opts := []codec.Option{
		codec.OutputStream(outstream),
	}
	for i, arg := range args[2:] {
		if op, ok := arg.(codec.Option); !ok {
			return nil, fmt.Errorf("f(OUTPUT) invalid option %d %T", i, arg)
		} else {
			opts = append(opts, op)
		}
	}
	encoder := codec.NewEncoder(format, opts...)
	return encoder, nil
}
