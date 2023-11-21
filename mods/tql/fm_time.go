package tql

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/util"
	"github.com/pkg/errors"
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
		return baseTime, errors.Wrap(err, "invalid time expression")
	}
	delta, err = util.ToDuration(deltaExpr)
	if err != nil {
		return baseTime, errors.Wrap(err, "invalid time expression")
	}
	return baseTime.Add(delta), nil
}

func (x *Node) fmParseTime(expr string, format string, tz *time.Location) (time.Time, error) {
	return util.ParseTime(expr, format, tz)
}

func (x *Node) fmTZ(timezone string) (*time.Location, error) {
	switch strings.ToUpper(timezone) {
	case "LOCAL":
		timezone = "Local"
	case "UTC":
		timezone = "UTC"
	}
	if timeLocation, err := time.LoadLocation(timezone); err != nil {
		return util.GetTimeLocation(timezone)
	} else {
		return timeLocation, nil
	}
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
	return &NullValue{altValue: v}
}

func (node *Node) fmTimeWindow(from any, until any, duration any, args ...any) any {
	var tw *TimeWindow

	if obj, ok := node.GetValue("timewindow"); ok {
		tw = obj.(*TimeWindow)
	} else {
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
			return ErrArgs("TIMEWINDOW", 3, err.Error())
		}
		node.SetFeedEOF(true)
		node.SetValue("timewindow", tw)
	}

	if node.Inflight().IsEOF() {
		// flush remain values
		if tw.curWindow.IsZero() {
			return nil
		}
		tw.Flush(node, tw.curWindow)
		tw.Fill(node, tw.curWindow, tw.tsUntil)
		return nil
	}

	var values []any
	if v, ok := node.Inflight().value.([]any); ok {
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
	if node.Rownum() == 1 {
		fromWindow := time.Unix(0, (tw.tsFrom.UnixNano()/int64(tw.period)-1)*int64(tw.period))
		tw.Fill(node, fromWindow, recWindow)
	}

	// window changed, yield buffered values
	if tw.curWindow != recWindow {
		tw.Flush(node, tw.curWindow)
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

type TimeWindow struct {
	tsFrom    time.Time
	tsUntil   time.Time
	period    time.Duration
	columns   []TimeWindowColumn
	timeIdx   int
	nullValue *NullValue

	curWindow time.Time
}

func NewTimeWindow() *TimeWindow {
	return &TimeWindow{
		timeIdx:   -1,
		nullValue: &NullValue{altValue: nil},
		curWindow: time.Time{},
	}
}

func (tw *TimeWindow) SetColumns(columns []string) error {
	for i, c := range columns {
		switch c {
		case "time":
			tw.timeIdx = i
			tw.columns = append(tw.columns, &TimeWindowColumnTime{})
		case "first", "last", "max", "min", "sum":
			tw.columns = append(tw.columns, &TimeWindowColumnSingle{name: c})
		case "avg", "rss", "rms":
			tw.columns = append(tw.columns, &TimeWindowColumnAggregate{name: c})
		default:
			return fmt.Errorf("unknown aggregator %q", c)
		}
	}
	if len(tw.columns) < 2 || tw.timeIdx == -1 {
		return fmt.Errorf("invalid columns count or no time column specified")
	}
	return nil
}

func (tw *TimeWindow) SeriesName(i int) string {
	return fmt.Sprintf("series%d", i)
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
		if err := tw.columns[i].Append(v); err != nil {
			return err
		}
	}
	return nil
}

type TimeWindowColumn interface {
	Append(v any) error
	Result(def any) any
}

// time
type TimeWindowColumnTime struct {
}

func (twc *TimeWindowColumnTime) Append(v any) error { return nil }
func (twc *TimeWindowColumnTime) Result(def any) any {
	return def
}

// first, last, min, max, sum
type TimeWindowColumnSingle struct {
	name     string
	value    any
	hasValue bool
}

func (twc *TimeWindowColumnSingle) Append(v any) error {
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

func (twc *TimeWindowColumnSingle) Result(def any) any {
	if twc.hasValue {
		ret := twc.value
		twc.value = 0
		twc.hasValue = false
		return ret
	}
	return def
}

// - avg average
// - rss root sum square
// - rms root mean square
type TimeWindowColumnAggregate struct {
	name  string
	value float64
	count int
}

func (twc *TimeWindowColumnAggregate) Append(v any) error {
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

func (twc *TimeWindowColumnAggregate) Result(def any) any {
	if twc.count == 0 {
		return def
	}
	defer func() {
		twc.count = 0
		twc.value = 0
	}()

	switch twc.name {
	case "avg":
		return twc.value / float64(twc.count)
	case "rss":
		return math.Sqrt(twc.value)
	case "rms":
		return math.Sqrt(twc.value / float64(twc.count))
	}
	return def
}

// curWindow: exclusive
// nextWindow: inclusive
func (tw *TimeWindow) Fill(node *Node, curWindow time.Time, nextWindow time.Time) {
	curWindow = curWindow.Add(tw.period)
	for nextWindow.Sub(curWindow) >= tw.period {
		ret := make([]any, len(tw.columns))
		for i := range ret {
			if i == tw.timeIdx {
				ret[i] = curWindow
			} else {
				ret[i] = tw.nullValue.Value()
			}
		}
		node.yield(curWindow, ret)
		curWindow = curWindow.Add(tw.period)
	}
}

func (tw *TimeWindow) Flush(node *Node, curWindow time.Time) {
	// aggregation
	ret := make([]any, len(tw.columns))
	for i, c := range tw.columns {
		if i == tw.timeIdx {
			ret[i] = curWindow
			continue
		}

		ret[i] = c.Result(tw.nullValue.Value())
	}
	// yield
	node.yield(curWindow, ret)
}
