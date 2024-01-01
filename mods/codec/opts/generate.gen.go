//go:generate go run generate.go

package opts

// Code generated by go generate; DO NOT EDIT.

import (
	"github.com/machbase/neo-server/mods/codec/facility"
	"github.com/machbase/neo-server/mods/nums"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/transcoder"
	"golang.org/x/text/encoding"
	"time"
)

// SetAutoRotate
//
//	mods/codec/internal/chart/chartcompat.go:234:1
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
//	mods/codec/internal/box/box_encode.go:81:1
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
//	mods/codec/internal/box/box_encode.go:77:1
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
//	mods/codec/internal/box/box_encode.go:73:1
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
//	mods/codec/internal/markdown/md_encode.go:84:1
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
//	mods/codec/internal/markdown/md_encode.go:92:1
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

// SetCharsetEncoding
//
//	mods/codec/internal/csv/csv_decode.go:41:1
type CanSetCharsetEncoding interface {
	SetCharsetEncoding(charset encoding.Encoding)
}

func CharsetEncoding(charset encoding.Encoding) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetCharsetEncoding); ok {
			_o.SetCharsetEncoding(charset)
		}
	}
}

// SetChartAssets
//
//	mods/codec/internal/chart/chart.go:106:1
type CanSetChartAssets interface {
	SetChartAssets(args ...string)
}

func ChartAssets(args ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetChartAssets); ok {
			_o.SetChartAssets(args...)
		}
	}
}

// SetChartDispatchAction
//
//	mods/codec/internal/chart/chart.go:124:1
type CanSetChartDispatchAction interface {
	SetChartDispatchAction(action string)
}

func ChartDispatchAction(action string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetChartDispatchAction); ok {
			_o.SetChartDispatchAction(action)
		}
	}
}

// SetChartId
//
//	mods/codec/internal/chart/chart.go:77:1
type CanSetChartId interface {
	SetChartId(id string)
}

func ChartId(id string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetChartId); ok {
			_o.SetChartId(id)
		}
	}
}

// SetChartJSCode
//
//	mods/codec/internal/chart/chart.go:116:1
type CanSetChartJSCode interface {
	SetChartJSCode(js string)
}

func ChartJSCode(js string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetChartJSCode); ok {
			_o.SetChartJSCode(js)
		}
	}
}

// SetChartJson
//
//	mods/codec/internal/chart/chart.go:90:1
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

// SetChartOption
//
//	mods/codec/internal/chart/chart.go:94:1
type CanSetChartOption interface {
	SetChartOption(opt string)
}

func ChartOption(opt string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetChartOption); ok {
			_o.SetChartOption(opt)
		}
	}
}

// SetColumnTypes
//
//	mods/codec/internal/csv/csv_decode.go:79:1
//	mods/codec/internal/json/json_decode.go:46:1
//	mods/codec/internal/json/json_encode.go:76:1
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
//	mods/codec/internal/box/box_encode.go:85:1
//	mods/codec/internal/csv/csv_decode.go:75:1
//	mods/codec/internal/csv/csv_encode.go:84:1
//	mods/codec/internal/json/json_encode.go:72:1
//	mods/codec/internal/markdown/md_encode.go:60:1
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
//	mods/codec/internal/chart/chartcompat.go:88:1
type CanSetDataZoom interface {
	SetDataZoom(typ string, minPercentage float32, maxPercentage float32)
}

func DataZoom(typ string, minPercentage float32, maxPercentage float32) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetDataZoom); ok {
			_o.SetDataZoom(typ, minPercentage, maxPercentage)
		}
	}
}

// SetDelimiter
//
//	mods/codec/internal/csv/csv_decode.go:62:1
//	mods/codec/internal/csv/csv_encode.go:79:1
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

// SetGeoMapJson
//
//	mods/codec/internal/geomap/geomap.go:121:1
type CanSetGeoMapJson interface {
	SetGeoMapJson(flag bool)
}

func GeoMapJson(flag bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetGeoMapJson); ok {
			_o.SetGeoMapJson(flag)
		}
	}
}

// SetGlobalOptions
//
//	mods/codec/internal/chart/chartcompat.go:76:1
type CanSetGlobalOptions interface {
	SetGlobalOptions(opt string)
}

func GlobalOptions(opt string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetGlobalOptions); ok {
			_o.SetGlobalOptions(opt)
		}
	}
}

// SetGridSize
//
//	mods/codec/internal/chart/chartcompat.go:219:1
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

// SetHeader
//
//	mods/codec/internal/box/box_encode.go:65:1
//	mods/codec/internal/csv/csv_decode.go:58:1
//	mods/codec/internal/csv/csv_encode.go:75:1
//	mods/codec/internal/json/json_encode.go:64:1
type CanSetHeader interface {
	SetHeader(show bool)
}

func Header(show bool) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetHeader); ok {
			_o.SetHeader(show)
		}
	}
}

// SetHeading
//
//	mods/codec/internal/box/box_encode.go:69:1
//	mods/codec/internal/csv/csv_decode.go:54:1
//	mods/codec/internal/csv/csv_encode.go:71:1
//	mods/codec/internal/json/json_encode.go:68:1
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
//	mods/codec/internal/markdown/md_encode.go:80:1
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

