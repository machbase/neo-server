package codec

import (
	"time"

	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/transcoder"
)

type CanSetOutputStream interface {
	SetOutputStream(o spec.OutputStream)
}

func OutputStream(s spec.OutputStream) Option {
	return func(one any) {
		if o, ok := one.(CanSetOutputStream); ok {
			o.SetOutputStream(s)
		}
	}
}

type CanSetTimeformat interface {
	SetTimeformat(format string)
}

func Timeformat(f string) Option {
	return func(one any) {
		if o, ok := one.(CanSetTimeformat); ok {
			o.SetTimeformat(f)
		}
	}
}

type CanSetPrecision interface {
	SetPrecision(precision int)
}

func Precision(p int) Option {
	return func(one any) {
		if o, ok := one.(CanSetPrecision); ok {
			o.SetPrecision(p)
		}
	}
}

type CanSetRownum interface {
	SetRownum(show bool)
}

func Rownum(b bool) Option {
	return func(one any) {
		if o, ok := one.(CanSetRownum); ok {
			o.SetRownum(b)
		}
	}
}

type CanSetHeading interface {
	SetHeading(show bool)
}

func Heading(b bool) Option {
	return func(one any) {
		if o, ok := one.(CanSetHeading); ok {
			o.SetHeading(b)
		}
	}
}

type CanSetTimeLocation interface {
	SetTimeLocation(tz *time.Location)
}

func TimeLocation(tz *time.Location) Option {
	return func(one any) {
		if o, ok := one.(CanSetTimeLocation); ok {
			o.SetTimeLocation(tz)
		}
	}
}

type CanSetTranspose interface {
	SetTranspose(bool)
}

func Transpose(flag bool) Option {
	return func(one any) {
		if o, ok := one.(CanSetTranspose); ok {
			o.SetTranspose(flag)
		}
	}
}

type CanSetAssetHost interface {
	SetAssetHost(string)
}

func AssetHost(path string) Option {
	return func(one any) {
		if o, ok := one.(CanSetAssetHost); ok {
			o.SetAssetHost(path)
		}
	}
}

type CanSetTitle interface {
	SetTitle(title string)
}

func Title(title string) Option {
	return func(one any) {
		if o, ok := one.(CanSetTitle); ok {
			o.SetTitle(title)
		}
	}
}

type CanSetSubtitle interface {
	SetSubtitle(subtitle string)
}

func Subtitle(subtitle string) Option {
	return func(one any) {
		if o, ok := one.(CanSetSubtitle); ok {
			o.SetSubtitle(subtitle)
		}
	}
}

type CanSetChartJson interface {
	SetChartJson(flag bool)
}

func ChartJson(flag bool) Option {
	return func(one any) {
		if o, ok := one.(CanSetChartJson); ok {
			o.SetChartJson(flag)
		}
	}
}

type CanSetXAxis interface {
	SetXAxis(int, string, string)
}

func XAxis(idx int, label string, typ string) Option {
	return func(one any) {
		if o, ok := one.(CanSetXAxis); ok {
			o.SetXAxis(idx, label, typ)
		}
	}
}

type CanSetYAxis interface {
	SetYAxis(int, string, string)
}

func YAxis(idx int, label string, typ string) Option {
	return func(one any) {
		if o, ok := one.(CanSetYAxis); ok {
			o.SetYAxis(idx, label, typ)
		}
	}
}

type CanSetZAxis interface {
	SetZAxis(int, string, string)
}

func ZAxis(idx int, label string, typ string) Option {
	return func(one any) {
		if o, ok := one.(CanSetZAxis); ok {
			o.SetZAxis(idx, label, typ)
		}
	}
}

type CanSetAutoRotate interface {
	SetAutoRotate(float64)
}

func AutoRotate(speed float64) Option {
	return func(one any) {
		if o, ok := one.(CanSetAutoRotate); ok {
			o.SetAutoRotate(speed)
		}
	}
}

type CanSetShowGrid interface {
	SetShowGrid(bool)
}

func ShowGrid(flag bool) Option {
	return func(one any) {
		if o, ok := one.(CanSetShowGrid); ok {
			o.SetShowGrid(flag)
		}
	}
}

type CanSetGridSize interface {
	SetGridSize(whd ...float64)
}

func GridSize(whd ...float64) Option {
	return func(one any) {
		if o, ok := one.(CanSetGridSize); ok {
			o.SetGridSize(whd...)
		}
	}
}

type CanSetLineWidth interface {
	SetLineWidth(width float64)
}

func LineWidth(w float64) Option {
	return func(one any) {
		if o, ok := one.(CanSetLineWidth); ok {
			o.SetLineWidth(w)
		}
	}
}

