package tql

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"
)

func TestStatementKindByFunctionName(t *testing.T) {
	kind, ok := StatementKindByFunctionName("CSV()")
	require.True(t, ok)
	require.Equal(t, StatementSourceOrSink, kind)

	kind, ok = StatementKindByFunctionName("SQL()")
	require.True(t, ok)
	require.Equal(t, StatementSourceOrMapOrSink, kind)

	kind, ok = StatementKindByFunctionName("customMap")
	require.True(t, ok)
	require.Equal(t, StatementMap, kind)

	kind, ok = StatementKindByFunctionName("")
	require.False(t, ok)
	require.Equal(t, StatementUnknown, kind)
}

type FunctionTestCase struct {
	f            func(args ...any) (any, error)
	args         []any
	expect       any
	expectErr    string
	expectErrAny bool
	verify       func(t *testing.T, ret any)
}

type volatileFileWriterStub struct{}

func (volatileFileWriterStub) VolatileFilePrefix() string {
	return ""
}

func (volatileFileWriterStub) VolatileFileWrite(string, []byte, time.Time) {}

func (tc FunctionTestCase) run(t *testing.T) {
	t.Helper()
	ret, err := tc.f(tc.args...)
	if tc.expectErr != "" {
		require.NotNil(t, err)
		require.Equal(t, tc.expectErr, err.Error())
		return
	}
	if tc.expectErrAny {
		require.Error(t, err)
		return
	}
	require.Nil(t, err)
	if tc.verify != nil {
		tc.verify(t, ret)
		return
	}
	require.Equal(t, tc.expect, ret)
}

func TestEscapeParam(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("escapeParam"),
		args:   []any{"a b"},
		expect: "a+b",
	}.run(t)
}

func TestGeoFunctions(t *testing.T) {
	node := NewNode(NewTask())
	seoul := nums.NewLatLon(37.5665, 126.9780)
	busan := nums.NewLatLon(35.1796, 129.0756)

	tests := []FunctionTestCase{
		{f: node.Function("latlon"), args: []any{37.5665, 126.9780}, expect: seoul},
		{f: node.Function("latlon"), args: []any{true, 126.9780}, expectErr: "f(latlon) arg(0) should be float64, but bool"},
		{f: node.Function("geoPoint"), args: []any{seoul, "Seoul"}, expect: nums.NewGeoPoint(seoul, "Seoul")},
		{f: node.Function("geoCircle"), args: []any{seoul, 1000.0, "Seoul"}, expect: nums.NewGeoCircle(seoul, 1000.0, "Seoul")},
		{f: node.Function("geoMultiPoint"), args: []any{seoul, busan, "cities"}, expect: nums.NewGeoMultiPointFunc(seoul, busan, "cities")},
		{f: node.Function("geoPolygon"), args: []any{seoul, busan, seoul, "area"}, expect: nums.NewGeoPolygonFunc(seoul, busan, seoul, "area")},
		{f: node.Function("geoLineString"), args: []any{seoul, busan, "route"}, expect: nums.NewGeoLineStringFunc(seoul, busan, "route")},
		{f: node.Function("geoPointMarker"), args: []any{seoul, "marker"}, expect: nums.NewGeoPointMarker(seoul, "marker")},
		{f: node.Function("geoCircleMarker"), args: []any{seoul, 1000.0, "marker"}, expect: nums.NewGeoCircleMarker(seoul, 1000.0, "marker")},
	}
	for index, test := range tests {
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			test.run(t)
		})
	}

	node.SetInflight(NewRecord("key", []any{"first", "second"}))
	FunctionTestCase{f: node.Function("key"), expect: "key"}.run(t)
	FunctionTestCase{f: node.Function("value"), expect: []any{"first", "second"}}.run(t)
	FunctionTestCase{f: node.Function("value"), args: []any{1}, expect: "second"}.run(t)
	FunctionTestCase{f: node.Function("payload"), expect: nil}.run(t)
	node.output = make(chan *Record, 1)
	FunctionTestCase{f: node.Function("ARGS"), expect: nil}.run(t)
}

