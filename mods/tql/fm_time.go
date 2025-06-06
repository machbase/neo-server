package tql

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/nums/fft"
	"github.com/machbase/neo-server/v8/mods/util"
	"gonum.org/v1/gonum/interp"
	"gonum.org/v1/gonum/stat"
)

type TimeRange struct {
	Time     time.Time
	Duration time.Duration
	Period   time.Duration
}

func (x *Node) fmTimeRange(ts any, dur any, period ...any) (*TimeRange, error) {
	var err error
	ret := &TimeRange{}
	ret.Time, err = util.ToTime(ts)
	if err != nil {
		return nil, ErrWrongTypeOfArgs("range", 0, "time", ts)
	}
	ret.Duration, err = util.ToDuration(dur)
	if err != nil {
		return nil, ErrWrongTypeOfArgs("range", 1, "duration", dur)
	}
	if len(period) == 0 {
		return ret, nil
	}
	ret.Period, err = util.ToDuration(period[0])
	if err != nil {
		return nil, ErrWrongTypeOfArgs("range", 2, "period", period[0])
	}
	abs := func(d time.Duration) time.Duration {
		if d < 0 {
			return d * -1
		}
		return d
	}
	if abs(ret.Duration) <= abs(ret.Period) {
		return nil, ErrArgs("range", 2, "period should be smaller than duration")
	}
	return ret, nil
}

// fmTimeYear returns the year in which ts occurs.
func (x *Node) fmTimeYear(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		return t.In(tz).Year(), nil
	}
}

// fmTimeMonth returns the month of the year specified by ts.
func (x *Node) fmTimeMonth(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		m := t.In(tz).Month()
		return int(m), nil
	}
}

// fmTimeDay returns the day of the month specified by ts.
func (x *Node) fmTimeDay(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		m := t.In(tz).Day()
		return int(m), nil
	}
}

// fmTimeHour returns the hour within the day specified by ts, in the range [0, 23].
func (x *Node) fmTimeHour(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		m := t.In(tz).Hour()
		return int(m), nil
	}
}

// fmTimeMinute returns the minute offset within the hour specified by t, in the range [0, 59].
func (x *Node) fmTimeMinute(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		m := t.In(tz).Minute()
		return int(m), nil
	}
}

// fmTimeSecond returns the second offset within the minute specified by t, in the range [0, 59].
func (x *Node) fmTimeSecond(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		m := t.In(tz).Second()
		return int(m), nil
	}
}

// fmTimeNanosecond returns the nanosecond offset within the second specified by ts,
// in the range [0, 999999999].
func (x *Node) fmTimeNanosecond(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		m := t.In(tz).Nanosecond()
		return int(m), nil
	}
}

// fmTimeISOWeek returns the ISO 8601 year number in which ts occurs.
func (x *Node) fmTimeISOYear(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		y, _ := t.In(tz).ISOWeek()
		return y, nil
	}
}

// fmTimeISOWeek returns the ISO 8601 week number in which t occurs.
// Week ranges from 1 to 53. Jan 01 to Jan 03 of year n might belong to
// week 52 or 53 of year n-1, and Dec 29 to Dec 31 might belong to week 1
// of year n+1.
func (x *Node) fmTimeISOWeek(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		_, w := t.In(tz).ISOWeek()
		return w, nil
	}
}

// fmTimeYearDay returns the day of the year specified by t, in the range [1,365] for non-leap years,
// and [1,366] in leap years.
func (x *Node) fmTimeYearDay(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		d := t.In(tz).YearDay()
		return d, nil
	}
}

// fmTimeWeekDay returns the day of the week specified by t. (Sunday = 0, ...).
func (x *Node) fmTimeWeekDay(ts any, args ...any) (int, error) {
	if t, err := util.ToTime(ts); err != nil {
		return 0, err
	} else {
		tz := time.UTC
		for _, arg := range args {
			switch av := arg.(type) {
			case *time.Location:
				tz = av
			}
		}
		d := t.In(tz).Weekday()
		return int(d), nil
	}
}

// ts : string | float64 | int64
// duration :  time.Time | *time.Time | float64 | int64
func (x *Node) fmRoundTime(ts any, duration any) (time.Time, error) {
	dur, err := util.ToDuration(duration)
	if err != nil {
		return time.Time{}, err
	}
	if dur == 0 {
		return time.Time{}, ErrArgs("roundTime", 1, "zero duration is not allowed")
	}
	t, err := util.ToTime(ts)
	if err != nil {
		return t, ErrArgs("roundTime", 0, err.Error())
	}
	ret := time.Unix(0, (t.UnixNano()/int64(dur))*int64(dur))
	return ret, nil
}