type CanSetVisualMap interface {
	SetVisualMap(float64, float64)
}

func VisualMap(minValue float64, maxValue float64) Option {
	return func(one any) {
		if o, ok := one.(CanSetVisualMap); ok {
			o.SetVisualMap(minValue, maxValue)
		}
	}
}

type CanSetOpacity interface {
	SetOpacity(float64)
}

func Opacity(opacity float64) Option {
	return func(one any) {
		if o, ok := one.(CanSetOpacity); ok {
			o.SetOpacity(opacity)
		}
	}
}

type CanSetSize interface {
	SetSize(width, height string)
}

func Size(width string, height string) Option {
	return func(one any) {
		if o, ok := one.(CanSetSize); ok {
			o.SetSize(width, height)
		}
	}
}

type CanSetDataZoom interface {
	SetDataZoom(typ string, start, end float32)
}

func DataZoom(typ string, start, end float32) Option {
	return func(one any) {
		if o, ok := one.(CanSetDataZoom); ok {
			o.SetDataZoom(typ, start, end)
		}
	}
}

type CanSetTheme interface {
	SetTheme(theme string)
}

func Theme(theme string) Option {
	return func(one any) {
		if o, ok := one.(CanSetTheme); ok {
			o.SetTheme(theme)
		}
	}
}

type CanSetSeriesLabels interface {
	SetSeriesLabels(labels ...string)
}

func Series(labels ...string) Option {
	return func(one any) {
		if o, ok := one.(CanSetSeriesLabels); ok {
			o.SetSeriesLabels(labels...)
		}
	}
}

type CanSetDelimiter interface {
	SetDelimiter(comma string)
}

func Delimiter(delimiter string) Option {
	return func(one any) {
		if o, ok := one.(CanSetDelimiter); ok {
			o.SetDelimiter(delimiter)
		}
	}
}

// BOX only

type CanSetBoxStyle interface {
	SetBoxStyle(style string)
}

func BoxStyle(style string) Option {
	return func(one any) {
		if o, ok := one.(CanSetBoxStyle); ok {
			o.SetBoxStyle(style)
		}
	}
}

type CanSetBoxSeparateColumns interface {
	SetBoxSeparateColumns(flag bool)
}

func BoxSeparateColumns(flag bool) Option {
	return func(one any) {
		if o, ok := one.(CanSetBoxSeparateColumns); ok {
			o.SetBoxSeparateColumns(flag)
		}
	}
}

type CanSetBoxDrawBorder interface {
	SetBoxDrawBorder(flag bool)
}

func BoxDrawBorder(flag bool) Option {
	return func(one any) {
		if o, ok := one.(CanSetBoxDrawBorder); ok {
			o.SetBoxDrawBorder(flag)
		}
	}
}

// Decoder only
type CanSetInputStream interface {
	SetInputStream(in spec.InputStream)
}

func InputStream(in spec.InputStream) Option {
	return func(one any) {
		if o, ok := one.(CanSetInputStream); ok {
			o.SetInputStream(in)
		}
	}
}

type CanSetTable interface {
	SetTable(table string)
}

func Table(table string) Option {
	return func(one any) {
		if o, ok := one.(CanSetTable); ok {
			o.SetTable(table)
		}
	}
}

type CanSetTranscoder interface {
	SetTranscoder(trans transcoder.Transcoder)
}

func Transcoder(trans transcoder.Transcoder) Option {
	return func(one any) {
		if o, ok := one.(CanSetTranscoder); ok {
			o.SetTranscoder(trans)
		}
	}
}

type CanSetColumns interface {
	SetColumns(labels []string, types []string)
}

func Columns(labels []string, types []string) Option {
	return func(one any) {
		if o, ok := one.(CanSetColumns); ok {
			o.SetColumns(labels, types)
		}
	}
}

type CanSetHtmlRender interface {
	SetHtmlRender(bool)
}

func HtmlRender(flag bool) Option {
	return func(one any) {
		if o, ok := one.(CanSetHtmlRender); ok {
			o.SetHtmlRender(flag)
		}
	}
}

type CanSetBrief interface {
	SetBrief(int)
}

func Brief(count int) Option {
	return func(one any) {
		if o, ok := one.(CanSetBrief); ok {
			o.SetBrief(count)
		}
	}
}

type CanSetMarkAreaNameCoord interface {
	SetMarkAreaNameCoord(from any, to any, label string, color string, opacity float64)
}

func MarkAreaNameCoord(from any, to any, label string, color string, opacity float64) Option {
	return func(one any) {
		if o, ok := one.(CanSetMarkAreaNameCoord); ok {
			o.SetMarkAreaNameCoord(from, to, label, color, opacity)
		}
	}
}
