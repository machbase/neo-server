//go:generate go run generate.go

package opts

// Code generated by go generate; DO NOT EDIT.

import (
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/transcoder"
	"time"
)

// SetAssetHost
//
//	mods/codec/internal/echart/echart.go:41:1
type CanSetAssetHost interface {
	SetAssetHost(path string)
}

func AssetHost(path string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetAssetHost); ok {
			_o.SetAssetHost(path)
		}
	}
}

// SetAutoRotate
//
//	mods/codec/internal/echart/echart_3d.go:83:1
type CanSetAutoRotate interface {
	SetAutoRotate(speed float64)
}

func AutoRotate(speed float64) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetAutoRotate); ok {
			_o.SetAutoRotate(speed)
		}
	}
}

// SetBoxDrawBorder
//
//	mods/codec/internal/box/box_encode.go:76:1
type CanSetBoxDrawBorder interface {
	SetBoxDrawBorder(flag bool)
}

func BoxDrawBorder(flag bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetBoxDrawBorder); ok {
			_o.SetBoxDrawBorder(flag)
		}
	}
}

// SetBoxSeparateColumns
//
//	mods/codec/internal/box/box_encode.go:72:1
type CanSetBoxSeparateColumns interface {
	SetBoxSeparateColumns(flag bool)
}

func BoxSeparateColumns(flag bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetBoxSeparateColumns); ok {
			_o.SetBoxSeparateColumns(flag)
		}
	}
}

// SetBoxStyle
//
//	mods/codec/internal/box/box_encode.go:68:1
type CanSetBoxStyle interface {
	SetBoxStyle(style string)
}

func BoxStyle(style string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetBoxStyle); ok {
			_o.SetBoxStyle(style)
		}
	}
}

// SetBrief
//
//	mods/codec/internal/markdown/md_encode.go:79:1
type CanSetBrief interface {
	SetBrief(flag bool)
}

func Brief(flag bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetBrief); ok {
			_o.SetBrief(flag)
		}
	}
}

// SetBriefCount
//
//	mods/codec/internal/markdown/md_encode.go:87:1
type CanSetBriefCount interface {
	SetBriefCount(count int)
}

func BriefCount(count int) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetBriefCount); ok {
			_o.SetBriefCount(count)
		}
	}
}

// SetChartJson
//
//	mods/codec/internal/echart/echart.go:45:1
type CanSetChartJson interface {
	SetChartJson(flag bool)
}

func ChartJson(flag bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetChartJson); ok {
			_o.SetChartJson(flag)
		}
	}
}

// SetColumnTypes
//
//	mods/codec/internal/csv/csv_decode.go:68:1
//	mods/codec/internal/json/json_decode.go:46:1
//	mods/codec/internal/json/json_encode.go:71:1
type CanSetColumnTypes interface {
	SetColumnTypes(types ...string)
}

func ColumnTypes(types ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetColumnTypes); ok {
			_o.SetColumnTypes(types...)
		}
	}
}

// SetColumns
//
//	mods/codec/internal/box/box_encode.go:80:1
//	mods/codec/internal/csv/csv_decode.go:64:1
//	mods/codec/internal/csv/csv_encode.go:78:1
//	mods/codec/internal/json/json_encode.go:67:1
//	mods/codec/internal/markdown/md_encode.go:71:1
type CanSetColumns interface {
	SetColumns(names ...string)
}

func Columns(names ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetColumns); ok {
			_o.SetColumns(names...)
		}
	}
}

// SetDataZoom
//
//	mods/codec/internal/echart/echart_2d.go:86:1
type CanSetDataZoom interface {
	SetDataZoom(typ string, start float32, end float32)
}

func DataZoom(typ string, start float32, end float32) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetDataZoom); ok {
			_o.SetDataZoom(typ, start, end)
		}
	}
}

// SetDelimiter
//
//	mods/codec/internal/csv/csv_decode.go:51:1
//	mods/codec/internal/csv/csv_encode.go:73:1
type CanSetDelimiter interface {
	SetDelimiter(delimiter string)
}

func Delimiter(delimiter string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetDelimiter); ok {
			_o.SetDelimiter(delimiter)
		}
	}
}

// SetGridSize
//
//	mods/codec/internal/echart/echart_3d.go:97:1
type CanSetGridSize interface {
	SetGridSize(args ...float64)
}

func GridSize(args ...float64) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetGridSize); ok {
			_o.SetGridSize(args...)
		}
	}
}

// SetHeading
//
//	mods/codec/internal/box/box_encode.go:64:1
//	mods/codec/internal/csv/csv_decode.go:47:1
//	mods/codec/internal/csv/csv_encode.go:69:1
//	mods/codec/internal/json/json_encode.go:63:1
type CanSetHeading interface {
	SetHeading(show bool)
}