func TestOptionFunctions(t *testing.T) {
	node := NewNode(NewTask())
	seoul := nums.NewLatLon(37.5665, 126.9780)
	verifyOption := func(t *testing.T, ret any) {
		t.Helper()
		option, ok := ret.(opts.Option)
		require.True(t, ok)
		require.NotNil(t, option)
	}

	tests := []FunctionTestCase{
		{f: node.Function("httpHeader"), args: []any{"X-Test", "value"}, verify: verifyOption},
		{f: node.Function("autoRotate"), args: []any{1.5}, verify: verifyOption},
		{f: node.Function("binaryformat"), args: []any{"hex"}, verify: verifyOption},
		{f: node.Function("boxDrawBorder"), args: []any{true}, verify: verifyOption},
		{f: node.Function("boxSeparateColumns"), args: []any{true}, verify: verifyOption},
		{f: node.Function("boxStyle"), args: []any{"light"}, verify: verifyOption},
		{f: node.Function("brief"), args: []any{true}, verify: verifyOption},
		{f: node.Function("briefCount"), args: []any{10}, verify: verifyOption},
		{f: node.Function("chartAssets"), args: []any{"asset.js", "theme.js"}, verify: verifyOption},
		{f: node.Function("chartDispatchAction"), args: []any{"highlight"}, verify: verifyOption},
		{f: node.Function("chartID"), args: []any{"chart-1"}, verify: verifyOption},
		{f: node.Function("chartId"), args: []any{"chart-1"}, verify: verifyOption},
		{f: node.Function("chartJSCode"), args: []any{"console.log(1)"}, verify: verifyOption},
		{f: node.Function("chartJson"), args: []any{true}, verify: verifyOption},
		{f: node.Function("chartOption"), args: []any{"{}"}, verify: verifyOption},
		{f: node.Function("columns"), args: []any{"time", "value"}, verify: verifyOption},
		{f: node.Function("contentType"), args: []any{"application/json"}, verify: verifyOption},
		{f: node.Function("dataZoom"), args: []any{"inside", 10.0, 90.0}, verify: verifyOption},
		{f: node.Function("delimiter"), args: []any{","}, verify: verifyOption},
		{f: node.Function("geoMapJson"), args: []any{true}, verify: verifyOption},
		{f: node.Function("geomapID"), args: []any{"map-1"}, verify: verifyOption},
		{f: node.Function("globalOptions"), args: []any{"{}"}, verify: verifyOption},
		{f: node.Function("gridSize"), args: []any{10.0, 20.0}, verify: verifyOption},
		{f: node.Function("header"), args: []any{true}, verify: verifyOption},
		{f: node.Function("headerColumns"), args: []any{true}, verify: verifyOption},
		{f: node.Function("heading"), args: []any{true}, verify: verifyOption},
		{f: node.Function("html"), args: []any{true}, verify: verifyOption},
		{f: node.Function("icon"), args: []any{"series", "circle"}, verify: verifyOption},
		{f: node.Function("initialLocation"), args: []any{seoul, 12}, verify: verifyOption},
		{f: node.Function("lineWidth"), args: []any{2.0}, verify: verifyOption},
		{f: node.Function("mapAssets"), args: []any{"map.js"}, verify: verifyOption},
		{f: node.Function("markAreaNameCoord"), args: []any{1.0, 2.0, "start", "end", 0.5}, verify: verifyOption},
		{f: node.Function("markLineXAxisCoord"), args: []any{1.0, "x"}, verify: verifyOption},
		{f: node.Function("markLineYAxisCoord"), args: []any{1.0, "y"}, verify: verifyOption},
		{f: node.Function("opacity"), args: []any{0.5}, verify: verifyOption},
		{f: node.Function("plugins"), args: []any{"dataZoom"}, verify: verifyOption},
		{f: node.Function("precision"), args: []any{3}, verify: verifyOption},
		{f: node.Function("rownum"), args: []any{true}, verify: verifyOption},
		{f: node.Function("rowsArray"), args: []any{true}, verify: verifyOption},
		{f: node.Function("rowsFlatten"), args: []any{true}, verify: verifyOption},
		{f: node.Function("seriesLabels"), args: []any{"cpu", "memory"}, verify: verifyOption},
		{f: node.Function("size"), args: []any{"800px", "600px"}, verify: verifyOption},
		{f: node.Function("substituteNull"), args: []any{"-"}, verify: verifyOption},
		{f: node.Function("subtitle"), args: []any{"subtitle"}, verify: verifyOption},
		{f: node.Function("tableName"), args: []any{"example"}, verify: verifyOption},
		{f: node.Function("template"), args: []any{"{{.Value}}"}, verify: verifyOption},
		{f: node.Function("theme"), args: []any{"dark"}, verify: verifyOption},
		{f: node.Function("tileGrayscale"), args: []any{0.5}, verify: verifyOption},
		{f: node.Function("tileOption"), args: []any{"{}"}, verify: verifyOption},
		{f: node.Function("tileTemplate"), args: []any{"https://tiles/{z}/{x}/{y}.png"}, verify: verifyOption},
		{f: node.Function("timeLocation"), args: []any{time.UTC}, verify: verifyOption},
		{f: node.Function("timeformat"), args: []any{"2006-01-02"}, verify: verifyOption},
		{f: node.Function("title"), args: []any{"title"}, verify: verifyOption},
		{f: node.Function("toolboxDataView"), verify: verifyOption},
		{f: node.Function("toolboxDataZoom"), verify: verifyOption},
		{f: node.Function("toolboxSaveAsImage"), args: []any{"chart"}, verify: verifyOption},
		{f: node.Function("transpose"), args: []any{true}, verify: verifyOption},
		{f: node.Function("visualMap"), args: []any{0.0, 100.0}, verify: verifyOption},
		{f: node.Function("visualMapColor"), args: []any{0.0, 100.0, "#000", "#fff"}, verify: verifyOption},
		{f: node.Function("xAxis"), args: []any{"time"}, verify: verifyOption},
		{f: node.Function("yAxis"), args: []any{"value"}, verify: verifyOption},
		{f: node.Function("zAxis"), args: []any{"depth"}, verify: verifyOption},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func TestGeneratedNodeFunctions(t *testing.T) {
	node := NewNode(NewTask())
	verifyValue := func(t *testing.T, ret any) {
		t.Helper()
		require.NotNil(t, ret)
	}
	verifyRecord := func(t *testing.T, ret any) {
		t.Helper()
		_, ok := ret.(*Record)
		require.True(t, ok)
	}

	tests := []FunctionTestCase{
		{f: node.Function("context"), verify: verifyValue},
		{f: node.Function("sep"), args: []any{","}, verify: verifyValue},
		{f: node.Function("THROTTLE"), args: []any{10.0}, verify: verifyRecord},
		{f: node.Function("use"), args: []any{"example"}, verify: verifyValue},
		{f: node.Function("CHART"), args: []any{opts.Title("chart")}, verify: verifyValue},
		{f: node.Function("CHART_LINE"), args: []any{opts.Title("line")}, verify: verifyValue},
		{f: node.Function("CHART_SCATTER"), args: []any{opts.Title("scatter")}, verify: verifyValue},
		{f: node.Function("CHART_BAR"), args: []any{opts.Title("bar")}, verify: verifyValue},
		{f: node.Function("CHART_LINE3D"), args: []any{opts.Title("line3d")}, verify: verifyValue},
		{f: node.Function("CHART_BAR3D"), args: []any{opts.Title("bar3d")}, verify: verifyValue},
		{f: node.Function("CHART_SURFACE3D"), args: []any{opts.Title("surface3d")}, verify: verifyValue},
		{f: node.Function("CHART_SCATTER3D"), args: []any{opts.Title("scatter3d")}, verify: verifyValue},
		{f: node.Function("DISCARD"), verify: verifyValue},
		{f: node.Function("col"), args: []any{0.0, "string", "name"}, verify: verifyValue},
		{f: node.Function("field"), args: []any{0.0, "string", "name"}, verify: verifyValue},
		{f: node.Function("stringType"), args: []any{"ignored"}, verify: verifyValue},
		{f: node.Function("datetimeType"), args: []any{"ns"}, verify: verifyValue},
		{f: node.Function("doubleType"), args: []any{"ignored"}, verify: verifyValue},
		{f: node.Function("floatType"), args: []any{"ignored"}, verify: verifyValue},
		{f: node.Function("boolType"), args: []any{"ignored"}, verify: verifyValue},
		{f: node.Function("charsetEncoding"), args: []any{unicode.UTF8}, verify: verifyValue},
		{f: node.Function("columnTypes"), args: []any{"s"}, verify: verifyValue},
		{f: node.Function("logger"), args: []any{facility.DiscardLogger}, verify: verifyValue},
		{f: node.Function("volatileFileWriter"), args: []any{volatileFileWriterStub{}}, verify: verifyValue},
		{f: node.Function("timeSecond"), args: []any{time.Unix(0, 0), time.UTC}, expect: 0},
		{f: node.Function("timeNanosecond"), args: []any{time.Unix(0, 123), time.UTC}, expect: 123},
		{f: node.Function("linspace50"), args: []any{0.0, 1.0}, verify: verifyValue},
	}
	for index, test := range tests {
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			test.run(t)
		})
	}
}

