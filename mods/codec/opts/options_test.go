package opts

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/nums"
	"golang.org/x/text/encoding/unicode"
)

type volatileWriterStub struct{}

func (v *volatileWriterStub) VolatileFilePrefix() string                                     { return "tmp" }
func (v *volatileWriterStub) VolatileFileWrite(name string, data []byte, deadline time.Time) {}

type optionSink struct {
	httpHeaders       map[string][]string
	autoRotate        float64
	boxDrawBorder     bool
	boxSeparate       bool
	boxStyle          string
	brief             bool
	briefCount        int
	charsetSet        bool
	chartAssets       []string
	chartDispatch     string
	chartID           string
	chartID2          string
	chartJS           string
	chartJSON         bool
	chartOption       string
	columnTypes       []api.DataType
	columns           []string
	contentType       string
	dataZoomType      string
	delimiter         string
	geoMapJSON        bool
	geomapID          string
	globalOptions     string
	gridSize          []float64
	header            bool
	headerColumns     bool
	heading           bool
	html              bool
	iconName          string
	iconOpt           string
	latlon            *nums.LatLon
	zoomLevel         int
	input             io.Reader
	lineWidth         float64
	logger            facility.Logger
	mapAssets         []string
	markAreaLabel     string
	markLineXName     string
	markLineYName     string
	opacity           float64
	output            io.Writer
	plugins           []string
	precision         int
	rownum            bool
	rowsArray         bool
	rowsFlatten       bool
	seriesLabels      []string
	width             string
	height            string
	substituteNull    any
	subtitle          string
	tableName         string
	templates         []string
	theme             string
	tileGrayscale     float64
	tileOption        string
	tileTemplate      string
	timeLocation      *time.Location
	timeformat        string
	title             string
	toolboxDataView   bool
	toolboxDataZoom   bool
	toolboxSaveAsName string
	transpose         bool
	visualMin         float64
	visualMax         float64
	visualColors      []string
	volatileWriter    facility.VolatileFileWriter
	xAxis             []any
	yAxis             []any
	zAxis             []any
}

