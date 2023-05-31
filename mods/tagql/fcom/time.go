package fcom

import (
	"fmt"
	"time"
)

func to_time(args ...any) (any, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("f(time) invalid number of args (n:%d)", len(args))
	}

	var baseTime time.Time
	var delta time.Duration

	if str, ok := args[0].(string); ok {
		if str == "now" {
			baseTime = time.Now()
		} else {
			return nil, fmt.Errorf("f(time) first args should be time, but %s", args[0])
		}
	} else if d, ok := args[0].(float64); ok {
		epoch := int64(d)
		baseTime = time.Unix(0, epoch)
	} else if t, ok := args[0].(time.Time); ok {
		baseTime = t
	} else if t, ok := args[0].(*time.Time); ok {
		baseTime = *t
	} else {
		return nil, fmt.Errorf("f(time) first args should be time, but %T", args[0])
	}

	if len(args) == 2 {
		if str, ok := args[1].(string); ok {
			d, err := time.ParseDuration(str)
			if err != nil {
				return nil, fmt.Errorf("f(time) second args should be duration, but %s", args[1])
			}
			delta = d
		} else if d, ok := args[0].(float64); ok {
			epoch := int64(d)
			delta = time.Duration(epoch)
		} else {
			return nil, fmt.Errorf("f(time) second args should be duration, but %T", args[1])
		}
	}

	fmt.Printf("%v      %v\n", baseTime, delta)
	return baseTime.Add(delta), nil
}

// `roundTime(time, duration)`
func roundTime(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("f(roundTime) invalud args 'roundTime(time, 'duration')' (n:%d)", len(args))
	}
	var dur time.Duration
	if str, ok := args[1].(string); ok {
		if d, err := time.ParseDuration(str); err != nil {
			return nil, fmt.Errorf("f(roundTime) 2nd arg should be duration")
		} else {
			dur = d
		}
	} else if num, ok := args[1].(float64); ok {
		dur = time.Duration(int64(num))
	}
	if dur == 0 {
		return nil, fmt.Errorf("f(roundTime) zero duration")
	}

	var ret time.Time
	if ts, ok := args[0].(time.Time); ok {
		ret = time.Unix(0, (ts.UnixNano()/int64(dur))*int64(dur))
	} else if ts, ok := args[0].(*time.Time); ok {
		ret = time.Unix(0, (ts.UnixNano()/int64(dur))*int64(dur))
	} else {
		return nil, fmt.Errorf("f(roundTime) 1st arg should be time, but %T", args[0])
	}
	return ret, nil
}

// `round(number, number)`
func round(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("f(round) invalud args 'round(int, int)' (n:%d)", len(args))
	}
	var num int64
	var mod int64
	if d, ok := args[0].(int64); ok {
		num = d
	} else {
		return nil, fmt.Errorf("f(round) args should be non-zero int")
	}
	if d, ok := args[1].(int64); ok {
		mod = d
	} else {
		return nil, fmt.Errorf("f(round) args should be non-zero int")
	}

	return (num / mod) * mod, nil
}
