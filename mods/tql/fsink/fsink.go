package fsink

import (
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql/fcom"
	"github.com/machbase/neo-server/mods/util"
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
	// csv, json options
	"heading":    sinkf_heading,
	"rownum":     sinkf_rownum,
	"timeformat": sinkf_timeformat,
	"tz":         sinkf_tz,
	"precision":  sinkf_precision,
	"columns":    sinkf_columns,
	// json options
	"transpose": sinkf_transpose,
	// chart options
	"xAxis":        sinkf_xAxis,
	"yAxis":        sinkf_yAxis,
	"zAxis":        sinkf_zAxis,
	"xaxis":        sinkf_xaxis, // deprecated
	"yaxis":        sinkf_yaxis, // deprecated
	"size":         sinkf_size,
	"theme":        sinkf_theme,
	"title":        sinkf_title,
	"subtitle":     sinkf_subtitle,
	"seriesLabels": sinkf_seriesLabels,
	"dataZoom":     sinkf_dataZoom,
	"visualMap":    sinkf_visualMap,
	"opacity":      sinkf_opacity,
	// sink functions
	"OUTPUT":          OUTPUT,
	"CSV":             CSV,
	"JSON":            JSON,
	"CHART_LINE":      CHART_LINE,
	"CHART_SCATTER":   CHART_SCATTER,
	"CHART_BAR":       CHART_BAR,
	"CHART_LINE3D":    CHART_LINE3D,
	"CHART_BAR3D":     CHART_BAR3D,
	"CHART_SURFACE3D": CHART_SURFACE3D,
	"CHART_SCATTER3D": CHART_SCATTER3D,
}

func init() {
	for k, v := range fcom.Functions {
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

func sinkf_tz(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(tz) invalid arg `tz(string)`")
	}
	if timezone, ok := args[0].(string); !ok {
		return nil, fmt.Errorf("f(tz) invalid arg `tz(string)`")
	} else {
		switch strings.ToUpper(timezone) {
		case "LOCAL":
			timezone = "Local"
		case "UTC":
			timezone = "UTC"
		}
		if timeLocation, err := time.LoadLocation(timezone); err != nil {
			timeLocation, err := util.GetTimeLocation(timezone)
			if err != nil {
				return nil, fmt.Errorf("f(tz) %s", err.Error())
			}
			return codec.TimeLocation(timeLocation), nil
		} else {
			return codec.TimeLocation(timeLocation), nil
		}
	}
}

func sinkf_timeformat(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(timeformat) invalid arg `timeformat(string)`")
	}
	if timeformat, ok := args[0].(string); ok {
		return codec.Timeformat(timeformat), nil
	} else {
		return nil, fmt.Errorf("f(timeformat) invalid arg `timeformat(string)`")
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

func sinkf_columns(args ...any) (any, error) {
	cols := []string{}
	for _, a := range args {
		if str, ok := a.(string); !ok {
			return nil, fmt.Errorf("f(columns) invalid arg `columns(string...)`")
		} else {
			cols = append(cols, str)
		}
	}
	return codec.Columns(cols, []string{}), nil
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

func sinkf_transpose(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(transpose) invalid arg `transpose(bool)`")
	}
	if flag, ok := args[0].(bool); !ok {
		return nil, fmt.Errorf("f(transpose) invalid arg `transpose(bool)`")
	} else {
		return codec.Transpose(flag), nil
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
		return nil, fmt.Errorf("f(size) invalid width, should be string, but '%T'", args[0])
	}
	height, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("f(size) invalid height, should be string, but '%T'", args[1])
	}

	return codec.Size(width, height), nil
}

func sinkf_title(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(title) invalid arg `title(string)`")
	}
	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("f(title) invalid title, should be string, but '%T'", args[0])
	}
	return codec.Title(str), nil
}

func sinkf_subtitle(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(subtitle) invalid arg `subtitle(string)`")
	}
	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("f(subtitle) invalid title, should be string, but '%T'", args[0])
	}
	return codec.Subtitle(str), nil
}

func sinkf_theme(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(theme) invalid arg `theme(string)`")
	}
	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("f(theme) invalid theme, should be string, but '%T'", args[0])
	}
	return codec.Theme(str), nil
}

// `series('value', 'rms-value')`
func sinkf_seriesLabels(args ...any) (any, error) {
	labels := []string{}
	for _, a := range args {
		if str, ok := a.(string); !ok {
			return nil, fmt.Errorf("f(series) invalid arg `series(string...)`")
		} else {
			labels = append(labels, str)
		}
	}

	return codec.Series(labels...), nil
}

