package fcom

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var StandardTimeNow func() time.Time = time.Now

func to_time(args ...any) (any, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("f(time) invalid number of args (n:%d)", len(args))
	}

	var baseTime time.Time
	var delta time.Duration

	if str, ok := args[0].(string); ok {
		if strings.HasPrefix(str, "now") {
			baseTime = StandardTimeNow()
			remain := strings.TrimSpace(str[3:])
			if len(remain) > 0 {
				dur, err := time.ParseDuration(remain)
				if err != nil {
					return nil, fmt.Errorf("f(time) %s", err.Error())
				}
				baseTime = baseTime.Add(dur)
			}
		} else {
			return nil, fmt.Errorf("f(time) first arg should be time, but %s", args[0])
		}
	} else if d, ok := args[0].(float64); ok {
		epoch := int64(d)
		baseTime = time.Unix(0, epoch)
	} else if t, ok := args[0].(time.Time); ok {
		baseTime = t
	} else if t, ok := args[0].(*time.Time); ok {
		baseTime = *t
	} else {
		return nil, fmt.Errorf("f(time) first arg should be time, but %T", args[0])
	}

	if len(args) == 2 {
		if str, ok := args[1].(string); ok {
			var sig int64 = 1
			var day int64 = 0
			var hour int64 = 0
			if i := strings.IndexRune(str, 'd'); i > 0 {
				digit := str[0:i]
				str = str[i+1:]
				d, err := strconv.ParseInt(digit, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("f(time) second arg should be duration, but %s", args[1])
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
					return nil, fmt.Errorf("f(time) second arg should be duration, but %s", args[1])
				}
				hour = int64(h)
			}
			delta = time.Duration(sig * (day*24*int64(time.Hour) + int64(hour)))
		} else if d, ok := args[0].(float64); ok {
			epoch := int64(d)
			delta = time.Duration(epoch)
		} else {
			return nil, fmt.Errorf("f(time) second arg should be duration, but %T", args[1])
		}
	}

	return baseTime.Add(delta), nil
}
