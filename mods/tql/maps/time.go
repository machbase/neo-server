package maps

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/tql/conv"
)

type TimeRange struct {
	Time     time.Time
	Duration time.Duration
	Period   time.Duration
}

func ToTimeRange(ts any, dur any, period ...any) (*TimeRange, error) {
	ret := &TimeRange{}
	switch val := ts.(type) {
	case string:
		if val != "now" {
			return nil, conv.ErrWrongTypeOfArgs("range", 0, "now", val)
		}
		ret.Time = StandardTimeNow()
	case float64:
		ret.Time = time.Unix(0, int64(val))
	case *float64:
		ret.Time = time.Unix(0, int64(*val))
	case int64:
		ret.Time = time.Unix(0, val)
	case *int64:
		ret.Time = time.Unix(0, *val)
	case time.Time:
		ret.Time = val
	case *time.Time:
		ret.Time = *val
	default:
		return nil, conv.ErrWrongTypeOfArgs("range", 0, "time", val)
	}
	switch val := dur.(type) {
	case string:
		if d, err := time.ParseDuration(val); err != nil {
			return nil, conv.ErrWrongTypeOfArgs("range", 1, "duration", dur)
		} else {
			ret.Duration = d
		}
	case float64:
		ret.Duration = time.Duration(int64(val))
	case *float64:
		ret.Duration = time.Duration(int64(*val))
	case int64:
		ret.Duration = time.Duration(val)
	case *int64:
		ret.Duration = time.Duration(*val)
	default:
		return nil, conv.ErrWrongTypeOfArgs("range", 1, "duration", val)
	}
	if len(period) == 0 {
		return ret, nil
	}
	switch val := period[0].(type) {
	case string:
		if d, err := time.ParseDuration(val); err != nil {
			return nil, conv.ErrWrongTypeOfArgs("range", 2, "period", val)
		} else {
			ret.Period = d
		}
	case float64:
		ret.Period = time.Duration(int64(val))
	case *float64:
		ret.Period = time.Duration(int64(*val))
	case int64:
		ret.Period = time.Duration(val)
	case *int64:
		ret.Period = time.Duration(*val)
	default:
		return nil, conv.ErrWrongTypeOfArgs("range", 2, "period", val)
	}
	if ret.Duration <= ret.Period {
		return nil, conv.ErrArgs("range", 2, "period should be smaller than duration")
	}

	return ret, nil
}

// ts : string | float64 | int64
// duration :  time.Time | *time.Time | float64 | int64
func RoundTime(ts any, duration any) (time.Time, error) {
	var dur time.Duration
	switch val := duration.(type) {
	case string:
		if d, err := time.ParseDuration(val); err != nil {
			return time.Time{}, err
		} else {
			dur = d
		}
	case float64:
		dur = time.Duration(int64(val))
	case int64:
		dur = time.Duration(int64(val))
	}
	if dur == 0 {
		return time.Time{}, fmt.Errorf("zero duration is not allowed")
	}
	var ret time.Time
	switch val := ts.(type) {
	case time.Time:
		ret = time.Unix(0, (val.UnixNano()/int64(dur))*int64(dur))
	case *time.Time:
		ret = time.Unix(0, (val.UnixNano()/int64(dur))*int64(dur))
	case float64:
		ret = time.Unix(0, (int64(val)/int64(dur))*int64(dur))
	case int64:
		ret = time.Unix(0, (int64(val)/int64(dur))*int64(dur))
	default:
		return time.Time{}, fmt.Errorf("unsupported time parameter")
	}
	return ret, nil
}

func Time(ts any) (time.Time, error) {
	return TimeAdd(ts, int64(0))
}

var StandardTimeNow func() time.Time = time.Now

func TimeAdd(tsExpr any, deltaExpr any) (time.Time, error) {
	var baseTime time.Time
	var delta time.Duration

	switch val := tsExpr.(type) {
	case string:
		if strings.HasPrefix(val, "now") {
			baseTime = StandardTimeNow()
			remain := strings.TrimSpace(val[3:])
			if len(remain) > 0 {
				dur, err := time.ParseDuration(remain)
				if err != nil {
					return baseTime, err
				}
				baseTime = baseTime.Add(dur)
			}
		} else {
			return baseTime, fmt.Errorf("invalid time expression '%s'", val)
		}
	case float64:
		baseTime = time.Unix(0, int64(val))
	case int64:
		baseTime = time.Unix(0, val)
	case time.Time:
		baseTime = val
	case *time.Time:
		baseTime = *val
	default:
		return baseTime, fmt.Errorf("invalid time expression '%v %T'", val, val)
	}

	switch val := deltaExpr.(type) {
	case string:
		var sig int64 = 1
		var day int64 = 0
		var hour int64 = 0
		var str = val
		if i := strings.IndexRune(str, 'd'); i > 0 {
			digit := str[0:i]
			str = str[i+1:]
			d, err := strconv.ParseInt(digit, 10, 64)
			if err != nil {
				return baseTime, fmt.Errorf("invalid delta expression '%v %T'", val, val)
			}
			if d < 0 {
				sig = -1
				day = d * -1
			} else {
				day = d
			}
		}
		if len(str) > 0 {
			h, err := time.ParseDuration(str)
			if err != nil {
				return baseTime, fmt.Errorf("invalid delta expression '%v %T'", val, val)
			}
			hour = int64(h)
		}
		delta = time.Duration(sig * (day*24*int64(time.Hour) + int64(hour)))
	case float64:
		delta = time.Duration(int64(val))
	case int64:
		delta = time.Duration(val)
	default:
		return baseTime, fmt.Errorf("invalid delta expression '%v %T'", val, val)
	}
	return baseTime.Add(delta), nil
}