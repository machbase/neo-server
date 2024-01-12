//go:generate go run ../codec/opts/generate.go fx

package tql

// Code generated by go generate; DO NOT EDIT.

import (
	"github.com/machbase/neo-server/mods/codec/opts"
)

var CodecOptsDefinitions = []Definition{
	{Name: "autoRotate", Func: opts.AutoRotate},
	{Name: "boxDrawBorder", Func: opts.BoxDrawBorder},
	{Name: "boxSeparateColumns", Func: opts.BoxSeparateColumns},
	{Name: "boxStyle", Func: opts.BoxStyle},
	{Name: "brief", Func: opts.Brief},
	{Name: "briefCount", Func: opts.BriefCount},
	{Name: "charsetEncoding", Func: opts.CharsetEncoding},
	{Name: "chartAssets", Func: opts.ChartAssets},
	{Name: "chartDispatchAction", Func: opts.ChartDispatchAction},
	{Name: "chartID", Func: opts.ChartID},
	{Name: "chartId", Func: opts.ChartId},
	{Name: "chartJSCode", Func: opts.ChartJSCode},
	{Name: "chartJson", Func: opts.ChartJson},
	{Name: "chartOption", Func: opts.ChartOption},
	{Name: "columnTypes", Func: opts.ColumnTypes},
	{Name: "columns", Func: opts.Columns},
	{Name: "dataZoom", Func: opts.DataZoom},
	{Name: "delimiter", Func: opts.Delimiter},
	{Name: "geoMapJson", Func: opts.GeoMapJson},
	{Name: "globalOptions", Func: opts.GlobalOptions},
	{Name: "gridSize", Func: opts.GridSize},
	{Name: "header", Func: opts.Header},
	{Name: "heading", Func: opts.Heading},
	{Name: "html", Func: opts.Html},
	{Name: "icon", Func: opts.Icon},
	{Name: "initialLocation", Func: opts.InitialLocation},
	{Name: "inputStream", Func: opts.InputStream},
	{Name: "layer", Func: opts.Layer},
	{Name: "lineWidth", Func: opts.LineWidth},
	{Name: "logger", Func: opts.Logger},
	{Name: "mapAssets", Func: opts.MapAssets},
	{Name: "mapId", Func: opts.MapId},
	{Name: "markAreaNameCoord", Func: opts.MarkAreaNameCoord},
	{Name: "markLineXAxisCoord", Func: opts.MarkLineXAxisCoord},
	{Name: "markLineYAxisCoord", Func: opts.MarkLineYAxisCoord},
	{Name: "opacity", Func: opts.Opacity},
	{Name: "outputStream", Func: opts.OutputStream},
	{Name: "plugins", Func: opts.Plugins},
	{Name: "pointStyle", Func: opts.PointStyle},
	{Name: "precision", Func: opts.Precision},
	{Name: "rownum", Func: opts.Rownum},
	{Name: "seriesLabels", Func: opts.SeriesLabels},
	{Name: "size", Func: opts.Size},
	{Name: "substituteNull", Func: opts.SubstituteNull},
	{Name: "subtitle", Func: opts.Subtitle},
	{Name: "tableName", Func: opts.TableName},
	{Name: "theme", Func: opts.Theme},
	{Name: "tileGrayscale", Func: opts.TileGrayscale},
	{Name: "tileOption", Func: opts.TileOption},
	{Name: "tileTemplate", Func: opts.TileTemplate},
	{Name: "timeLocation", Func: opts.TimeLocation},
	{Name: "timeformat", Func: opts.Timeformat},
	{Name: "title", Func: opts.Title},
	{Name: "toolboxDataView", Func: opts.ToolboxDataView},
	{Name: "toolboxDataZoom", Func: opts.ToolboxDataZoom},
	{Name: "toolboxSaveAsImage", Func: opts.ToolboxSaveAsImage},
	{Name: "transcoder", Func: opts.Transcoder},
	{Name: "transpose", Func: opts.Transpose},
	{Name: "visualMap", Func: opts.VisualMap},
	{Name: "visualMapColor", Func: opts.VisualMapColor},
	{Name: "volatileFileWriter", Func: opts.VolatileFileWriter},
	{Name: "xAxis", Func: opts.XAxis},
	{Name: "yAxis", Func: opts.YAxis},
	{Name: "zAxis", Func: opts.ZAxis},
}