func TestGeneratedNodeWrapperValidation(t *testing.T) {
	node := NewNode(NewTask())
	verifyOption := func(t *testing.T, ret any) {
		t.Helper()
		_, ok := ret.(opts.Option)
		require.True(t, ok)
	}

	FunctionTestCase{f: node.Function("inputStream"), args: []any{bytes.NewBufferString("input")}, verify: verifyOption}.run(t)
	FunctionTestCase{f: node.Function("outputStream"), args: []any{&bytes.Buffer{}}, verify: verifyOption}.run(t)
	FunctionTestCase{f: node.Function("MAP_DISTANCE"), args: []any{0, 1}, expect: nil}.run(t)
	FunctionTestCase{f: node.Function("option"), expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("ARGS"), args: []any{1}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("SET"), expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("key"), args: []any{1}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("payload"), args: []any{1}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("geoCircle"), args: []any{"invalid", 1.0, "circle"}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("geoLineString"), args: []any{"invalid", "invalid"}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("timeSecond"), args: []any{true}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("latlon"), args: []any{1.0, true}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("geoCircle"), args: []any{nums.NewLatLon(0, 0), true, "circle"}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("geoLineString"), args: []any{nums.NewLatLon(0, 0), "invalid"}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("timeNanosecond"), args: []any{true}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"), args: []any{time.Unix(0, 0), true}, expectErrAny: true}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"), args: []any{true, "1s"}, expectErrAny: true}.run(t)
	chartNode := NewNode(NewTask())
	chartNode.name = "CHART()"
	FunctionTestCase{f: chartNode.Function("option"), args: []any{"title: chart"}, verify: verifyOption}.run(t)
	FunctionTestCase{f: node.Function("covariance"), args: []any{1.0, 2.0}, verify: func(t *testing.T, ret any) { require.NotNil(t, ret) }}.run(t)
	FunctionTestCase{f: node.Function("quantileInterpolated"), args: []any{1.0, 0.5}, verify: func(t *testing.T, ret any) { require.NotNil(t, ret) }}.run(t)

	for _, name := range []string{
		"latlon", "geoPoint", "geoCircle", "geoPointMarker", "geoCircleMarker",
		"httpHeader", "autoRotate", "binaryformat", "boxDrawBorder", "boxSeparateColumns", "boxStyle", "brief", "briefCount",
		"THROTTLE", "use", "charsetEncoding", "logger", "volatileFileWriter",
		"chartDispatchAction", "chartID", "chartId", "chartJSCode", "chartJson", "chartOption", "contentType", "dataZoom", "delimiter",
		"geoMapJson", "geomapID", "globalOptions", "header", "headerColumns", "heading", "html", "icon", "initialLocation", "lineWidth",
		"markAreaNameCoord", "markLineXAxisCoord", "markLineYAxisCoord", "opacity", "precision", "rownum", "rowsArray", "rowsFlatten",
		"size", "substituteNull", "subtitle", "tableName", "theme", "tileGrayscale", "tileOption", "tileTemplate", "timeLocation", "timeformat",
		"title", "toolboxSaveAsImage", "transpose", "visualMap", "visualMapColor", "linspace50", "MAP_DISTANCE", "covariance", "quantileInterpolated",
	} {
		t.Run(name, func(t *testing.T) {
			FunctionTestCase{f: node.Function(name), expectErrAny: true}.run(t)
		})
	}

	for _, name := range []string{
		"SET", "param", "escapeParam", "period", "time", "timeUnix", "timeUnixMilli", "timeUnixMicro", "timeUnixNano",
		"timeYear", "timeMonth", "timeDay", "timeHour", "timeMinute", "timeSecond", "timeNanosecond", "timeISOYear", "timeISOWeek", "timeYearDay", "timeWeekDay",
		"parseTime", "timeAdd", "roundTime", "range", "sqlTimeformat", "ansiTimeformat",
		"HISTOGRAM", "bins", "boxplotInterp", "boxplotOutput", "category",
		"FILTER", "FILTER_CHANGED", "retain", "useFirstWithLast", "MAPKEY", "PUSHVALUE", "MAPVALUE",
		"MAP_AVG", "MAP_MOVAVG", "MAP_LOWPASS", "MAP_KALMAN", "MAP_DIFF", "MAP_ABSDIFF", "MAP_NONEGDIFF", "TIMEWINDOW",
		"glob", "regexp", "strTime", "strTrimSpace", "strTrimPrefix", "strTrimSuffix", "strReplace", "strReplaceAll", "strSprintf", "strSub", "strIndex", "strLastIndex", "strToUpper", "strToLower",
		"by", "timewindow", "where", "predict", "weight", "first", "last", "min", "max", "sum", "mean", "variance", "cdf", "correlation", "quantile", "median", "medianInterpolated", "stddev", "stderr", "entropy", "mode", "moment", "avg", "rss", "rms", "lrs",
	} {
		t.Run(name, func(t *testing.T) {
			FunctionTestCase{f: node.Function(name), expectErrAny: true}.run(t)
		})
	}
}

