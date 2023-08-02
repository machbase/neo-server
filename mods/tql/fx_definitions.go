package tql

import (
	"math"

	"github.com/machbase/neo-server/mods/nums"
)

type Definition struct {
	Name string
	Func any
}

var FxDefinitions = []Definition{
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
	{"linspace", nums.Linspace},
	{"linspace50", nums.Linspace50},
	{"meshgrid", nums.Meshgrid},
	// maps.time
	{"// maps.time", nil},
	{"time", fmTime},
	{"timeAdd", fmTimeAdd},
	{"roundTime", fmRoundTime},
	{"range", fmTimeRange},
	// maps.monad
	{"// maps.monad", nil},
	{"TAKE", fmTake},
	{"DROP", fmDrop},
	{"FILTER", fmFilter},
	{"FLATTEN", fmFlatten},
	{"GROUPBYKEY", fmGroupByKey},
	{"POPKEY", fmPopKey},
	{"PUSHKEY", fmPushKey},
	{"SCRIPT", fmScriptTengo},
	{"lazy", fmLazy},
	// maps.dbsrc
	{"// maps.dbsrc", nil},
	{"from", fmFrom},
	{"limit", fmLimit},
	{"between", fmBetween},
	{"dump", fmDump},
	{"QUERY", fmQuery},
	{"SQL", fmSql},
	// maps.dbsink
	{"// maps.dbsink", nil},
	{"table", fmTable},
	{"tag", fmTag},
	{"INSERT", fmInsert},
	{"APPEND", fmAppend},
	// maps.fourier
	{"// maps.fourier", nil},
	{"minHz", fmMinHz},
	{"maxHz", fmMaxHz},
	{"FFT", fmFastFourierTransform},
	// maps.encoder
	{"// maps.encoder", nil},
	{"CSV", fmCsv},
	{"JSON", fmJson},
	{"MARKDOWN", fmMarkdown},
	{"CHART_LINE", ChartLine},
	{"CHART_SCATTER", fmChartScatter},
	{"CHART_BAR", fmChartBar},
	{"CHART_LINE3D", fmChartLine3D},
	{"CHART_BAR3D", fmChartBar3D},
	{"CHART_SURFACE3D", fmChartSurface3D},
	{"CHART_SCATTER3D", fmChartScatter3D},
	// maps.bytes
	{"// maps.bytes", nil},
	{"separator", fmSeparator},
	{"file", fmFile},
	{"STRING", fmString},
	{"BYTES", fmBytes},
	// maps.csv
	{"// maps.csv", nil},
	{"col", fmCol},
	{"field", fmField},
	{"header", ToHeader},
	{"datetimeType", fmDatetimeType},
	{"stringType", fmStringType},
	{"doubleType", fmDoubleType},
	// maps.fake
	{"freq", fmFreq},
	{"oscillator", fmOscillator},
	{"sphere", fmSphere},
	{"FAKE", fmFake},
	// input, output
	{"// maps.input", nil},
	{"INPUT", fmINPUT},
	{"// maps.output", nil},
	{"OUTPUT", fmOUTPUT},
	// aliases
	{"// aliases", nil},
	{"markArea", "fmMarkArea"},
	{"markXAxis", "x.gen_markLineXAxisCoord"},
	{"markYAxis", "x.gen_markLineYAxisCoord"},
	{"tz", TimeLocation},
	{"sep", fmSeparator},
}