// SetIcon
//
//	mods/codec/internal/geomap/geomap.go:144:1
type CanSetIcon interface {
	SetIcon(name string, opt string)
}

func Icon(name string, opt string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetIcon); ok {
			_o.SetIcon(name, opt)
		}
	}
}

// SetInitialLocation
//
//	mods/codec/internal/geomap/geomap.go:100:1
type CanSetInitialLocation interface {
	SetInitialLocation(latlon *nums.LatLon, zoomLevel int)
}

func InitialLocation(latlon *nums.LatLon, zoomLevel int) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetInitialLocation); ok {
			_o.SetInitialLocation(latlon, zoomLevel)
		}
	}
}

// SetInputStream
//
//	mods/codec/internal/csv/csv_decode.go:37:1
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

// SetLayer
//
//	mods/codec/internal/geomap/geomap.go:140:1
type CanSetLayer interface {
	SetLayer(obj nums.Geography)
}

func Layer(obj nums.Geography) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetLayer); ok {
			_o.SetLayer(obj)
		}
	}
}

// SetLineWidth
//
//	mods/codec/internal/chart/chartcompat.go:225:1
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

// SetLogger
//
//	mods/codec/internal/chart/chart.go:65:1
//	mods/codec/internal/geomap/geomap.go:69:1
//	mods/codec/internal/markdown/md_encode.go:52:1
type CanSetLogger interface {
	SetLogger(l facility.Logger)
}

func Logger(l facility.Logger) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetLogger); ok {
			_o.SetLogger(l)
		}
	}
}

// SetMapAssets
//
//	mods/codec/internal/geomap/geomap.go:90:1
type CanSetMapAssets interface {
	SetMapAssets(args ...string)
}

func MapAssets(args ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetMapAssets); ok {
			_o.SetMapAssets(args...)
		}
	}
}

// SetMapId
//
//	mods/codec/internal/geomap/geomap.go:81:1
type CanSetMapId interface {
	SetMapId(id string)
}

func MapId(id string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetMapId); ok {
			_o.SetMapId(id)
		}
	}
}

// SetMarkAreaNameCoord
//
//	mods/codec/internal/chart/chartcompat.go:265:1
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
//	mods/codec/internal/chart/chartcompat.go:273:1
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
//	mods/codec/internal/chart/chartcompat.go:279:1
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
//	mods/codec/internal/chart/chartcompat.go:229:1
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
//	mods/codec/internal/box/box_encode.go:45:1
//	mods/codec/internal/chart/chart.go:73:1
//	mods/codec/internal/csv/csv_encode.go:50:1
//	mods/codec/internal/geomap/geomap.go:77:1
//	mods/codec/internal/json/json_encode.go:44:1
//	mods/codec/internal/markdown/md_encode.go:56:1
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

// SetPlugins
//
//	mods/codec/internal/chart/chart.go:102:1
type CanSetPlugins interface {
	SetPlugins(plugins ...string)
}

func Plugins(plugins ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetPlugins); ok {
			_o.SetPlugins(plugins...)
		}
	}
}

// SetPointStyle
//
//	mods/codec/internal/geomap/geomap.go:169:1
type CanSetPointStyle interface {
	SetPointStyle(name string, typ string, opt string)
}

func PointStyle(name string, typ string, opt string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetPointStyle); ok {
			_o.SetPointStyle(name, typ, opt)
		}
	}
}

// SetPrecision
//
//	mods/codec/internal/box/box_encode.go:57:1
//	mods/codec/internal/csv/csv_encode.go:62:1
//	mods/codec/internal/json/json_encode.go:56:1
//	mods/codec/internal/markdown/md_encode.go:72:1
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
//	mods/codec/internal/box/box_encode.go:61:1
//	mods/codec/internal/csv/csv_encode.go:66:1
//	mods/codec/internal/json/json_encode.go:60:1
//	mods/codec/internal/markdown/md_encode.go:76:1
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
//	mods/codec/internal/chart/chartcompat.go:84:1
type CanSetSeriesLabels interface {
	SetSeriesLabels(args ...string)
}

func SeriesLabels(args ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetSeriesLabels); ok {
			_o.SetSeriesLabels(args...)
		}
	}
}

// SetSize
//
//	mods/codec/internal/chart/chart.go:81:1
//	mods/codec/internal/geomap/geomap.go:85:1
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
//	mods/codec/internal/csv/csv_encode.go:88:1
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
//	mods/codec/internal/chart/chartcompat.go:215:1
type CanSetSubtitle interface {
	SetSubtitle(str string)
}

func Subtitle(str string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetSubtitle); ok {
			_o.SetSubtitle(str)
		}
	}
}

// SetTableName
//
//	mods/codec/internal/csv/csv_decode.go:67:1
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
//	mods/codec/internal/chart/chart.go:86:1
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

// SetTileGrayscale
//
//	mods/codec/internal/geomap/geomap.go:125:1
type CanSetTileGrayscale interface {
	SetTileGrayscale(grayscale float64)
}