func TestParseFloat(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("parseFloat"),
		args:   []any{"0"},
		expect: 0.0,
	}.run(t)
	FunctionTestCase{f: node.Function("parseFloat"),
		args:   []any{"-1.23"},
		expect: -1.23,
	}.run(t)
}

func TestParseBool(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("parseBool"),
		args:   []any{"true"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("parseBool"),
		args:   []any{"0"},
		expect: false,
	}.run(t)
	FunctionTestCase{f: node.Function("parseBool"),
		args:      []any{"some other text"},
		expectErr: "parseBool: parsing \"some other text\": invalid syntax",
	}.run(t)
}

func TestStrTime(t *testing.T) {
	now := time.Unix(0, 1704871917655327000)
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "RFC822", time.UTC},
		expect: "10 Jan 24 07:31 UTC",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "2006/01/02 15:04:05.999999", time.UTC},
		expect: "2024/01/10 07:31:57.655327",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, opts.Timeformat(util.ToTimeformatSql("YYYY/MM/DD HH24:MI:SS.nnnnnn")), time.UTC},
		expect: "2024/01/10 07:31:57.655327",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "ns", time.UTC},
		expect: "1704871917655327000",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "us"},
		expect: "1704871917655327",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "ms", time.UTC},
		expect: "1704871917655",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "s"},
		expect: "1704871917",
	}.run(t)
}

func TestStrTrimSpace(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strTrimSpace"),
		args:   []any{"  text\t\n"},
		expect: "text",
	}.run(t)
	FunctionTestCase{f: node.Function("strTrimSpace"),
		args:   []any{"   "},
		expect: "",
	}.run(t)
}

func TestStrTrimPrefix(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strTrimPrefix"),
		args:   []any{"  text\t\n", "  "},
		expect: "text\t\n",
	}.run(t)
	FunctionTestCase{f: node.Function("strTrimPrefix"),
		args:   []any{"__text", "_"},
		expect: "_text",
	}.run(t)
}

func TestStrTrimSuffix(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strTrimSuffix"),
		args:   []any{"  text\t\n", "\t\n"},
		expect: "  text",
	}.run(t)
	FunctionTestCase{f: node.Function("strTrimSuffix"),
		args:   []any{"__text", "text"},
		expect: "__",
	}.run(t)
}

func TestStrReplace(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strReplace"),
		args:   []any{"apple", "a", "A", 1},
		expect: "Apple",
	}.run(t)
	FunctionTestCase{f: node.Function("strReplace"),
		args:   []any{"apple", "p", "P", 1},
		expect: "aPple",
	}.run(t)
	FunctionTestCase{f: node.Function("strReplace"),
		args:   []any{"apple", "p", "P", -1},
		expect: "aPPle",
	}.run(t)
}

func TestStrReplaceAll(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strReplaceAll"),
		args:   []any{"apple", "a", "A"},
		expect: "Apple",
	}.run(t)
	FunctionTestCase{f: node.Function("strReplaceAll"),
		args:   []any{"apple", "p", "P"},
		expect: "aPPle",
	}.run(t)
}

func TestStrSprintf(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strSprintf"),
		args:   []any{"hello %s %1.2f", "world", 3.141592},
		expect: "hello world 3.14",
	}.run(t)
}

func TestStrSub(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World"},
		expect: "HelLo 😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"😀HelLo World", 0, 3},
		expect: "😀He",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", 6, -2},
		expect: "😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -7},
		expect: "😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -7, 3},
		expect: "😀 W",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -0},
		expect: "HelLo 😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -1},
		expect: "d",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -30},
		expect: "",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", 0, 30},
		expect: "HelLo 😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", 30, 30},
		expect: "",
	}.run(t)
}

