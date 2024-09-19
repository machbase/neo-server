package tql

import (
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
	{"SET", defTask.fmSET},
	{"context", defTask.GetContext},
	{"key", defTask.GetRecordKey},
	{"value", defTask.GetRecordValue},
	{"param", defTask.GetRequestParam},
	{"payload", defTask.GetRequestPayload},
	{"escapeParam", defTask.EscapeParam},
	{"option", defTask.fmOption},
	{"ARGS", defTask.fmArgs},
	// math
	{"// math", nil},
	{"abs", `mathWrap("abs", math.Abs)`},
	{"acos", `mathWrap("acos", math.Acos)`},
	{"acosh", `mathWrap("acosh", math.Acosh)`},
	{"asin", `mathWrap("asin", math.Asin)`},
	{"asinh", `mathWrap("asinh", math.Asinh)`},
	{"atan", `mathWrap("atan", math.Atan)`},
	{"atanh", `mathWrap("atanh", math.Atanh)`},
	{"ceil", `mathWrap("ceil", math.Ceil)`},
	{"cos", `mathWrap("cos", math.Cos)`},
	{"cosh", `mathWrap("cosh", math.Cosh)`},
	{"exp", `mathWrap("exp", math.Exp)`},
	{"exp2", `mathWrap("exp2", math.Exp2)`},
	{"floor", `mathWrap("floor", math.Floor)`},
	{"log", `mathWrap("log", math.Log)`},
	{"log10", `mathWrap("log10", math.Log10)`},
	{"log2", `mathWrap("log2", math.Log2)`},
	{"mod", `mathWrap2("mod", math.Mod)`},
	{"pow", `mathWrap2("pow", math.Pow)`},
	{"pow10", `mathWrapi("pow10", math.Pow10)`},
	{"remainder", `mathWrap2("remainder", math.Remainder)`},
	{"round", `mathWrap("round", math.Round)`},
	{"sin", `mathWrap("sin", math.Sin)`},
	{"sinh", `mathWrap("sinh", math.Sinh)`},
	{"sqrt", `mathWrap("sqrt", math.Sqrt)`},
	{"tan", `mathWrap("tan", math.Tan)`},
	{"tanh", `mathWrap("tanh", math.Tanh)`},
	{"trunc", `mathWrap("trunc", math.Trunc)`},
	// nums
	{"// nums", nil},
	{"len", "nums.Len"},
	{"element", "nums.Element"},
	{"linspace", defTask.fmLinspace},
	{"linspace50", defTask.fmLinspace50},
	{"meshgrid", defTask.fmMeshgrid},
	{"arrange", defTask.fmArrange},
	{"once", defTask.fmOnce},
	// geo
	{"latlon", nums.NewLatLon},
	{"geoPoint", nums.NewGeoPoint},
	{"geoCircle", nums.NewGeoCircle},
	{"geoMultiPoint", nums.NewGeoMultiPointFunc},
	{"geoPolygon", nums.NewGeoPolygonFunc},
	{"geoLineString", nums.NewGeoLineStringFunc},
	{"geoPointMarker", nums.NewGeoPointMarker},
	{"geoCircleMarker", nums.NewGeoCircleMarker},
	// maps.time
	{"// maps.time", nil},
	{"period", defTask.fmPeriod},
	{"nullValue", defTask.fmNullValue},
	{"time", defTask.fmTime},
	{"timeUnix", defTask.fmTimeUnix},
	{"timeUnixMilli", defTask.fmTimeUnixMilli},
	{"timeUnixMicro", defTask.fmTimeUnixMicro},
	{"timeUnixNano", defTask.fmTimeUnixNano},
	{"timeYear", defTask.fmTimeYear},
	{"timeMonth", defTask.fmTimeMonth},
	{"timeDay", defTask.fmTimeDay},
	{"timeHour", defTask.fmTimeHour},
	{"timeMinute", defTask.fmTimeMinute},
	{"timeSecond", defTask.fmTimeSecond},
	{"timeNanosecond", defTask.fmTimeNanosecond},
	{"timeISOYear", defTask.fmTimeISOYear},
	{"timeISOWeek", defTask.fmTimeISOWeek},
	{"timeYearDay", defTask.fmTimeYearDay},
	{"timeWeekDay", defTask.fmTimeWeekDay},
	{"parseTime", defTask.fmParseTime},
	{"timeAdd", defTask.fmTimeAdd},
	{"roundTime", defTask.fmRoundTime},
	{"range", defTask.fmTimeRange},
	{"sqlTimeformat", defTask.fmSqlTimeformat},
	{"ansiTimeformat", defTask.fmAnsiTimeformat},
	// maps.stat
	{"// maps.stat", nil},
	{"HISTOGRAM", defTask.fmHistogram},
	{"bins", defTask.fmBins},
	{"BOXPLOT", defTask.fmBoxplot},
	{"boxplotInterp", defTask.fmBoxplotInterp},
	{"boxplotOutput", defTask.fmBoxplotOutputFormat},
	{"category", defTask.fmCategory},
	{"order", defTask.fmOrder},
	// maps.monad
	{"// maps.monad", nil},
	{"TAKE", defTask.fmTake},
	{"DROP", defTask.fmDrop},
	{"FILTER", defTask.fmFilter},
	{"FILTER_CHANGED", defTask.fmFilterChanged},
	{"retain", defTask.fmRetain},
	{"useFirstWithLast", defTask.fmUseFirstWithLast},
	{"FLATTEN", defTask.fmFlatten},
	{"GROUPBYKEY", defTask.fmGroupByKey},
	{"POPKEY", defTask.fmPopKey},
	{"PUSHKEY", defTask.fmPushKey},
	{"MAPKEY", defTask.fmMapKey},
	{"POPVALUE", defTask.fmPopValue},
	{"PUSHVALUE", defTask.fmPushValue},
	{"MAPVALUE", defTask.fmMapValue},
	{"MAP_AVG", defTask.fmMapAvg},
	{"MAP_MOVAVG", defTask.fmMapMovAvg},
	{"noWait", defTask.fmNoWait},
	{"MAP_LOWPASS", defTask.fmMapLowPass},
	{"MAP_KALMAN", defTask.fmMapKalman},
	{"model", defTask.fmKalmanModel},
	{"MAP_DIFF", defTask.fmDiff},
	{"MAP_ABSDIFF", defTask.fmAbsDiff},
	{"MAP_NONEGDIFF", defTask.fmNonNegativeDiff},
	{"MAP_DISTANCE", defTask.fmGeoDistance},
	{"TRANSPOSE", defTask.fmTranspose},
	{"fixed", defTask.fmFixed},
	{"TIMEWINDOW", defTask.fmTimeWindow},
	{"SCRIPT", defTask.fmScript},
	{"SHELL", defTask.fmShell},
	{"list", defTask.fmList},
	{"dict", defTask.fmDictionary},
	{"lazy", defTask.fmLazy},
	{"glob", defTask.fmGlob},
	{"regexp", defTask.fmRegexp},
	{"doLog", defTask.fmDoLog},
	{"doHttp", defTask.fmDoHttp},
	{"do", defTask.fmDo},
	{"args", defTask.fmArgsParam},
	{"WHEN", defTask.fmWhen},
	{"THROTTLE", defTask.fmThrottle},
	// maps.dbsrc
	{"// maps.dbsrc", nil},
	{"from", defTask.fmFrom},
	{"limit", defTask.fmLimit},
	{"between", defTask.fmBetween},
	{"dump", defTask.fmDump},
	{"QUERY", defTask.fmQuery},
	{"SQL", defTask.fmSql},
	{"SQL_SELECT", defTask.fmSqlSelect},
	// maps.dbsink
	{"// maps.dbsink", nil},
	{"table", defTask.fmTable},
	{"tag", defTask.fmTag},
	{"INSERT", defTask.fmInsert},
	{"APPEND", defTask.fmAppend},
	// maps.bridge
	{"// maps.bridge", nil},
	{"bridge", defTask.fmBridge},
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
	{"DISCARD", defTask.fmDiscard},
	{"CHART", defTask.fmChart},
	{"CHART_LINE", defTask.fmChartLine},
	{"CHART_SCATTER", defTask.fmChartScatter},
	{"CHART_BAR", defTask.fmChartBar},
	{"CHART_LINE3D", defTask.fmChartLine3D},
	{"CHART_BAR3D", defTask.fmChartBar3D},
	{"CHART_SURFACE3D", defTask.fmChartSurface3D},
	{"CHART_SCATTER3D", defTask.fmChartScatter3D},
	{"GEOMAP", defTask.fmGeoMap},
	// maps.bytes
	{"// maps.bytes", nil},
	{"separator", defTask.fmSeparator},
	{"trimspace", defTask.fmTrimspace},
	{"file", defTask.fmFile},
	{"charset", defTask.fmCharset},
	{"STRING", defTask.fmString},
	{"BYTES", defTask.fmBytes},
	// maps.csv
	{"// maps.csv", nil},
	{"col", defTask.fmCol},
	{"field", defTask.fmField},
	{"stringType", defTask.fmStringType},
	{"datetimeType", defTask.fmDatetimeType}, // deprecated by timeType
	{"doubleType", defTask.fmDoubleType},     // deprecated by floatType
	{"timeType", defTask.fmDatetimeType},     // since v8.0.20
	{"floatType", defTask.fmDoubleType},      // since v8.0.20
	{"boolType", defTask.fmBoolType},         // since v8.0.20
	{"logProgress", defTask.fmLogProgress},   // since v8.0.29
	// maps.fake
	{"simplex", defTask.fmSimplex},
	{"random", defTask.fmRandom},
	{"parseFloat", defTask.fmParseFloat},
	{"parseBool", defTask.fmParseBoolean},
	{"strTime", defTask.fmStrTime},
	{"strTrimSpace", defTask.fmStrTrimSpace},
	{"strTrimPrefix", defTask.fmStrTrimPrefix},
	{"strTrimSuffix", defTask.fmStrTrimSuffix},
	{"strReplaceAll", defTask.fmStrReplaceAll},
	{"strReplace", defTask.fmStrReplace},
	{"strHasPrefix", defTask.fmStrHasPrefix},
	{"strHasSuffix", defTask.fmStrHasSuffix},
	{"strSprintf", defTask.fmStrSprintf},
	{"strSub", defTask.fmStrSub},
	{"strIndex", defTask.fmStrIndex},
	{"strLastIndex", defTask.fmStrLastIndex},
	{"strToUpper", defTask.fmStrToUpper},
	{"strToLower", defTask.fmStrToLower},
	{"freq", defTask.fmFreq},
	{"oscillator", defTask.fmOscillator},
	{"sphere", defTask.fmSphere},
	{"json", defTask.fmJsonData},
	{"csv", defTask.fmCsvData},
	{"FAKE", defTask.fmFake},
	// maps.group
	{"GROUP", defTask.fmGroup},
	{"by", defTask.fmBy},
	{"timewindow", defTask.fmByTimeWindow},
	{"where", defTask.fmWhere},
	{"predict", defTask.fmPredict},
	{"weight", defTask.fmWeight},
	{"first", defTask.fmFirst},
	{"last", defTask.fmLast},
	{"min", defTask.fmMin},
	{"max", defTask.fmMax},
	{"count", defTask.fmCount},
	{"sum", defTask.fmSum},
	{"mean", defTask.fmMean},
	{"variance", defTask.fmVariance},
	{"cdf", defTask.fmCDF},
	{"correlation", defTask.fmCorrelation},
	{"covariance", defTask.fmCovariance},
	{"quantile", defTask.fmQuantile},
	{"quantileInterpolated", defTask.fmQuantileInterpolated},
	{"median", defTask.fmMedian},
	{"medianInterpolated", defTask.fmMedianInterpolated},
	{"stddev", defTask.fmStdDev},
	{"stderr", defTask.fmStdErr},
	{"entropy", defTask.fmEntropy},
	{"mode", defTask.fmMode},
	{"moment", defTask.fmMoment},
	{"avg", defTask.fmAvg},
	{"rss", defTask.fmRSS},
	{"rms", defTask.fmRMS},
	{"lrs", defTask.fmLRS},
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

func mathWrap(name string, f func(float64) float64) func(args ...any) (any, error) {
	return func(args ...any) (any, error) {
		if args == nil {
			return nil, nil
		}
		if len(args) != 1 {
			return nil, ErrInvalidNumOfArgs(name, 1, len(args))
		}
		if args[0] == nil {
			return nil, nil
		}
		p0, err := convFloat64(args, 0, name, "float64")
		if err != nil {
			return nil, err
		}
		ret := f(p0)
		return ret, nil
	}
}

func mathWrapi(name string, f func(int) float64) func(args ...any) (any, error) {
	return func(args ...any) (any, error) {
		if args == nil {
			return nil, nil
		}
		if len(args) != 1 {
			return nil, ErrInvalidNumOfArgs(name, 1, len(args))
		}
		if args[0] == nil {
			return nil, nil
		}
		p0, err := convInt(args, 0, name, "int")
		if err != nil {
			return nil, err
		}
		ret := f(p0)
		return ret, nil
	}
}

func mathWrap2(name string, f func(float64, float64) float64) func(args ...any) (any, error) {
	return func(args ...any) (any, error) {
		if len(args) != 2 {
			return nil, ErrInvalidNumOfArgs(name, 2, len(args))
		}
		if args[0] == nil || args[1] == nil {
			return nil, nil
		}
		p0, err := convFloat64(args, 0, name, "float64")
		if err != nil {
			return nil, err
		}
		p1, err := convFloat64(args, 1, name, "float64")
		if err != nil {
			return nil, err
		}
		ret := f(p0, p1)
		return ret, nil
	}
}
