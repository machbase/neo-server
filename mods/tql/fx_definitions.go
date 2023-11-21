package tql

import (
	"math"

	"github.com/machbase/neo-server/mods/nums"
)

type Definition struct {
	Name string
	Func any
}

var defTask = &Node{}

var FxDefinitions = []Definition{
	// context
	{"// context", nil},
	{"context", defTask.GetContext},
	{"key", defTask.GetRecordKey},
	{"value", defTask.GetRecordValue},
	{"param", defTask.GetRequestParam},
	{"payload", defTask.GetRequestPayload},
	// math
	{"// math", nil},
	{"sin", math.Sin},
	{"cos", math.Cos},
	{"tan", math.Tan},
	{"exp", math.Exp},
	{"exp2", math.Exp2},
	{"log", math.Log},
	{"log10", math.Log10},
	// nums
	{"// nums", nil},
	{"count", "nums.Count"},
	{"len", "nums.Len"},
	{"element", "nums.Element"},
	{"round", nums.Round},
	{"linspace", defTask.fmLinspace},
	{"linspace50", defTask.fmLinspace50},
	{"meshgrid", defTask.fmMeshgrid},
	// maps.time
	{"// maps.time", nil},
	{"period", defTask.fmPeriod},
	{"nullValue", defTask.fmNullValue},
	{"time", defTask.fmTime},
	{"parseTime", defTask.fmParseTime},
	{"timeAdd", defTask.fmTimeAdd},
	{"roundTime", defTask.fmRoundTime},
	{"range", defTask.fmTimeRange},
	{"sqlTimeformat", defTask.fmSqlTimeformat},
	{"ansiTimeformat", defTask.fmAnsiTimeformat},
	// maps.monad
	{"// maps.monad", nil},
	{"TAKE", defTask.fmTake},
	{"DROP", defTask.fmDrop},
	{"FILTER", defTask.fmFilter},
	{"FLATTEN", defTask.fmFlatten},
	{"GROUPBYKEY", defTask.fmGroupByKey},
	{"POPKEY", defTask.fmPopKey},
	{"PUSHKEY", defTask.fmPushKey},
	{"MAPKEY", defTask.fmMapKey},
	{"POPVALUE", defTask.fmPopValue},
	{"PUSHVALUE", defTask.fmPushValue},
	{"MAPVALUE", defTask.fmMapValue},
	{"TIMEWINDOW", defTask.fmTimeWindow},
	{"SCRIPT", defTask.fmScript},
	{"lazy", defTask.fmLazy},
	// maps.dbsrc
	{"// maps.dbsrc", nil},
	{"from", defTask.fmFrom},
	{"limit", defTask.fmLimit},
	{"between", defTask.fmBetween},
	{"dump", defTask.fmDump},
	{"QUERY", defTask.fmQuery},
	{"SQL", defTask.fmSql},
	// maps.dbsink
	{"// maps.dbsink", nil},
	{"table", defTask.fmTable},
	{"tag", defTask.fmTag},
	{"INSERT", defTask.fmInsert},
	{"APPEND", defTask.fmAppend},
	// maps.bridge
	{"// maps.bridge", nil},
	{"bridge", defTask.fmBridge},
	{"BRIDGE_QUERY", defTask.fmBridgeQuery}, // do not use, under development...
	// maps.fourier
	{"// maps.fourier", nil},
	{"minHz", defTask.fmMinHz},
	{"maxHz", defTask.fmMaxHz},
	{"FFT", defTask.fmFastFourierTransform},
	// maps.encoder
	{"// maps.encoder", nil},
	{"CSV", defTask.fmCsv},
	{"JSON", defTask.fmJson},
	{"MARKDOWN", defTask.fmMarkdown},
	{"HTML", defTask.fmHtml},
	{"CHART_LINE", defTask.fmChartLine},
	{"CHART_SCATTER", defTask.fmChartScatter},
	{"CHART_BAR", defTask.fmChartBar},
	{"CHART_LINE3D", defTask.fmChartLine3D},
	{"CHART_BAR3D", defTask.fmChartBar3D},
	{"CHART_SURFACE3D", defTask.fmChartSurface3D},
	{"CHART_SCATTER3D", defTask.fmChartScatter3D},
	// maps.bytes
	{"// maps.bytes", nil},
	{"separator", defTask.fmSeparator},
	{"trimspace", defTask.fmTrimSpace},
	{"file", defTask.fmFile},
	{"STRING", defTask.fmString},
	{"BYTES", defTask.fmBytes},
	// maps.csv
	{"// maps.csv", nil},
	{"col", defTask.fmCol},
	{"field", defTask.fmField},
	{"datetimeType", defTask.fmDatetimeType},
	{"stringType", defTask.fmStringType},
	{"doubleType", defTask.fmDoubleType},
	// maps.fake
	{"freq", defTask.fmFreq},
	{"oscillator", defTask.fmOscillator},
	{"sphere", defTask.fmSphere},
	{"FAKE", defTask.fmFake},
	// input, output
	{"// maps.input", nil},
	{"INPUT", defTask.fmINPUT},
	{"// maps.output", nil},
	{"OUTPUT", defTask.fmOUTPUT},
	// aliases
	{"// aliases", nil},
	{"markArea", "x.fmMarkArea"},
	{"markXAxis", "x.gen_markLineXAxisCoord"},
	{"markYAxis", "x.gen_markLineYAxisCoord"},
	{"tz", defTask.fmTZ},
	{"sep", defTask.fmSeparator},
}