func TestStrIndex(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strIndex"),
		args:   []any{"HelLo 😀 World", "😀"},
		expect: 6,
	}.run(t)
	FunctionTestCase{f: node.Function("strIndex"),
		args:   []any{"HelLo 😀 World", "o"},
		expect: 4,
	}.run(t)
	FunctionTestCase{f: node.Function("strIndex"),
		args:   []any{"HelLo 😀 World", "l"},
		expect: 2,
	}.run(t)
}

func TestStrLastIndex(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("strLastIndex"),
		args:   []any{"HelLo 😀 World", "😀"},
		expect: 6,
	}.run(t)
	FunctionTestCase{f: node.Function("strLastIndex"),
		args:   []any{"HelLo 😀 World", "o"},
		expect: 12,
	}.run(t)
	FunctionTestCase{f: node.Function("strLastIndex"),
		args:   []any{"HelLo 😀 World", "H"},
		expect: 0,
	}.run(t)
	FunctionTestCase{f: node.Function("strLastIndex"),
		args:   []any{"HelLo 😀 World", "l"},
		expect: 14,
	}.run(t)
}

func TestList(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("list"),
		args:   []any{"HelLo 😀", 3.14, true},
		expect: []any{"HelLo 😀", 3.14, true},
	}.run(t)
}