func Heading(show bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetHeading); ok {
			_o.SetHeading(show)
		}
	}
}

// SetHtml
//
//	mods/codec/internal/markdown/md_encode.go:75:1
type CanSetHtml interface {
	SetHtml(flag bool)
}

func Html(flag bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetHtml); ok {
			_o.SetHtml(flag)
		}
	}
}

// SetInputStream
//
//	mods/codec/internal/csv/csv_decode.go:35:1
//	mods/codec/internal/json/json_decode.go:30:1
type CanSetInputStream interface {
	SetInputStream(in spec.InputStream)
}

func InputStream(in spec.InputStream) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetInputStream); ok {
			_o.SetInputStream(in)
		}
	}
}

// SetLineWidth
//
//	mods/codec/internal/echart/echart_3d.go:109:1
type CanSetLineWidth interface {
	SetLineWidth(width float64)
}

func LineWidth(width float64) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetLineWidth); ok {
			_o.SetLineWidth(width)
		}
	}
}

// SetMarkAreaNameCoord
//
//	mods/codec/internal/echart/echart_2d.go:96:1
type CanSetMarkAreaNameCoord interface {
	SetMarkAreaNameCoord(from any, to any, label string, color string, opacity float64)
}

func MarkAreaNameCoord(from any, to any, label string, color string, opacity float64) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetMarkAreaNameCoord); ok {
			_o.SetMarkAreaNameCoord(from, to, label, color, opacity)
		}
	}
}

// SetMarkLineXAxisCoord
//
//	mods/codec/internal/echart/echart_2d.go:106:1
type CanSetMarkLineXAxisCoord interface {
	SetMarkLineXAxisCoord(xaxis any, name string)
}

func MarkLineXAxisCoord(xaxis any, name string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetMarkLineXAxisCoord); ok {
			_o.SetMarkLineXAxisCoord(xaxis, name)
		}
	}
}

// SetMarkLineYAxisCoord
//
//	mods/codec/internal/echart/echart_2d.go:113:1
type CanSetMarkLineYAxisCoord interface {
	SetMarkLineYAxisCoord(yaxis any, name string)
}

func MarkLineYAxisCoord(yaxis any, name string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetMarkLineYAxisCoord); ok {
			_o.SetMarkLineYAxisCoord(yaxis, name)
		}
	}
}

// SetOpacity
//
//	mods/codec/internal/echart/echart_3d.go:105:1
type CanSetOpacity interface {
	SetOpacity(opacity float64)
}

func Opacity(opacity float64) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetOpacity); ok {
			_o.SetOpacity(opacity)
		}
	}
}

// SetOutputStream
//
//	mods/codec/internal/box/box_encode.go:44:1
//	mods/codec/internal/csv/csv_encode.go:49:1
//	mods/codec/internal/echart/echart.go:20:1
//	mods/codec/internal/json/json_encode.go:43:1
//	mods/codec/internal/markdown/md_encode.go:51:1
type CanSetOutputStream interface {
	SetOutputStream(o spec.OutputStream)
}

func OutputStream(o spec.OutputStream) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetOutputStream); ok {
			_o.SetOutputStream(o)
		}
	}
}

// SetPrecision
//
//	mods/codec/internal/box/box_encode.go:56:1
//	mods/codec/internal/csv/csv_encode.go:61:1
//	mods/codec/internal/json/json_encode.go:55:1
//	mods/codec/internal/markdown/md_encode.go:63:1
type CanSetPrecision interface {
	SetPrecision(precision int)
}

func Precision(precision int) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetPrecision); ok {
			_o.SetPrecision(precision)
		}
	}
}

// SetRownum
//
//	mods/codec/internal/box/box_encode.go:60:1
//	mods/codec/internal/csv/csv_encode.go:65:1
//	mods/codec/internal/json/json_encode.go:59:1
//	mods/codec/internal/markdown/md_encode.go:67:1
type CanSetRownum interface {
	SetRownum(show bool)
}

func Rownum(show bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetRownum); ok {
			_o.SetRownum(show)
		}
	}
}

// SetSeriesLabels
//
//	mods/codec/internal/echart/echart_2d.go:92:1
type CanSetSeriesLabels interface {
	SetSeriesLabels(labels ...string)
}

func SeriesLabels(labels ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetSeriesLabels); ok {
			_o.SetSeriesLabels(labels...)
		}
	}
}

// SetShowGrid
//
//	mods/codec/internal/echart/echart_3d.go:93:1
type CanSetShowGrid interface {
	SetShowGrid(flag bool)
}

func ShowGrid(flag bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetShowGrid); ok {
			_o.SetShowGrid(flag)
		}
	}
}

// SetSize
//
//	mods/codec/internal/echart/echart.go:24:1
type CanSetSize interface {
	SetSize(width string, height string)
}

func Size(width string, height string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetSize); ok {
			_o.SetSize(width, height)
		}
	}
}