func (x *Node) fmPeriod(dur any) (time.Duration, error) {
	return util.ToDuration(dur)
}

func (x *Node) fmTime(ts any) (time.Time, error) {
	return x.fmTimeAdd(ts, int64(0))
}

func (x *Node) fmTimeAdd(tsExpr any, deltaExpr any) (time.Time, error) {
	var baseTime time.Time
	var delta time.Duration
	var err error
	baseTime, err = util.ToTime(tsExpr)
	if err != nil {
		return baseTime, fmt.Errorf("invalid time expression: %s", err.Error())
	}
	delta, err = util.ToDuration(deltaExpr)
	if err != nil {
		return baseTime, fmt.Errorf("invalid time expression: %s", err.Error())
	}
	return baseTime.Add(delta), nil
}

func (node *Node) fmTimeUnix(t any) (float64, error) {
	return node.fmTimeUnix0(t, "")
}

func (node *Node) fmTimeUnixMilli(t any) (float64, error) {
	return node.fmTimeUnix0(t, "ms")
}

func (node *Node) fmTimeUnixMicro(t any) (float64, error) {
	return node.fmTimeUnix0(t, "µs")
}

func (node *Node) fmTimeUnixNano(t any) (float64, error) {
	return node.fmTimeUnix0(t, "ns")
}

func (node *Node) fmTimeUnix0(t any, unit string) (float64, error) {
	var tm time.Time
	switch v := t.(type) {
	case time.Time:
		tm = v
	case *time.Time:
		tm = *v
	default:
		return 0, fmt.Errorf("timeUnix%s: %T(%v) is not time type", unit, t, t)
	}
	switch unit {
	case "ns":
		return float64(tm.UnixNano()), nil
	case "µs":
		return float64(tm.UnixMicro()), nil
	case "ms":
		return float64(tm.UnixMilli()), nil
	default:
		return float64(tm.Unix()), nil
	}
}

func (node *Node) fmStrTime(t any, format any, args ...any) (string, error) {
	var tm time.Time
	var tf string
	switch tv := t.(type) {
	case time.Time:
		tm = tv
	case *time.Time:
		tm = *tv
	case int64:
		tm = time.Unix(0, tv)
	default:
		return "", fmt.Errorf("strTime: %T(%v) is not time type", t, t)
	}
	switch fm := format.(type) {
	case string:
		switch fm {
		case "s", "ms", "us", "ns":
			tf = fm
		default:
			tf = util.GetTimeformat(fm)
		}
	case opts.Option:
		r := &fmParseTimeReceiver{}
		fm(r)
		if r.format == "" {
			return "", fmt.Errorf("strTime: %T(%v) is not time formatter", format, format)
		}
		tf = r.format
	default:
		return "", fmt.Errorf("strTime: %T(%v) is not time formatter", format, format)
	}
	tz := time.UTC
	for _, arg := range args {
		switch v := arg.(type) {
		case *time.Location:
			tz = v
		}
	}
	formatter := util.NewTimeFormatter(util.Timeformat(tf), util.TimeLocation(tz))
	return formatter.Format(tm), nil
}

type fmParseTimeReceiver struct {
	format string
}

func (r *fmParseTimeReceiver) SetTimeformat(f string) {
	r.format = f
}

func (x *Node) fmParseTime(expr string, format any, args ...any) (time.Time, error) {
	var tz = time.UTC
	for _, arg := range args {
		switch av := arg.(type) {
		case *time.Location:
			tz = av
		}
	}
	switch fv := format.(type) {
	case string:
		return util.ParseTime(expr, fv, tz)
	case opts.Option:
		r := &fmParseTimeReceiver{}
		fv(r)
		if r.format != "" {
			return util.ParseTime(expr, r.format, tz)
		}
	}
	return time.Time{}, fmt.Errorf("%q unsupported timeformat %T (%v)", x.Name(), format, format)
}

func (x *Node) fmTZ(timezone string) (*time.Location, error) {
	return util.ParseTimeLocation(timezone, nil)
}

func (x *Node) fmSqlTimeformat(format string) opts.Option {
	return opts.Timeformat(util.ToTimeformatSql(format))
}

func (x *Node) fmAnsiTimeformat(format string) opts.Option {
	return opts.Timeformat(util.ToTimeformatAnsi(format))
}