func TileGrayscale(grayscale float64) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTileGrayscale); ok {
			_o.SetTileGrayscale(grayscale)
		}
	}
}

// SetTileOption
//
//	mods/codec/internal/geomap/geomap.go:109:1
type CanSetTileOption interface {
	SetTileOption(opt string)
}

func TileOption(opt string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTileOption); ok {
			_o.SetTileOption(opt)
		}
	}
}

// SetTileTemplate
//
//	mods/codec/internal/geomap/geomap.go:105:1
type CanSetTileTemplate interface {
	SetTileTemplate(url string)
}

func TileTemplate(url string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTileTemplate); ok {
			_o.SetTileTemplate(url)
		}
	}
}

// SetTimeLocation
//
//	mods/codec/internal/box/box_encode.go:53:1
//	mods/codec/internal/csv/csv_decode.go:49:1
//	mods/codec/internal/csv/csv_encode.go:58:1
//	mods/codec/internal/json/json_decode.go:38:1
//	mods/codec/internal/json/json_encode.go:52:1
//	mods/codec/internal/markdown/md_encode.go:68:1
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
//	mods/codec/internal/box/box_encode.go:49:1
//	mods/codec/internal/chart/chartcompat.go:285:1
//	mods/codec/internal/csv/csv_decode.go:45:1
//	mods/codec/internal/csv/csv_encode.go:54:1
//	mods/codec/internal/json/json_decode.go:34:1
//	mods/codec/internal/json/json_encode.go:48:1
//	mods/codec/internal/markdown/md_encode.go:64:1
type CanSetTimeformat interface {
	SetTimeformat(f string)
}

func Timeformat(f string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTimeformat); ok {
			_o.SetTimeformat(f)
		}
	}
}

// SetTitle
//
//	mods/codec/internal/chart/chartcompat.go:211:1
type CanSetTitle interface {
	SetTitle(str string)
}

func Title(str string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetTitle); ok {
			_o.SetTitle(str)
		}
	}
}

// SetToolboxDataView
//
//	mods/codec/internal/chart/chartcompat.go:261:1
type CanSetToolboxDataView interface {
	SetToolboxDataView()
}

func ToolboxDataView() Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetToolboxDataView); ok {
			_o.SetToolboxDataView()
		}
	}
}

// SetToolboxDataZoom
//
//	mods/codec/internal/chart/chartcompat.go:257:1
type CanSetToolboxDataZoom interface {
	SetToolboxDataZoom()
}

func ToolboxDataZoom() Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetToolboxDataZoom); ok {
			_o.SetToolboxDataZoom()
		}
	}
}

// SetToolboxSaveAsImage
//
//	mods/codec/internal/chart/chartcompat.go:245:1
type CanSetToolboxSaveAsImage interface {
	SetToolboxSaveAsImage(name string)
}

func ToolboxSaveAsImage(name string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetToolboxSaveAsImage); ok {
			_o.SetToolboxSaveAsImage(name)
		}
	}
}

// SetTranscoder
//
//	mods/codec/internal/csv/csv_decode.go:71:1
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
//	mods/codec/internal/json/json_encode.go:80:1
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
//	mods/codec/internal/chart/chartcompat.go:95:1
type CanSetVisualMap interface {
	SetVisualMap(min float64, max float64)
}

func VisualMap(min float64, max float64) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetVisualMap); ok {
			_o.SetVisualMap(min, max)
		}
	}
}

// SetVisualMapColor
//
//	mods/codec/internal/chart/chartcompat.go:103:1
type CanSetVisualMapColor interface {
	SetVisualMapColor(min float64, max float64, colors ...string)
}

func VisualMapColor(min float64, max float64, colors ...string) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetVisualMapColor); ok {
			_o.SetVisualMapColor(min, max, colors...)
		}
	}
}

// SetVolatileFileWriter
//
//	mods/codec/internal/chart/chart.go:69:1
//	mods/codec/internal/geomap/geomap.go:73:1
type CanSetVolatileFileWriter interface {
	SetVolatileFileWriter(w facility.VolatileFileWriter)
}

func VolatileFileWriter(w facility.VolatileFileWriter) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetVolatileFileWriter); ok {
			_o.SetVolatileFileWriter(w)
		}
	}
}

// SetXAxis
//
//	mods/codec/internal/chart/chartcompat.go:113:1
type CanSetXAxis interface {
	SetXAxis(args ...any)
}

func XAxis(args ...any) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetXAxis); ok {
			_o.SetXAxis(args...)
		}
	}
}

// SetYAxis
//
//	mods/codec/internal/chart/chartcompat.go:146:1
type CanSetYAxis interface {
	SetYAxis(args ...any)
}

func YAxis(args ...any) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetYAxis); ok {
			_o.SetYAxis(args...)
		}
	}
}

// SetZAxis
//
//	mods/codec/internal/chart/chartcompat.go:179:1
type CanSetZAxis interface {
	SetZAxis(args ...any)
}

func ZAxis(args ...any) Option {
	return func(_one any) {
		if _o, ok := _one.(CanSetZAxis); ok {
			_o.SetZAxis(args...)
		}
	}
}