func (s *optionSink) SetHttpHeader(key, value string) {
	if s.httpHeaders == nil {
		s.httpHeaders = map[string][]string{}
	}
	s.httpHeaders[key] = append(s.httpHeaders[key], value)
}
func (s *optionSink) SetAutoRotate(speed float64)          { s.autoRotate = speed }
func (s *optionSink) SetBoxDrawBorder(flag bool)           { s.boxDrawBorder = flag }
func (s *optionSink) SetBoxSeparateColumns(flag bool)      { s.boxSeparate = flag }
func (s *optionSink) SetBoxStyle(style string)             { s.boxStyle = style }
func (s *optionSink) SetBrief(flag bool)                   { s.brief = flag }
func (s *optionSink) SetBriefCount(count int)              { s.briefCount = count }
func (s *optionSink) SetCharsetEncoding(_ any)             { s.charsetSet = true }
func (s *optionSink) SetChartAssets(args ...string)        { s.chartAssets = args }
func (s *optionSink) SetChartDispatchAction(action string) { s.chartDispatch = action }
func (s *optionSink) SetChartID(id string)                 { s.chartID = id }
func (s *optionSink) SetChartId(id string)                 { s.chartID2 = id }
func (s *optionSink) SetChartJSCode(js string)             { s.chartJS = js }
func (s *optionSink) SetChartJson(flag bool)               { s.chartJSON = flag }
func (s *optionSink) SetChartOption(opt string)            { s.chartOption = opt }
func (s *optionSink) SetColumnTypes(types ...api.DataType) { s.columnTypes = types }
func (s *optionSink) SetColumns(names ...string)           { s.columns = names }
func (s *optionSink) SetContentType(contentType string)    { s.contentType = contentType }
func (s *optionSink) SetDataZoom(typ string, minPercentage float32, maxPercentage float32) {
	s.dataZoomType = typ
}
func (s *optionSink) SetDelimiter(newDelimiter string)    { s.delimiter = newDelimiter }
func (s *optionSink) SetGeoMapJson(flag bool)             { s.geoMapJSON = flag }
func (s *optionSink) SetGeomapID(id string)               { s.geomapID = id }
func (s *optionSink) SetGlobalOptions(opt string)         { s.globalOptions = opt }
func (s *optionSink) SetGridSize(args ...float64)         { s.gridSize = args }
func (s *optionSink) SetHeader(show bool)                 { s.header = show }
func (s *optionSink) SetHeaderColumns(headerColumns bool) { s.headerColumns = headerColumns }
func (s *optionSink) SetHeading(show bool)                { s.heading = show }
func (s *optionSink) SetHtml(flag bool)                   { s.html = flag }
func (s *optionSink) SetIcon(name string, opt string)     { s.iconName, s.iconOpt = name, opt }
func (s *optionSink) SetInitialLocation(latlon *nums.LatLon, zoomLevel int) {
	s.latlon, s.zoomLevel = latlon, zoomLevel
}
func (s *optionSink) SetInputStream(in io.Reader) { s.input = in }
func (s *optionSink) SetLineWidth(width float64)  { s.lineWidth = width }
func (s *optionSink) SetLogger(l facility.Logger) { s.logger = l }
func (s *optionSink) SetMapAssets(args ...string) { s.mapAssets = args }
func (s *optionSink) SetMarkAreaNameCoord(from any, to any, label string, color string, opacity float64) {
	s.markAreaLabel = label
}
func (s *optionSink) SetMarkLineXAxisCoord(xAxis any, name string) { s.markLineXName = name }
func (s *optionSink) SetMarkLineYAxisCoord(yAxis any, name string) { s.markLineYName = name }
func (s *optionSink) SetOpacity(opacity float64)                   { s.opacity = opacity }
func (s *optionSink) SetOutputStream(o io.Writer)                  { s.output = o }
func (s *optionSink) SetPlugins(plugins ...string)                 { s.plugins = plugins }
func (s *optionSink) SetPrecision(precision int)                   { s.precision = precision }
func (s *optionSink) SetRownum(show bool)                          { s.rownum = show }
func (s *optionSink) SetRowsArray(flag bool)                       { s.rowsArray = flag }
func (s *optionSink) SetRowsFlatten(flag bool)                     { s.rowsFlatten = flag }
func (s *optionSink) SetSeriesLabels(args ...string)               { s.seriesLabels = args }
func (s *optionSink) SetSize(width string, height string)          { s.width, s.height = width, height }
func (s *optionSink) SetSubstituteNull(alternative any)            { s.substituteNull = alternative }
func (s *optionSink) SetSubtitle(str string)                       { s.subtitle = str }
func (s *optionSink) SetTableName(tableName string)                { s.tableName = tableName }
func (s *optionSink) SetTemplate(templates ...string)              { s.templates = templates }
func (s *optionSink) SetTheme(theme string)                        { s.theme = theme }
func (s *optionSink) SetTileGrayscale(grayscale float64)           { s.tileGrayscale = grayscale }
func (s *optionSink) SetTileOption(opt string)                     { s.tileOption = opt }
func (s *optionSink) SetTileTemplate(url string)                   { s.tileTemplate = url }
func (s *optionSink) SetTimeLocation(tz *time.Location)            { s.timeLocation = tz }
func (s *optionSink) SetTimeformat(f string)                       { s.timeformat = f }
func (s *optionSink) SetTitle(str string)                          { s.title = str }
func (s *optionSink) SetToolboxDataView()                          { s.toolboxDataView = true }
func (s *optionSink) SetToolboxDataZoom()                          { s.toolboxDataZoom = true }
func (s *optionSink) SetToolboxSaveAsImage(name string)            { s.toolboxSaveAsName = name }
func (s *optionSink) SetTranspose(flag bool)                       { s.transpose = flag }
func (s *optionSink) SetVisualMap(min float64, max float64)        { s.visualMin, s.visualMax = min, max }
func (s *optionSink) SetVisualMapColor(min float64, max float64, colors ...string) {
	s.visualMin, s.visualMax, s.visualColors = min, max, colors
}
func (s *optionSink) SetVolatileFileWriter(w facility.VolatileFileWriter) { s.volatileWriter = w }
func (s *optionSink) SetXAxis(args ...any)                                { s.xAxis = args }
func (s *optionSink) SetYAxis(args ...any)                                { s.yAxis = args }
func (s *optionSink) SetZAxis(args ...any)                                { s.zAxis = args }

