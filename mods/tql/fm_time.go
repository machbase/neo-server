package tql

import (
	"fmt"
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

func (node *Node) fmTimeWindow(from any, until any, duration any, args ...any) any {
	var tsFrom, tsUntil time.Time
	var period time.Duration
	var aggregations = []string{}
	var timeIdx = -1
	var nilValue = 0.0

	if node.Rownum() == 1 {
		if ts, err := util.ToTime(from); err != nil {
			return ErrArgs("TIMEWINDOW", 0, fmt.Sprintf("from is not compatible type, %T", from))
		} else {
			tsFrom = ts
		}
		if ts, err := util.ToTime(until); err != nil {
			return ErrArgs("TIMEWINDOW", 1, fmt.Sprintf("until is not compatible type, %T", until))
		} else {
			tsUntil = ts
		}
		if d, err := util.ToDuration(duration); err != nil {
			return ErrArgs("TIMEWINDOW", 2, fmt.Sprintf("duration is not compatible, %T", duration))
		} else if d == 0 {
			return ErrArgs("TIMEWINDOW", 2, "duration is zero")
		} else {
			period = d
		}
		if tsUntil.Sub(tsFrom) <= period {
			return ErrArgs("TIMEWINDOW", 0, "from ~ until should be larger than period")
		}
		for i, arg := range args {
			switch v := arg.(type) {
			case string:
				aggregations = append(aggregations, v)
				if v == "time" {
					timeIdx = i
				}
			default:
				return ErrArgs("TIMEWINDOW", 3, fmt.Sprintf("column name invalid type, %T", v))
			}
		}
		if len(aggregations) < 2 || timeIdx == -1 {
			return ErrArgs("TIMEWINDOW", 3, "invalid columns count or no time column specified")
		}
		node.SetValue("from", tsFrom)
		node.SetValue("until", tsUntil)
		node.SetValue("period", period)
		node.SetValue("aggregations", aggregations)
		node.SetValue("timeIdx", timeIdx)
		node.SetFeedEOF(true)
	} else {
		f, _ := node.GetValue("from")
		tsFrom = f.(time.Time)
		t, _ := node.GetValue("until")
		tsUntil = t.(time.Time)
		p, _ := node.GetValue("period")
		period = p.(time.Duration)
		a, _ := node.GetValue("aggregations")
		aggregations = a.([]string)
		i, _ := node.GetValue("timeIdx")
		timeIdx = i.(int)
	}

	if node.Inflight().IsEOF() {
		// flush remain values
		var curWindow time.Time
		if t, ok := node.GetValue("curWindow"); !ok {
			return nil
		} else {
			curWindow = t.(time.Time)
		}
		timewindow_flush(node, curWindow, aggregations, timeIdx, nilValue)
		timewindow_fill(node, curWindow, period, tsUntil, nilValue, aggregations, timeIdx)
		return nil
	}

	var values []any
	if v, ok := node.Inflight().value.([]any); ok {
		values = v
	} else {
		return ErrorRecord(fmt.Errorf("TIMEWINDOW value should be array"))
	}
	if len(aggregations) != len(values) {
		return ErrorRecord(fmt.Errorf("TIMEWINDOW column count does not match %d", len(values)))
	}

	var ts time.Time
	if v, err := util.ToTime(values[timeIdx]); err != nil {
		return ErrorRecord(err)
	} else {
		ts = v
	}

	// recWindow value of the current record
	var recWindow = time.Unix(0, (ts.UnixNano()/int64(period))*int64(period))

	// current processing window
	var curWindow time.Time
	if w, ok := node.GetValue("curWindow"); !ok {
		node.SetValue("curWindow", recWindow)
		curWindow = recWindow
	} else {
		curWindow = w.(time.Time)
	}

	// out of range
	if curWindow.Sub(tsFrom) < 0 || curWindow.Sub(tsUntil) >= 0 {
		return nil
	}

	// fill missing leading records
	if node.Rownum() == 1 {
		fromWindow := time.Unix(0, (tsFrom.UnixNano()/int64(period)-1)*int64(period))
		timewindow_fill(node, fromWindow, period, recWindow, nilValue, aggregations, timeIdx)
	}

	// window changed, yield buffere values
	if curWindow != recWindow {
		timewindow_flush(node, curWindow, aggregations, timeIdx, nilValue)
		timewindow_fill(node, curWindow, period, recWindow, nilValue, aggregations, timeIdx)
		// update processing window
		node.SetValue("curWindow", recWindow)
	}
	// append buffered values
	for i, v := range values {
		if i == timeIdx {
			continue
		}
		serName := timewindow_seriesname(i)
		if ser, ok := node.GetValue(serName); !ok {
			node.SetValue(serName, []any{v})
		} else {
			series := ser.([]any)
			node.SetValue(serName, append(series, v))
		}
	}
	return nil
}

func timewindow_seriesname(i int) string {
	return fmt.Sprintf("series%d", i)
}

func timewindow_flush(node *Node, curWindow time.Time, aggregations []string, timeIdx int, nullValue any) {
	// aggregation
	ret := make([]any, len(aggregations))
	for i := range ret {
		if i == timeIdx {
			ret[i] = curWindow
			continue
		}
		serName := timewindow_seriesname(i)
		ser, ok := node.GetValue(serName)
		if !ok {
			break
		}
		series := ser.([]any)
		ret[i] = timewindowResult(series, aggregations[i], nullValue)
		// clear
		node.DeleteValue(serName)
	}
	// yield
	node.yield(curWindow, ret)
}

func timewindow_fill(node *Node, curWindow time.Time, period time.Duration, nextWindow time.Time, nilValue any, aggregations []string, timeIdx int) {
	curWindow = curWindow.Add(period)
	for nextWindow.Sub(curWindow) >= period {
		ret := make([]any, len(aggregations))
		for i := range ret {
			if i == timeIdx {
				ret[i] = curWindow
			} else {
				ret[i] = nilValue
			}
		}
		node.yield(curWindow, ret)
		curWindow = curWindow.Add(period)
	}
}

func timewindowResult(series []any, aggregation string, nullValue any) any {
	switch aggregation {
	case "first":
		return timewindowFirst(series, nullValue)
	case "last":
		return timewindowLast(series, nullValue)
	case "avg":
		return timewindowAvg(series, nullValue)
	case "max":
		return timewindowMax(series, nullValue)
	case "min":
		return timewindowMin(series, nullValue)
	case "sum":
		return timewindowSum(series, nullValue)
	default:
		return timewindowLast(series, nullValue)
	}
}

func timewindowFirst(values []any, nullValue any) any {
	if len(values) == 0 {
		return nullValue
	}
	return values[0]
}

func timewindowLast(values []any, nullValue any) any {
	if len(values) == 0 {
		return nullValue
	}
	return values[len(values)-1]
}

func timewindowAvg(values []any, nullValue any) any {
	if len(values) == 0 {
		return nullValue
	}
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

func timewindowSum(values []any, nullValue any) any {
	if len(values) == 0 {
		return nullValue
	}
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

func timewindowMax(values []any, nullValue any) any {
	if len(values) == 0 {
		return nullValue
	}
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

func timewindowMin(values []any, nullValue any) any {
	if len(values) == 0 {
		return nullValue
	}
	var ret float64
	for _, v := range values {
		f, err := util.ToFloat64(v)
		if err != nil {
			return values[len(values)-1]
		}
		if f < ret {
			ret = f
		}
	}
	return ret
}
