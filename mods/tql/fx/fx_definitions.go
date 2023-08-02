package fx

import (
	"math"

	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/nums"
	"github.com/machbase/neo-server/mods/tql/conv"
	"github.com/machbase/neo-server/mods/tql/maps"
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
	{"time", maps.Time},
	{"timeAdd", maps.TimeAdd},
	{"roundTime", maps.RoundTime},
	{"range", maps.ToTimeRange},
	// maps.monad
	{"// maps.monad", nil},
	{"TAKE", maps.Take},
	{"DROP", maps.Drop},
	{"FILTER", maps.Filter},
	{"FLATTEN", maps.Flatten},
	{"GROUPBYKEY", maps.GroupByKey},
	{"POPKEY", maps.PopKey},
	{"PUSHKEY", maps.PushKey},
	{"SCRIPT", maps.ScriptTengo},
	{"lazy", maps.ToLazy},
	// maps.dbsrc
	{"// maps.dbsrc", nil},
	{"from", maps.ToFrom},
	{"limit", maps.ToLimit},
	{"between", maps.ToBetween},
	{"dump", maps.ToDump},
	{"QUERY", maps.ToQuery},
	{"SQL", maps.ToSql},
	// maps.dbsink
	{"// maps.dbsink", nil},
	{"table", maps.ToTable},
	{"tag", maps.ToTag},
	{"INSERT", maps.ToInsert},
	{"APPEND", maps.ToAppend},
	// maps.fourier
	{"// maps.fourier", nil},
	{"minHz", maps.ToMinHz},
	{"maxHz", maps.ToMaxHz},
	{"FFT", maps.FastFourierTransform},
	// maps.encoder
	{"// maps.encoder", nil},
	{"CSV", maps.ToCsv},
	{"JSON", maps.ToJson},
	{"MARKDOWN", maps.ToMarkdown},
	{"CHART_LINE", maps.ChartLine},
	{"CHART_SCATTER", maps.ChartScatter},
	{"CHART_BAR", maps.ChartBar},
	{"CHART_LINE3D", maps.ChartLine3D},
	{"CHART_BAR3D", maps.ChartBar3D},
	{"CHART_SURFACE3D", maps.ChartSurface3D},
	{"CHART_SCATTER3D", maps.ChartScatter3D},
	// maps.bytes
	{"// maps.bytes", nil},
	{"separator", maps.ToSeparator},
	{"file", maps.ToFile},
	{"STRING", maps.String},
	{"BYTES", maps.Bytes},
	// maps.csv
	{"// maps.csv", nil},
	{"col", maps.ToCol},
	{"field", maps.ToField},
	{"header", maps.ToHeader},
	{"datetimeType", maps.ToDatetimeType},
	{"stringType", maps.ToStringType},
	{"doubleType", maps.ToDoubleType},
	// maps.fake
	{"freq", maps.ToFreq},
	{"oscillator", maps.Oscillator},
	{"sphere", maps.Sphere},
	{"FAKE", maps.Fake},
	// input, output
	{"// maps.input", nil},
	{"INPUT", maps.INPUT},
	{"// maps.output", nil},
	{"OUTPUT", maps.OUTPUT},
	// aliases
	{"// aliases", nil},
	{"markArea", "markArea"},
	{"markXAxis", "x.gen_markLineXAxisCoord"},
	{"markYAxis", "x.gen_markLineYAxisCoord"},
	{"tz", maps.TimeLocation},
	{"sep", maps.ToSeparator},
}

func markArea(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, conv.ErrInvalidNumOfArgs("markArea", 2, len(args))
	}
	var err error
	coord0 := args[0]
	coord1 := args[1]
	label := ""
	color := ""
	opacity := 1.0
	if len(args) >= 3 {
		if label, err = conv.String(args, 2, "markArea", "label"); err != nil {
			return nil, err
		}
	}
	if len(args) >= 4 {
		if color, err = conv.String(args, 3, "markArea", "color"); err != nil {
			return nil, err
		}
	}
	if len(args) >= 5 {
		if opacity, err = conv.Float64(args, 4, "markArea", "opacity"); err != nil {
			return nil, err
		}
	}
	return opts.MarkAreaNameCoord(coord0, coord1, label, color, opacity), nil
}