type NullValue struct {
	altValue any
}

func (n *NullValue) Value() any {
	return n.altValue
}

func (node *Node) fmNullValue(v any) any {
	if node.Name() == "CSV()" { // if CSV sink, obsolete substituteNull()
		return opts.SubstituteNull(v)
	} else {
		return &NullValue{altValue: v}
	}
}

func (node *Node) fmTimeWindow(from any, until any, duration any, args ...any) any {
	var tw *TimeWindow
	var isFirstRecord bool

	if obj, ok := node.GetValue("timewindow"); ok {
		tw = obj.(*TimeWindow)
	} else {
		// deprecated
		node.task.LogWarn("TIMEWINDOW() deprecated, use GROUP(timewindow()) instead")

		isFirstRecord = true
		tw = NewTimeWindow()
		if ts, err := util.ToTime(from); err != nil {
			return ErrArgs("TIMEWINDOW", 0, fmt.Sprintf("from is not compatible type, %T", from))
		} else {
			tw.tsFrom = ts
		}
		if ts, err := util.ToTime(until); err != nil {
			return ErrArgs("TIMEWINDOW", 1, fmt.Sprintf("until is not compatible type, %T", until))
		} else {
			tw.tsUntil = ts
		}
		if d, err := util.ToDuration(duration); err != nil {
			return ErrArgs("TIMEWINDOW", 2, fmt.Sprintf("duration is not compatible, %T", duration))
		} else if d == 0 {
			return ErrArgs("TIMEWINDOW", 2, "duration is zero")
		} else {
			tw.period = d
		}
		if tw.tsUntil.Sub(tw.tsFrom) <= tw.period {
			return ErrorRecord(ErrArgs("TIMEWINDOW", 0, "from ~ until should be larger than period"))
		}
		columns := []string{}
		for _, arg := range args {
			switch v := arg.(type) {
			case string:
				columns = append(columns, v)
			case *NullValue:
				tw.nullValue = v
			default:
				return ErrArgs("TIMEWINDOW", 3, fmt.Sprintf("column name invalid type, %T", v))
			}
		}
		if err := tw.SetColumns(columns); err != nil {
			node.task.LogError("TIMEWINDOW", err.Error())
			return ErrArgs("TIMEWINDOW", 3, err.Error())
		}
		node.SetEOF(tw.onEOF)
		node.SetValue("timewindow", tw)
	}

	inflight := node.Inflight()
	if inflight == nil {
		return nil
	}
	var values []any
	if v, ok := inflight.value.([]any); ok {
		values = v
	} else {
		return ErrorRecord(fmt.Errorf("TIMEWINDOW value should be array"))
	}
	if len(tw.columns) != len(values) {
		return ErrorRecord(fmt.Errorf("TIMEWINDOW column count does not match %d", len(values)))
	}

	var ts time.Time
	if v, err := util.ToTime(values[tw.timeIdx]); err != nil {
		return ErrorRecord(err)
	} else {
		ts = v
	}

	// recWindow value of the current record
	var recWindow = time.Unix(0, (ts.UnixNano()/int64(tw.period))*int64(tw.period))

	// out of range
	if !tw.IsInRange(recWindow) {
		return nil
	}

	// current processing window
	if tw.curWindow.IsZero() {
		tw.curWindow = recWindow
	}

	// fill missing leading records
	if isFirstRecord {
		fromWindow := time.Unix(0, (tw.tsFrom.UnixNano()/int64(tw.period)-1)*int64(tw.period))
		tw.Fill(node, fromWindow, recWindow)
	}

	// window changed, yield buffered values
	if tw.curWindow != recWindow {
		tw.Push(node, tw.curWindow)
		tw.Fill(node, tw.curWindow, recWindow)
		// update processing window
		tw.curWindow = recWindow
	}

	// append buffered values
	if err := tw.Buffer(values); err != nil {
		return err
	}

	return nil
}

type TimeWindowRecord struct {
	ts  time.Time
	val []any
}

type TimeWindow struct {
	tsFrom    time.Time
	tsUntil   time.Time
	period    time.Duration
	columns   []TimeWindowColumn
	timeIdx   int
	nullValue *NullValue

	curWindow time.Time

	bufferSize int
	buffer     []*TimeWindowRecord
	fillers    []TimeWindowFiller
	hasInterp  bool
}