// SetSubstituteNull
//
//	mods/codec/internal/csv/csv_encode.go:82:1
type CanSetSubstituteNull interface {
	SetSubstituteNull(nullString string)
}

func SubstituteNull(nullString string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetSubstituteNull); ok {
			_o.SetSubstituteNull(nullString)
		}
	}
}

// SetSubtitle
//
//	mods/codec/internal/echart/echart.go:37:1
type CanSetSubtitle interface {
	SetSubtitle(subtitle string)
}

func Subtitle(subtitle string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetSubtitle); ok {
			_o.SetSubtitle(subtitle)
		}
	}
}

// SetTableName
//
//	mods/codec/internal/csv/csv_decode.go:56:1
//	mods/codec/internal/json/json_decode.go:42:1
type CanSetTableName interface {
	SetTableName(tableName string)
}

func TableName(tableName string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTableName); ok {
			_o.SetTableName(tableName)
		}
	}
}

// SetTheme
//
//	mods/codec/internal/echart/echart.go:29:1
type CanSetTheme interface {
	SetTheme(theme string)
}

func Theme(theme string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTheme); ok {
			_o.SetTheme(theme)
		}
	}
}

// SetTimeLocation
//
//	mods/codec/internal/box/box_encode.go:52:1
//	mods/codec/internal/csv/csv_decode.go:43:1
//	mods/codec/internal/csv/csv_encode.go:57:1
//	mods/codec/internal/echart/echart_2d.go:79:1
//	mods/codec/internal/json/json_decode.go:38:1
//	mods/codec/internal/json/json_encode.go:51:1
//	mods/codec/internal/markdown/md_encode.go:59:1
type CanSetTimeLocation interface {
	SetTimeLocation(tz *time.Location)
}

func TimeLocation(tz *time.Location) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTimeLocation); ok {
			_o.SetTimeLocation(tz)
		}
	}
}

// SetTimeformat
//
//	mods/codec/internal/box/box_encode.go:48:1
//	mods/codec/internal/csv/csv_decode.go:39:1
//	mods/codec/internal/csv/csv_encode.go:53:1
//	mods/codec/internal/echart/echart_2d.go:72:1
//	mods/codec/internal/json/json_decode.go:34:1
//	mods/codec/internal/json/json_encode.go:47:1
//	mods/codec/internal/markdown/md_encode.go:55:1
type CanSetTimeformat interface {
	SetTimeformat(format string)
}

func Timeformat(format string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTimeformat); ok {
			_o.SetTimeformat(format)
		}
	}
}

// SetTitle
//
//	mods/codec/internal/echart/echart.go:33:1
type CanSetTitle interface {
	SetTitle(title string)
}

func Title(title string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTitle); ok {
			_o.SetTitle(title)
		}
	}
}

// SetTranscoder
//
//	mods/codec/internal/csv/csv_decode.go:60:1
type CanSetTranscoder interface {
	SetTranscoder(trans transcoder.Transcoder)
}

func Transcoder(trans transcoder.Transcoder) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTranscoder); ok {
			_o.SetTranscoder(trans)
		}
	}
}

// SetTranspose
//
//	mods/codec/internal/json/json_encode.go:75:1
type CanSetTranspose interface {
	SetTranspose(flag bool)
}

func Transpose(flag bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTranspose); ok {
			_o.SetTranspose(flag)
		}
	}
}

// SetVisualMap
//
//	mods/codec/internal/echart/echart_3d.go:77:1
type CanSetVisualMap interface {
	SetVisualMap(minValue float64, maxValue float64)
}

func VisualMap(minValue float64, maxValue float64) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetVisualMap); ok {
			_o.SetVisualMap(minValue, maxValue)
		}
	}
}

// SetXAxis
//
//	mods/codec/internal/echart/echart_2d.go:56:1
//	mods/codec/internal/echart/echart_3d.go:53:1
type CanSetXAxis interface {
	SetXAxis(idx int, label string, typ ...string)
}

func XAxis(idx int, label string, typ ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetXAxis); ok {
			_o.SetXAxis(idx, label, typ...)
		}
	}
}

// SetYAxis
//
//	mods/codec/internal/echart/echart_2d.go:64:1
//	mods/codec/internal/echart/echart_3d.go:61:1
type CanSetYAxis interface {
	SetYAxis(idx int, label string, typ ...string)
}

func YAxis(idx int, label string, typ ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetYAxis); ok {
			_o.SetYAxis(idx, label, typ...)
		}
	}
}

// SetZAxis
//
//	mods/codec/internal/echart/echart_3d.go:69:1
type CanSetZAxis interface {
	SetZAxis(idx int, label string, typ ...string)
}

func ZAxis(idx int, label string, typ ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetZAxis); ok {
			_o.SetZAxis(idx, label, typ...)
		}
	}
}
