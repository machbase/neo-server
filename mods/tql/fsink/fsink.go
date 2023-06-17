package fsink

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql/conv"
	"github.com/machbase/neo-server/mods/tql/fcom"
	"github.com/machbase/neo-server/mods/util"
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
}

func Compile(code string, params map[string][]string, writer io.Writer) (Output, error) {
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
	"assetHost":    sinkf_assetHost,
	"title":        sinkf_title,
	"subtitle":     sinkf_subtitle,
	"seriesLabels": sinkf_seriesLabels,
	"dataZoom":     sinkf_dataZoom,
	"visualMap":    sinkf_visualMap,
	"opacity":      sinkf_opacity,
	"autoRotate":   sinkf_autoRotate,
	"showGrid":     sinkf_showGrid,
	"gridSize":     sinkf_gridSize,
	// db options
	"table": to_table,
	"tag":   to_tag,
	// sink functions
	"OUTPUT":          OUTPUT,
	"CSV":             CSV,
	"JSON":            JSON,
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
	if flag, err := conv.Bool(args, 0, "transpose", "boolean"); err != nil {
		return nil, err
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

func sinkf_assetHost(args ...any) (any, error) {
	if str, err := conv.String(args, 0, "assetHost", "string"); err != nil {
		return nil, err
	} else {
		return codec.AssetHost(str), nil
	}
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

func sinkf_autoRotate(args ...any) (any, error) {
	speed := 10.0
	if len(args) > 0 {
		if d, err := conv.Float64(args, 0, "autoRotate", "number"); err != nil {
			return nil, err
		} else {
			speed = d
		}
	}
	return codec.AutoRotate(speed), nil
}

func sinkf_showGrid(args ...any) (any, error) {
	flag, err := conv.Bool(args, 0, "showGrid", "boolean")
	if err != nil {
		return nil, err
	}
	// go-echarts bug? not working
	return codec.ShowGrid(flag), nil
}

func sinkf_gridSize(args ...any) (any, error) {
	whd := []float64{}
	for i := 0; i < 3 && i < len(args); i++ {
		if gs, err := conv.Float64(args, i, "gridSize", "number"); err != nil {
			return nil, err
		} else {
			whd = append(whd, gs)
		}
	}
	return codec.GridSize(whd...), nil
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
		return nil, fmt.Errorf("f(OUTPUT) invalid number of args (n:%d)", len(args))
	}
	outstream, ok := args[0].(spec.OutputStream)
	if !ok {
		return nil, fmt.Errorf("f(OUTPUT) invalid output stream, but %T", args[0])
	}

	switch sink := args[1].(type) {
	case *Encoder:
		opts := []codec.Option{
			codec.AssetHost("/web/echarts/"),
			codec.OutputStream(outstream),
		}
		opts = append(opts, sink.opts...)
		for i, arg := range args[2:] {
			if op, ok := arg.(codec.Option); !ok {
				return nil, fmt.Errorf("f(OUTPUT) invalid option %d %T", i, arg)
			} else {
				opts = append(opts, op)
			}
		}
		ret := codec.NewEncoder(sink.format, opts...)
		return ret, nil
	case dbSink:
		sink.SetOutputStream(outstream)
		return sink, nil
	default:
		return nil, fmt.Errorf("f(OUTPUT) 1st arg must be Encoder in string, but %T", args[1])
	}
}