func NewTimeWindow() *TimeWindow {
	return &TimeWindow{
		timeIdx:    -1,
		nullValue:  &NullValue{altValue: nil},
		curWindow:  time.Time{},
		bufferSize: 10,
	}
}

func (tw *TimeWindow) onEOF(node *Node) {
	if tw.curWindow.IsZero() {
		// It means there were no data in the given time range (from ~ until).
		// So fill the result with null values.
		tw.Fill(node, tw.tsFrom, tw.tsUntil)
		return
	}
	// flush remain values
	tw.Push(node, tw.curWindow)
	tw.Fill(node, tw.curWindow, tw.tsUntil)
	tw.flushBuffer(node)
}

func (tw *TimeWindow) SetColumns(columns []string) error {
	defaultFiller := &TimeWindowFillerConstant{value: tw.nullValue.Value()}
	var filler TimeWindowFiller
	for i, c := range columns {
		filler = defaultFiller
		typ, predict, found := strings.Cut(c, ":")
		if found {
			switch strings.ToLower(predict) {
			case "piecewiseconstant":
				filler = &TimeWindowFillerPredict{predictor: &interp.PiecewiseConstant{}, fallback: defaultFiller}
			case "piecewiselinear":
				filler = &TimeWindowFillerPredict{predictor: &interp.PiecewiseLinear{}, fallback: defaultFiller}
			case "akimaspline":
				filler = &TimeWindowFillerPredict{predictor: &interp.AkimaSpline{}, fallback: defaultFiller}
			case "fritschbutland":
				filler = &TimeWindowFillerPredict{predictor: &interp.FritschButland{}, fallback: defaultFiller}
			case "linearregression":
				filler = &TimeWindowFillerLinearRegression{fallback: defaultFiller}
			case "-":
				// use default interpolation
			default:
				return fmt.Errorf("unknown interpolation method %q", predict)
			}
		}
		if filler != defaultFiller {
			tw.hasInterp = true
		}
		tw.fillers = append(tw.fillers, filler)

		typ = strings.ToLower(typ)
		switch typ {
		case "time":
			tw.timeIdx = i
			tw.columns = append(tw.columns, &TimeWindowColumnTime{})
		case "first", "last", "max", "min", "sum":
			tw.columns = append(tw.columns, &TimeWindowColumnSingle{name: typ})
		case "avg", "rss", "rms":
			tw.columns = append(tw.columns, &TimeWindowColumnAggregate{name: typ})
		case "mean", "median", "median-interpolated", "stddev", "stderr", "entropy", "mode":
			tw.columns = append(tw.columns, &TimeWindowColumnStore{name: typ})
		case "fft":
			tw.columns = append(tw.columns, &TimeWindowColumnDsp{name: typ})
		default:
			return fmt.Errorf("unknown aggregator %q", typ)
		}
	}
	if len(tw.columns) < 2 || tw.timeIdx == -1 {
		return fmt.Errorf("invalid columns count or no time column specified")
	}
	return nil
}

func (tw *TimeWindow) pushBuffer(node *Node, ts time.Time, val []any) {
	if !tw.hasInterp {
		for i := range val {
			if val[i] == nil {
				val[i] = tw.fillers[i].Predict(ts)
			}
		}
		node.yield(ts, val)
		return
	}

	for i, filler := range tw.fillers {
		if val[i] != nil {
			filler.Add(ts, val[i])
		}
	}

	tw.buffer = append(tw.buffer, &TimeWindowRecord{ts: ts, val: val})
	if len(tw.buffer) <= tw.bufferSize {
		return
	}
	rec := tw.buffer[0]
	tw.buffer = tw.buffer[1:]
	for i := range rec.val {
		if rec.val[i] == nil {
			rec.val[i] = tw.fillers[i].Predict(rec.ts)
		}
	}
	node.yield(rec.ts, rec.val)
}

func (tw *TimeWindow) flushBuffer(node *Node) {
	for _, r := range tw.buffer {
		for i := range r.val {
			if r.val[i] == nil {
				r.val[i] = tw.fillers[i].Predict(r.ts)
			}
		}
		node.yield(r.ts, r.val)
	}
}

func (tw *TimeWindow) IsInRange(ts time.Time) bool {
	return ts.Sub(tw.tsFrom) >= 0 && ts.Sub(tw.tsUntil) < 0
}

