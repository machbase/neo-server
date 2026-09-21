package tql

import (
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/util"
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