func TestGlob(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("glob"),
		args:   []any{"test*me", "test123me"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("glob"),
		args:   []any{"test*me", "testme"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("glob"),
		args:   []any{"test*me", "test123not"},
		expect: false,
	}.run(t)
}

func TestRegexp(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("regexp"),
		args:      []any{`^test[0-9$`, "test123"},
		expectErr: "error parsing regexp: missing closing ]: `[0-9$`",
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test[0-9]{3}$`, "test123"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test[0-9]{3}$`, "test12"},
		expect: false,
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test\d{3}$`, "test12345x"},
		expect: false,
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test\d{3}$`, "test999"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test\d{5}x$`, "test12345x"},
		expect: true,
	}.run(t)
}

func TestTime(t *testing.T) {
	node := NewNode(NewTask())

	tick := time.Now()
	util.StandardTimeNow = func() time.Time { return tick }
	// invalid number of args
	FunctionTestCase{f: node.Function("time"),
		args:      []any{},
		expectErr: "f(time) invalid number of args; expect:1, actual:0",
	}.run(t)
	// first args should be time, but %s",
	FunctionTestCase{f: node.Function("time"),
		args:      []any{"last"},
		expectErr: "invalid time expression: incompatible conv 'last' (string) to time.Time",
	}.run(t)
	// first args should be time, but
	FunctionTestCase{f: node.Function("time"),
		args:      []any{true},
		expectErr: "invalid time expression: incompatible conv 'true' (bool) to time.Time",
	}.run(t)
	// f(time) second args should be time, but %s
	FunctionTestCase{f: node.Function("time"),
		args:      []any{"oned2h"},
		expectErr: "invalid time expression: incompatible conv 'oned2h' (string) to time.Time",
	}.run(t)
	// f(time) second args should be time, but %s
	FunctionTestCase{f: node.Function("time"),
		args:      []any{"1d27h"},
		expectErr: "invalid time expression: incompatible conv '1d27h' (string) to time.Time",
	}.run(t)
	// f(time) second args should be duration, but %s
	FunctionTestCase{f: node.Function("timeAdd"),
		args:      []any{tick, "-2x"},
		expectErr: "invalid time expression: time: unknown unit \"x\" in duration \"-2x\"",
	}.run(t)
	FunctionTestCase{f: node.Function("time"),
		args:   []any{123456789.0},
		expect: time.Unix(0, 123456789),
	}.run(t)
	FunctionTestCase{f: node.Function("time"),
		args:   []any{"now"},
		expect: tick,
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "1s"},
		expect: tick.Add(1 * time.Second),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "1d"},
		expect: tick.Add(24 * time.Hour),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "-2d"},
		expect: tick.Add(-24 * 2 * time.Hour),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "-1d12h"},
		expect: tick.Add(-24 * 1.5 * time.Hour),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "-1d2h3m4s"},
		expect: tick.Add(-24*1*time.Hour - 2*time.Hour - 3*time.Minute - 4*time.Second),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now-1s", 1000000000},
		expect: tick,
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:      []any{"now-1x", 1000000000},
		expectErr: "invalid time expression: incompatible conv 'now-1x', time: unknown unit \"x\" in duration \"1x\"",
	}.run(t)
	// time.Time
	FunctionTestCase{f: node.Function("time"),
		args:   []any{tick},
		expect: tick,
	}.run(t)
	// *time.Time
	FunctionTestCase{f: node.Function("time"),
		args:   []any{&tick},
		expect: tick,
	}.run(t)

	FunctionTestCase{f: node.Function("timeUnix"),
		args:   []any{&tick},
		expect: float64(tick.Unix()),
	}.run(t)
	FunctionTestCase{f: node.Function("timeUnixMilli"),
		args:   []any{&tick},
		expect: float64(tick.UnixMilli()),
	}.run(t)
	FunctionTestCase{f: node.Function("timeUnixMicro"),
		args:   []any{tick},
		expect: float64(tick.UnixMicro()),
	}.run(t)
	FunctionTestCase{f: node.Function("timeUnixNano"),
		args:   []any{tick},
		expect: float64(tick.UnixNano()),
	}.run(t)
}

func TestTimeYear(t *testing.T) {
	node := NewNode(NewTask())

	tick := time.Now().In(time.UTC)
	util.StandardTimeNow = func() time.Time { return tick }

	FunctionTestCase{f: node.Function("timeYear"),
		args:   []any{tick},
		expect: tick.Year(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeYear"),
		args:   []any{tick, time.Local},
		expect: tick.In(time.Local).Year(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeMonth"),
		args:   []any{tick},
		expect: int(tick.Month()),
	}.run(t)
	FunctionTestCase{f: node.Function("timeMonth"),
		args:   []any{tick, time.Local},
		expect: int(tick.In(time.Local).Month()),
	}.run(t)
	FunctionTestCase{f: node.Function("timeDay"),
		args:   []any{tick},
		expect: tick.Day(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeDay"),
		args:   []any{tick, time.Local},
		expect: tick.In(time.Local).Day(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeHour"),
		args:   []any{tick},
		expect: tick.Hour(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeHour"),
		args:   []any{tick, time.Local},
		expect: tick.In(time.Local).Hour(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeMinute"),
		args:   []any{tick},
		expect: tick.Minute(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeMinute"),
		args:   []any{tick, time.Local},
		expect: tick.In(time.Local).Minute(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeSecond"),
		args:   []any{tick},
		expect: tick.Second(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeNanosecond"),
		args:   []any{tick},
		expect: tick.Nanosecond(),
	}.run(t)
	year, week := tick.ISOWeek()
	FunctionTestCase{f: node.Function("timeISOYear"),
		args:   []any{tick},
		expect: year,
	}.run(t)
	FunctionTestCase{f: node.Function("timeISOWeek"),
		args:   []any{tick},
		expect: week,
	}.run(t)
	year, week = tick.In(time.Local).ISOWeek()
	FunctionTestCase{f: node.Function("timeISOYear"),
		args:   []any{tick, time.Local},
		expect: year,
	}.run(t)
	FunctionTestCase{f: node.Function("timeISOWeek"),
		args:   []any{tick, time.Local},
		expect: week,
	}.run(t)
	FunctionTestCase{f: node.Function("timeYearDay"),
		args:   []any{tick},
		expect: tick.YearDay(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeYearDay"),
		args:   []any{tick, time.Local},
		expect: tick.In(time.Local).YearDay(),
	}.run(t)
	FunctionTestCase{f: node.Function("timeWeekDay"),
		args:   []any{tick},
		expect: int(tick.Weekday()),
	}.run(t)
	FunctionTestCase{f: node.Function("timeWeekDay"),
		args:   []any{tick, time.Local},
		expect: int(tick.In(time.Local).Weekday()),
	}.run(t)
}

func TestParseTime(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("tz"),
		args:   []any{"local"},
		expect: time.Local,
	}.run(t)
	FunctionTestCase{f: node.Function("tz"),
		args:   []any{"utc"},
		expect: time.UTC,
	}.run(t)
	FunctionTestCase{f: node.Function("tz"),
		args:      []any{"wrong/place"},
		expectErr: "unknown time zone wrong/place",
	}.run(t)
	FunctionTestCase{f: node.Function("parseTime"),
		args:   []any{"2023-03-01 14:01:02", "DEFAULT", time.Local},
		expect: time.Time(time.Date(2023, time.March, 1, 14, 1, 2, 0, time.Local)),
	}.run(t)

	FunctionTestCase{f: node.Function("parseTime"),
		args:   []any{"2023-03-01 14:01:02", "DEFAULT", time.UTC},
		expect: time.Time(time.Date(2023, time.March, 1, 14, 1, 2, 0, time.UTC)),
	}.run(t)

	FunctionTestCase{f: node.Function("parseTime"),
		args:   []any{"2023-03-01 14:01:02", "DEFAULT"},
		expect: time.Time(time.Date(2023, time.March, 1, 14, 1, 2, 0, time.UTC)),
	}.run(t)
}

func TestRoundTime(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("roundTime"),
		args:      []any{time.Unix(123, 456789123), "0s"},
		expectErr: "f(roundTime) arg(1) zero duration is not allowed",
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:      []any{true, "500ms"},
		expectErr: "f(roundTime) arg(0) incompatible conv 'true' (bool) to time.Time",
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:   []any{time.Unix(123, 456789123), "1s"},
		expect: time.Unix(123, 000000000),
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:   []any{time.Unix(123, 456789123), "10ms"},
		expect: time.Unix(123, 450000000),
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:   []any{time.Unix(123, 456789123), "10us"},
		expect: time.Unix(123, 456780000),
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:   []any{123456789123.0, "10us"},
		expect: time.Unix(123, 456780000),
	}.run(t)
}

func TestRangeTime(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("range"),
		args:      []any{false, "1s", "100ms"},
		expectErr: "f(range) arg(0) should be time, but bool",
	}.run(t)
	FunctionTestCase{f: node.Function("range"),
		args:      []any{0, "1x", "100ms"},
		expectErr: "f(range) arg(1) should be duration, but string",
	}.run(t)
	FunctionTestCase{f: node.Function("range"),
		args:      []any{0, "1s", "100x"},
		expectErr: "f(range) arg(2) should be period, but string",
	}.run(t)
	FunctionTestCase{f: node.Function("range"),
		args:      []any{0, "500ms", "1s"},
		expectErr: "f(range) arg(2) period should be smaller than duration",
	}.run(t)
	FunctionTestCase{f: node.Function("range"),
		args:   []any{0, "1s"},
		expect: &TimeRange{Time: time.Unix(0, 0), Duration: time.Second},
	}.run(t)
}

func TestLen(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("len"),
		args:   []any{[]string{"1", "2", "3", "4"}},
		expect: 4.0,
	}.run(t)
	FunctionTestCase{f: node.Function("len"),
		args:   []any{"1234"},
		expect: 4.0,
	}.run(t)
}

func TestElement(t *testing.T) {
	node := NewNode(NewTask())
	// invalid number of args
	FunctionTestCase{f: node.Function("element"),
		args:      []any{1, 2},
		expectErr: "f(element) invalud number of args (n:2)",
	}.run(t)
	// out of index
	FunctionTestCase{f: node.Function("element"),
		args:      []any{0.0, 1.0, 2.0, 3.0, 4.0, 5.0},
		expectErr: "f(element) out of index 5 / 5",
	}.run(t)
	// invalid index
	FunctionTestCase{f: node.Function("element"),
		args:      []any{0.0, 1.0, 2.0, 3.0, 4.0, "4"},
		expectErr: "f(element) index of element should be int, but string",
	}.run(t)
	// unsupported type
	FunctionTestCase{f: node.Function("element"),
		args:      []any{0.0, 1.0, 2.0, 3.0, time.Duration(1), 4},
		expectErr: "f(element) unsupported type time.Duration",
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{0.0, 1.0, 2.0, 3.0, 4.0, 1.0},
		expect: 1.0,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{0.0, 1.0, 2.0, 3.0, 4.0, 4},
		expect: 4.0,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{"abc", "bcd", "cde", "def", "efg", 4},
		expect: "efg",
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{"abc", "bcd", "cde", "def", true, 4},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{"abc", "bcd", "cde", "def", 123, 4},
		expect: 123.0,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{"abc", "bcd", "cde", "def", int64(12345), 4},
		expect: 12345.0,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{0.0, 1.0, 2.0, 3.0, time.Unix(123, int64(456)*int64(time.Millisecond)), 4},
		expect: 123.456 * 1000000000,
	}.run(t)
	tick1 := time.Unix(123, int64(456)*int64(time.Millisecond))
	FunctionTestCase{f: node.Function("element"),
		args:   []any{0.0, 1.0, 2.0, 3.0, &tick1, 4},
		expect: 123.456 * 1000000000,
	}.run(t)
}

func TestMathFunctions(t *testing.T) {
	node := NewNode(NewTask())
	FunctionTestCase{f: node.Function("round"),
		args:      []any{},
		expectErr: "f(round) invalid number of args; expect:1, actual:0",
	}.run(t)
	FunctionTestCase{f: node.Function("round"),
		args:      []any{"not_a_number"},
		expectErr: "f(round) arg(0) should be float64, but string",
	}.run(t)

	FunctionTestCase{f: node.Function("pow10"),
		args:      []any{},
		expectErr: "f(pow10) invalid number of args; expect:1, actual:0",
	}.run(t)
	FunctionTestCase{f: node.Function("pow10"),
		args:      []any{"not_a_number"},
		expectErr: "f(pow10) arg(0) should be int, but string",
	}.run(t)

	FunctionTestCase{f: node.Function("pow"),
		args:      []any{},
		expectErr: "f(pow) invalid number of args; expect:2, actual:0",
	}.run(t)
	FunctionTestCase{f: node.Function("pow"),
		args:      []any{1.0},
		expectErr: "f(pow) invalid number of args; expect:2, actual:1",
	}.run(t)
	FunctionTestCase{f: node.Function("pow"),
		args:      []any{"not_a_number", "2.0"},
		expectErr: "f(pow) arg(0) should be float64, but string",
	}.run(t)
	FunctionTestCase{f: node.Function("pow"),
		args:      []any{"1.0", "not_a_number"},
		expectErr: "f(pow) arg(1) should be float64, but string",
	}.run(t)

	tests := []FunctionTestCase{
		{f: node.Function("abs"), args: []any{1.1}, expect: 1.1},
		{f: node.Function("abs"), args: []any{-1.1}, expect: float64(1.1)},
		{f: node.Function("acos"), args: []any{math.Cos(math.Pi)}, expect: math.Pi},
		{f: node.Function("asin"), args: []any{math.Sin(math.Pi / 2)}, expect: math.Pi / 2},
		{f: node.Function("ceil"), args: []any{3.1415}, expect: 4.0},
		{f: node.Function("cos"), args: []any{math.Pi}, expect: -1.0},
		{f: node.Function("exp"), args: []any{0.0}, expect: 1.0},
		{f: node.Function("exp2"), args: []any{2.0}, expect: 4.0},
		{f: node.Function("floor"), args: []any{3.14}, expect: 3.0},
		{f: node.Function("log"), args: []any{1.0}, expect: 0.0},
		{f: node.Function("log2"), args: []any{8.0}, expect: 3.0},
		{f: node.Function("log10"), args: []any{100.0}, expect: 2.0},
		{f: node.Function("min"), args: []any{1.0, 1.1}, expect: float64(1.0)},
		{f: node.Function("max"), args: []any{1.0, 1.1}, expect: float64(1.1)},
		{f: node.Function("mod"), args: []any{5.0, 2.0}, expect: float64(1.0)},
		{f: node.Function("pow"), args: []any{2.0, 3.0}, expect: float64(8.0)},
		{f: node.Function("pow"), args: []any{nil, nil}, expect: nil},
		{f: node.Function("pow10"), args: []any{3.0}, expect: float64(1000.0)},
		{f: node.Function("pow10"), args: []any{nil}, expect: nil},
		{f: node.Function("remainder"), args: []any{5.0, 2.0}, expect: float64(1.0)},
		{f: node.Function("round"), args: []any{123.4567}, expect: float64(123)},
		{f: node.Function("round"), args: []any{234.5678}, expect: float64(235)},
		{f: node.Function("round"), args: []any{nil}, expect: nil},
		{f: node.Function("sin"), args: []any{math.Pi / 2}, expect: 1.0},
		{f: node.Function("sqrt"), args: []any{4.0}, expect: 2.0},
		{f: node.Function("tan"), args: []any{0.0}, expect: 0.0},
		{f: node.Function("trunc"), args: []any{math.Pi}, expect: 3.0},
	}
	for _, tt := range tests {
		tt.run(t)
	}
}

// TestCase
type MapFuncTestCase struct {
	input     string
	params    *Node // expression.Parameters
	expect    *Record
	expectErr string
}

func FuncParamMock(k any, v any) *Node {
	task := NewTask()
	node := NewNode(task)
	node.SetInflight(NewRecord(k, v))
	return node
}

func TestMapFunc_roundTime(t *testing.T) {
	MapFuncTestCase{
		input:     `roundTime()`,
		params:    FuncParamMock(1, ""),
		expectErr: "f(roundTime) invalid number of args; expect:2, actual:0",
	}.run(t)
	MapFuncTestCase{
		input:     `roundTime(123, '1x')`,
		params:    FuncParamMock(1, ""),
		expectErr: "time: unknown unit \"x\" in duration \"1x\"",
	}.run(t)
}

func TestMapFunc_TAKE(t *testing.T) {
	MapFuncTestCase{
		input:  `TAKE(1)`,
		params: FuncParamMock("sam", []any{1, 2, 3}),
		expect: NewRecord("sam", []any{1, 2, 3}),
	}.run(t)
}

func TestMapFunc_PUSHKEY(t *testing.T) {
	extime := time.Unix(123, 0)
	MapFuncTestCase{
		input:     `PUSHKEY()`,
		params:    FuncParamMock(extime, []any{1, 2, 3}),
		expectErr: "f(PUSHKEY) invalid number of args; expect:1, actual:0",
	}.run(t)
	MapFuncTestCase{
		input:  `PUSHKEY('sam')`,
		params: FuncParamMock(extime, []any{1, 2, 3}),
		expect: NewRecord("sam", []any{extime, 1, 2, 3}),
	}.run(t)
	tick := time.Now()
	tick100ms := time.Unix(0, (tick.UnixNano()/100000000)*100000000)
	MapFuncTestCase{
		input:  `PUSHKEY(roundTime(key(), '100ms'))`,
		params: FuncParamMock(tick, []any{"v"}),
		expect: NewRecord(tick100ms, []any{tick, "v"}),
	}.run(t)
}

func TestMapFunc_POPKEY(t *testing.T) {
	MapFuncTestCase{
		input:     `POPKEY()`,
		params:    FuncParamMock("x", []int{1, 2, 3}),
		expectErr: "f(POPKEY) V should be []any or [][]any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY()`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: NewRecord(1, []any{2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY()`,
		params: FuncParamMock("x", []any{[]int{10, 11, 12}, []int{20, 21, 22}, []int{30, 31, 32}}),
		expect: NewRecord([]int{10, 11, 12}, []any{[]int{20, 21, 22}, []int{30, 31, 32}}),
	}.run(t)
	MapFuncTestCase{
		input:     `POPKEY(0)`,
		params:    FuncParamMock("x", []int{1, 2, 3}),
		expectErr: "f(POPKEY) V should be []any or [][]any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY(1)`,
		params: FuncParamMock("x", []any{"K", 1, 2}),
		expect: NewRecord(1, []any{"K", 2}),
	}.run(t)
}

func TestMapFunc_FILTER(t *testing.T) {
	MapFuncTestCase{
		input:  `FILTER(10<100)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(10>100)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(key() == 'x')`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(key() != 'x')`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(key() != 'y')`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(len(value()) > 2)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(len(value()) > 4)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(element(value(), 0) >= 1)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(element(value(), 0) > 0)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: NewRecord("x", []any{1, 2, 3}),
	}.run(t)
}

func TestMapFunc_GROUPBYKEY(t *testing.T) {
	MapFuncTestCase{
		input:  `GROUPBYKEY()`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
}

func (tc MapFuncTestCase) run(t *testing.T) {
	msg := fmt.Sprintf("TestCase %s", tc.input)
	expr, err := tc.params.Parse(tc.input)
	require.Nil(t, err, msg)
	require.NotNil(t, expr, msg)

	ret, err := expr.Eval(tc.params)
	if tc.expectErr != "" {
		require.NotNil(t, err, msg)
		require.Equal(t, tc.expectErr, err.Error(), fmt.Sprintf(`"%s"`, msg))
		return
	}
	require.Nil(t, err, msg)

	if tc.expect == nil {
		require.Nil(t, ret)
		return
	}
	require.NotNil(t, ret, msg)
	// compare key
	if retParam, ok := ret.(*Record); !ok {
		t.Fatalf("invalid return type: %T", ret)
	} else {
		require.True(t, tc.expect.EqualKey(retParam), "K's are different")
		require.True(t, tc.expect.EqualValue(retParam), "V's are different")
	}
}
