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
		argIdx := 0
		for _, arg := range args {
			switch v := arg.(type) {
			case string:
				tw.aggregations = append(tw.aggregations, v)
				switch v {
				case "time":
					tw.timeIdx = argIdx
					argIdx++
				case "avg", "max", "min", "first", "last", "sum", "rss":
					// ok
					argIdx++
				default:
					return ErrArgs("TIMEWINDOW", 2, fmt.Sprintf("unknown aggregator %q", v))
				}
			case *NullValue:
				tw.nullValue = v
			default:
				return ErrArgs("TIMEWINDOW", 3, fmt.Sprintf("column name invalid type, %T", v))
			}
		}
		if len(tw.aggregations) < 2 || tw.timeIdx == -1 {
			return ErrArgs("TIMEWINDOW", 3, "invalid columns count or no time column specified")
		}
		node.SetFeedEOF(true)
		node.SetValue("timewindow", tw)
	}

	if node.Inflight().IsEOF() {
		// flush remain values
		var curWindow time.Time
		if t, ok := node.GetValue("curWindow"); !ok {
			return nil
		} else {
			curWindow = t.(time.Time)
		}
		tw.Flush(node, curWindow)
		tw.Fill(node, curWindow, tw.tsUntil)
		return nil
	}

	var values []any
	if v, ok := node.Inflight().value.([]any); ok {
		values = v
	} else {
		return ErrorRecord(fmt.Errorf("TIMEWINDOW value should be array"))
	}
	if len(tw.aggregations) != len(values) {
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
	var curWindow time.Time
	if w, ok := node.GetValue("curWindow"); ok {
		curWindow = w.(time.Time)
	} else {
		node.SetValue("curWindow", recWindow)
		curWindow = recWindow
	}

	// fill missing leading records
	if node.Rownum() == 1 {
		fromWindow := time.Unix(0, (tw.tsFrom.UnixNano()/int64(tw.period)-1)*int64(tw.period))
		tw.Fill(node, fromWindow, recWindow)
	}

	// window changed, yield buffered values
	if curWindow != recWindow {
		tw.Flush(node, curWindow)
		tw.Fill(node, curWindow, recWindow)
		// update processing window
		node.SetValue("curWindow", recWindow)
	}

	// append buffered values
	for i, v := range values {
		if i == tw.timeIdx {
			continue
		}
		serName := tw.SeriesName(i)
		if ser, ok := node.GetValue(serName); !ok {
			node.SetValue(serName, []any{v})
		} else {
			series := ser.([]any)
			node.SetValue(serName, append(series, v))
		}
	}
	return nil
}

type TimeWindow struct {
	tsFrom       time.Time
	tsUntil      time.Time
	period       time.Duration
	aggregations []string
	timeIdx      int
	nullValue    *NullValue
}

func NewTimeWindow() *TimeWindow {
	return &TimeWindow{
		aggregations: []string{},
		timeIdx:      -1,
		nullValue:    &NullValue{altValue: nil},
	}
}

func (tw *TimeWindow) SeriesName(i int) string {
	return fmt.Sprintf("series%d", i)
}

func (tw *TimeWindow) IsInRange(ts time.Time) bool {
	return ts.Sub(tw.tsFrom) >= 0 && ts.Sub(tw.tsUntil) < 0
}

func (tw *TimeWindow) Flush(node *Node, curWindow time.Time) {
	// aggregation
	ret := make([]any, len(tw.aggregations))
	for i := range ret {
		if i == tw.timeIdx {
			ret[i] = curWindow
			continue
		}
		serName := tw.SeriesName(i)
		ser, ok := node.GetValue(serName)
		if !ok {
			break
		}
		series := ser.([]any)
		if len(series) == 0 {
			ret[i] = tw.nullValue.Value()
		} else {
			switch tw.aggregations[i] {
			case "first":
				ret[i] = timewindowFirst(series)
			case "last":
				ret[i] = timewindowLast(series)
			case "avg":
				ret[i] = timewindowAvg(series)
			case "max":
				ret[i] = timewindowMax(series)
			case "min":
				ret[i] = timewindowMin(series)
			case "sum":
				ret[i] = timewindowSum(series)
			case "rss":
				ret[i] = timewindowRss(series)
			default:
				ret[i] = timewindowLast(series)
			}
		}
		// clear
		node.DeleteValue(serName)
	}
	// yield
	node.yield(curWindow, ret)
}

func (tw *TimeWindow) Fill(node *Node, curWindow time.Time, nextWindow time.Time) {
	curWindow = curWindow.Add(tw.period)
	for nextWindow.Sub(curWindow) >= tw.period {
		ret := make([]any, len(tw.aggregations))
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

func timewindowFirst(values []any) any {
	return values[0]
}

func timewindowLast(values []any) any {
	return values[len(values)-1]
}

func timewindowAvg(values []any) any {
	var sum float64
	for _, v := range values {
		f, err := util.ToFloat64(v)
		if err != nil {
			return values[len(values)-1]
		}
		sum += f
	}
	return sum / float64(len(values))
}

func timewindowSum(values []any) any {
	var ret float64
	for _, v := range values {
		f, err := util.ToFloat64(v)
		if err != nil {
			return values[len(values)-1]
		}
		ret += f
	}
	return ret
}

func timewindowMax(values []any) any {
	var ret float64
	for _, v := range values {
		f, err := util.ToFloat64(v)
		if err != nil {
			return values[len(values)-1]
		}
		if f > ret {
			ret = f
		}
	}
	return ret
}

func timewindowMin(values []any) any {
	var ret float64
	for i, v := range values {
		f, err := util.ToFloat64(v)
		if err != nil {
			return values[len(values)-1]
		}
		if i == 0 || f < ret {
			ret = f
		}
	}
	return ret
}

func timewindowRss(values []any) any {
	var sum float64
	for _, v := range values {
		f, err := util.ToFloat64(v)
		if err != nil {
			return values[len(values)-1]
		}
		sum += f * f
	}
	return math.Sqrt(sum)
}