func TestOptionsApplyToCompatibleTargets(t *testing.T) {
	sink := &optionSink{}
	buf := &bytes.Buffer{}
	input := bytes.NewBufferString("hello")
	latlon := nums.NewLatLon(37.5, 127.0)
	writer := &volatileWriterStub{}

	ops := []Option{
		HttpHeader("X-Test", "one"),
		AutoRotate(1.5),
		BoxDrawBorder(true),
		BoxSeparateColumns(true),
		BoxStyle("round"),
		Brief(true),
		BriefCount(3),
		CharsetEncoding(unicode.UTF8),
		ChartAssets("a.js", "b.js"),
		ChartDispatchAction("zoom"),
		ChartID("id-1"),
		ChartId("id-2"),
		ChartJSCode("return 1"),
		ChartJson(true),
		ChartOption("{}"),
		ColumnTypes(api.DataTypeString, api.DataTypeDatetime),
		Columns("NAME", "TIME"),
		ContentType("application/json"),
		DataZoom("inside", 10, 90),
		Delimiter(";"),
		GeoMapJson(true),
		GeomapID("map-1"),
		GlobalOptions("global"),
		GridSize(2, 3),
		Header(true),
		HeaderColumns(true),
		Heading(true),
		Html(true),
		Icon("star", "red"),
		InitialLocation(latlon, 7),
		InputStream(input),
		LineWidth(2.5),
		Logger(facility.DiscardLogger),
		MapAssets("map.js"),
		MarkAreaNameCoord("a", "b", "label", "blue", 0.5),
		MarkLineXAxisCoord(10, "xline"),
		MarkLineYAxisCoord(20, "yline"),
		Opacity(0.7),
		OutputStream(buf),
		Plugins("p1", "p2"),
		Precision(4),
		Rownum(true),
		RowsArray(true),
		RowsFlatten(true),
		SeriesLabels("s1", "s2"),
		Size("100px", "200px"),
		SubstituteNull("NULL"),
		Subtitle("sub"),
		TableName("example"),
		Template("tmpl"),
		Theme("dark"),
		TileGrayscale(0.2),
		TileOption("opt"),
		TileTemplate("url"),
		TimeLocation(time.UTC),
		Timeformat("ns"),
		Title("title"),
		ToolboxDataView(),
		ToolboxDataZoom(),
		ToolboxSaveAsImage("img"),
		Transpose(true),
		VisualMap(1, 9),
		VisualMapColor(0, 100, "red", "green"),
		VolatileFileWriter(writer),
		XAxis("x"),
		YAxis("y"),
		ZAxis("z"),
	}
	for _, op := range ops {
		op(sink)
	}

	if sink.httpHeaders["X-Test"][0] != "one" || sink.chartDispatch != "zoom" || sink.tableName != "example" {
		t.Fatal("options were not applied as expected")
	}
	if sink.logger == nil || sink.output != buf || sink.input != input || sink.volatileWriter != writer {
		t.Fatal("io or logger options were not applied")
	}
	if sink.title != "title" || sink.subtitle != "sub" || sink.theme != "dark" {
		t.Fatal("title/theme options were not applied")
	}
	if sink.timeLocation != time.UTC || sink.timeformat != "ns" {
		t.Fatal("time options were not applied")
	}
	if sink.xAxis[0] != "x" || sink.yAxis[0] != "y" || sink.zAxis[0] != "z" {
		t.Fatal("axis options were not applied")
	}
}