// append buffered values
func (tw *TimeWindow) Buffer(values []any) error {
	if len(tw.columns) != len(values) {
		return fmt.Errorf("invalid columns count, expect %d, got %d", len(tw.columns), len(values))
	}
	for i, v := range values {
		if i == tw.timeIdx {
			continue
		}
		if ts, err := util.ToTime(values[tw.timeIdx]); err != nil {
			return err
		} else if err := tw.columns[i].Append(ts, v); err != nil {
			return err
		}
	}
	return nil
}

// curWindow: exclusive
// nextWindow: inclusive
func (tw *TimeWindow) Fill(node *Node, curWindow time.Time, nextWindow time.Time) {
	curWindow = curWindow.Add(tw.period)
	for nextWindow.Sub(curWindow) >= tw.period {
		ret := make([]any, len(tw.columns))
		for i, col := range tw.columns {
			ret[i] = col.Result(curWindow)
		}
		tw.pushBuffer(node, curWindow, ret)
		curWindow = curWindow.Add(tw.period)
	}
}

func (tw *TimeWindow) Push(node *Node, curWindow time.Time) {
	// aggregation
	ret := make([]any, len(tw.columns))
	for i, col := range tw.columns {
		ret[i] = col.Result(curWindow)
	}
	tw.pushBuffer(node, curWindow, ret)
}

type TimeWindowFiller interface {
	Add(ts time.Time, value any)
	Predict(ts time.Time) any
}

type TimeWindowFillerConstant struct {
	value any
}

func (fill *TimeWindowFillerConstant) Add(ts time.Time, val any) {}
func (fill *TimeWindowFillerConstant) Predict(x time.Time) any {
	return fill.value
}

type TimeWindowFillerPredict struct {
	predictor interp.FittablePredictor
	fallback  TimeWindowFiller
	xs        []float64
	ys        []float64
}

func (fill *TimeWindowFillerPredict) Add(ts time.Time, val any) {
	y, err := util.ToFloat64(val)
	if err != nil {
		return
	}
	x := float64(ts.UnixNano())
	fill.xs = append(fill.xs, x)
	fill.ys = append(fill.ys, y)

	limit := 100
	if len(fill.xs) > limit {
		fill.xs = fill.xs[len(fill.xs)-limit:]
		fill.ys = fill.ys[len(fill.ys)-limit:]
	}
}

func (fill *TimeWindowFillerPredict) Predict(ts time.Time) any {
	if len(fill.xs) < 2 || len(fill.xs) != len(fill.ys) {
		goto fallback
	}
	if err := fill.predictor.Fit(fill.xs, fill.ys); err != nil {
		goto fallback
	}
	return fill.predictor.Predict(float64(ts.UnixNano()))
fallback:
	return fill.fallback.Predict(ts)
}

type TimeWindowFillerLinearRegression struct {
	fallback TimeWindowFiller
	xs       []float64
	ys       []float64
}

func (fill *TimeWindowFillerLinearRegression) Add(ts time.Time, val any) {
	y, err := util.ToFloat64(val)
	if err != nil {
		return
	}
	x := float64(ts.UnixNano())
	fill.xs = append(fill.xs, x)
	fill.ys = append(fill.ys, y)

	limit := 100
	if len(fill.xs) > limit {
		fill.xs = fill.xs[len(fill.xs)-limit:]
		fill.ys = fill.ys[len(fill.ys)-limit:]
	}
}

func (fill *TimeWindowFillerLinearRegression) Predict(ts time.Time) (ret any) {
	if len(fill.xs) < 2 || len(fill.xs) != len(fill.ys) {
		ret = fill.fallback.Predict(ts)
		return ret
	}
	origin := false
	// y = alpha + beta*x
	alpha, beta := stat.LinearRegression(fill.xs, fill.ys, nil, origin)
	ret = alpha + beta*float64(ts.UnixNano())

	return ret
}

type TimeWindowColumn interface {
	Append(ts time.Time, v any) error
	Result(ts time.Time) any
}

// time
type TimeWindowColumnTime struct {
}

func (twc *TimeWindowColumnTime) Append(ts time.Time, v any) error { return nil }
func (twc *TimeWindowColumnTime) Result(ts time.Time) any {
	return ts
}

// first, last, min, max, sum
type TimeWindowColumnSingle struct {
	name     string
	value    any
	hasValue bool
}