func sinkf_dataZoom(args ...any) (any, error) {
	var typ string
	var start, end float32
	if len(args) != 3 {
		return nil, fmt.Errorf("f(dataZoom) invalid arg, `dataZoom(type, start, end)`, but (n:%d)", len(args))
	}
	if s, ok := args[0].(string); ok {
		typ = s
	} else {
		typ = "slider"
	}
	if d, ok := args[1].(float64); ok {
		start = float32(d)
	}
	if d, ok := args[2].(float64); ok {
		end = float32(d)
	}
	return codec.DataZoom(typ, start, end), nil
}

func sinkf_visualMap(args ...any) (any, error) {
	var minValue, maxValue float64
	if len(args) != 2 {
		return nil, fmt.Errorf("f(visualMap) invalid arg, `visualMap(minValue, maxValue)`, but (n:%d)", len(args))
	}
	if d, ok := args[0].(float64); ok {
		minValue = d
	}
	if d, ok := args[1].(float64); ok {
		maxValue = d
	}
	return codec.VisualMap(minValue, maxValue), nil
}

func sinkf_opacity(args ...any) (any, error) {
	var value float64
	if len(args) != 1 {
		return nil, fmt.Errorf("f(opacity) invalid arg, `opacity(opacity)`, but (n:%d)", len(args))
	}
	if d, ok := args[0].(float64); ok {
		value = d
	}
	return codec.Opacity(value), nil
}

func availableAxisType(typ string) bool {
	switch typ {
	case "time":
		return true
	case "value":
		return true
	default:
		return false
	}
}

func sinkf_xaxis(args ...any) (any, error) {
	fmt.Println("WARNING, 'xaxis()' is deprecated, use 'xAxis()' instead.!!!")
	return sinkf_xAxis(args...)
}

func sinkf_xAxis(args ...any) (any, error) {
	idx := 0
	label := "x"
	typ := "value"
	if len(args) >= 1 {
		if d, ok := args[0].(float64); !ok {
			return nil, fmt.Errorf("f(xAxis) invalid index, should be int, but '%T'", args[0])
		} else {
			idx = int(d)
		}
	}
	if len(args) >= 2 {
		if s, ok := args[1].(string); !ok {
			return nil, fmt.Errorf("f(xAxis) invalid label, should be string, but '%T'", args[1])
		} else {
			label = s
		}
	}
	if len(args) >= 3 {
		if s, ok := args[2].(string); !ok {
			return nil, fmt.Errorf("f(xAxis) invalid type, should be string, but '%T'", args[2])
		} else {
			if availableAxisType(s) {
				typ = s
			} else {
				return nil, fmt.Errorf("f(xAxis) invalid axis type, '%s'", s)
			}
		}
	}
	return codec.XAxis(idx, label, typ), nil
}

func sinkf_yaxis(args ...any) (any, error) {
	fmt.Println("WARNING, 'yaxis()' is deprecated, use 'yAxis()' instead.!!!")
	return sinkf_yAxis(args...)
}

func sinkf_yAxis(args ...any) (any, error) {
	idx := 0
	label := "y"
	typ := "value"
	if len(args) >= 1 {
		if d, ok := args[0].(float64); !ok {
			return nil, fmt.Errorf("f(yAxis) invalid index, should be int, but '%T'", args[0])
		} else {
			idx = int(d)
		}
	}
	if len(args) == 2 {
		if s, ok := args[1].(string); !ok {
			return nil, fmt.Errorf("f(yAxis) invalid label, should be string, but '%T'`", args[1])
		} else {
			label = s
		}
	}
	if len(args) >= 3 {
		if s, ok := args[2].(string); !ok {
			return nil, fmt.Errorf("f(yAxis) invalid type, should be string, but '%T'", args[2])
		} else {
			if availableAxisType(s) {
				typ = s
			} else {
				return nil, fmt.Errorf("f(yAxis) invalid axis type, '%s'", s)
			}
		}
	}
	return codec.YAxis(idx, label, typ), nil
}

func sinkf_zAxis(args ...any) (any, error) {
	idx := 0
	label := "z"
	typ := "value"
	if len(args) >= 1 {
		if d, ok := args[0].(float64); !ok {
			return nil, fmt.Errorf("f(zAxis) invalid index, should be int, but '%T'", args[0])
		} else {
			idx = int(d)
		}
	}
	if len(args) == 2 {
		if s, ok := args[1].(string); !ok {
			return nil, fmt.Errorf("f(zAxis) invalid label, should be string, but '%T'`", args[1])
		} else {
			label = s
		}
	}
	if len(args) >= 3 {
		if s, ok := args[2].(string); !ok {
			return nil, fmt.Errorf("f(zAxis) invalid type, should be string, but '%T'", args[2])
		} else {
			if availableAxisType(s) {
				typ = s
			} else {
				return nil, fmt.Errorf("f(yAxis) invalid axis type, '%s'", s)
			}
		}
	}
	return codec.ZAxis(idx, label, typ), nil
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