func (twc *TimeWindowColumnSingle) Append(ts time.Time, v any) error {
	if twc.name == "first" {
		if twc.hasValue {
			return nil
		}
		twc.value = v
		twc.hasValue = true
		return nil
	} else if twc.name == "last" {
		twc.value = v
		twc.hasValue = true
		return nil
	}

	f, err := util.ToFloat64(v)
	if err != nil {
		return err
	}
	if !twc.hasValue {
		twc.value = f
		twc.hasValue = true
		return nil
	}

	old := twc.value.(float64)
	switch twc.name {
	case "min":
		if old > f {
			twc.value = f
		}
	case "max":
		if old < f {
			twc.value = f
		}
	case "sum":
		twc.value = old + f
	}
	return nil
}

func (twc *TimeWindowColumnSingle) Result(ts time.Time) any {
	if twc.hasValue {
		ret := twc.value
		twc.value = 0
		twc.hasValue = false
		return ret
	}
	return nil
}

// - avg average
// - rss root sum square
// - rms root mean square
type TimeWindowColumnAggregate struct {
	name  string
	value float64
	count int
}

func (twc *TimeWindowColumnAggregate) Append(ts time.Time, v any) error {
	f, err := util.ToFloat64(v)
	if err != nil {
		return err
	}
	switch twc.name {
	case "avg":
		twc.count++
		twc.value += f
	case "rss", "rms":
		twc.count++
		twc.value += f * f
	}
	return nil
}

func (twc *TimeWindowColumnAggregate) Result(ts time.Time) any {
	if twc.count == 0 {
		return nil
	}
	defer func() {
		twc.count = 0
		twc.value = 0
	}()

	var ret float64
	switch twc.name {
	case "avg":
		ret = twc.value / float64(twc.count)
	case "rss":
		ret = math.Sqrt(twc.value)
	case "rms":
		ret = math.Sqrt(twc.value / float64(twc.count))
	default:
		return nil
	}
	return ret
}

type TimeWindowColumnStore struct {
	name   string
	values []float64
}

func (twc *TimeWindowColumnStore) Append(ts time.Time, v any) error {
	f, err := util.ToFloat64(v)
	if err != nil {
		return err
	}
	twc.values = append(twc.values, f)
	return nil
}

func (twc *TimeWindowColumnStore) Result(ts time.Time) any {
	defer func() {
		twc.values = twc.values[0:0]
	}()

	var ret float64
	switch twc.name {
	case "mean":
		if len(twc.values) == 0 {
			return nil
		}
		ret, _ = stat.MeanStdDev(twc.values, nil)
	case "median":
		if len(twc.values) == 0 {
			return nil
		}
		sort.Float64s(twc.values)
		ret = stat.Quantile(0.5, stat.Empirical, twc.values, nil)
	case "median-interpolated":
		if len(twc.values) == 0 {
			return nil
		}
		sort.Float64s(twc.values)
		ret = stat.Quantile(0.5, stat.LinInterp, twc.values, nil)
	case "stddev":
		if len(twc.values) < 1 {
			return nil
		}
		_, ret = stat.MeanStdDev(twc.values, nil)
	case "stderr":
		if len(twc.values) < 1 {
			return nil
		}
		_, std := stat.MeanStdDev(twc.values, nil)
		ret = stat.StdErr(std, float64(len(twc.values)))
	case "entropy":
		if len(twc.values) == 0 {
			return nil
		}
		ret = stat.Entropy(twc.values)
	case "mode":
		if len(twc.values) == 0 {
			return nil
		}
		ret, _ = stat.Mode(twc.values, nil)
	default:
		return nil
	}
	if ret != ret { // NaN
		return nil
	}
	return ret
}

type TimeWindowColumnDsp struct {
	name   string
	times  []time.Time
	values []float64
}

func (twc *TimeWindowColumnDsp) Append(ts time.Time, v any) error {
	f, err := util.ToFloat64(v)
	if err != nil {
		return err
	}
	twc.times = append(twc.times, ts)
	twc.values = append(twc.values, f)
	return nil
}

func (twc *TimeWindowColumnDsp) Result(ts time.Time) any {
	defer func() {
		twc.times = twc.times[0:0]
		twc.values = twc.values[0:0]
	}()

	if len(twc.times) == 0 || len(twc.times) != len(twc.values) {
		return nil
	}
	switch twc.name {
	case "fft":
		frequencies, values := fft.FastFourierTransform(twc.times, twc.values)
		ret := [][]any{}
		for i := range frequencies {
			hz := frequencies[i]
			amp := values[i]
			if hz == 0 || hz != hz {
				continue
			}
			ret = append(ret, []any{hz, amp})
		}
		if len(ret) > 0 {
			return ret
		}
	}
	return nil
}
