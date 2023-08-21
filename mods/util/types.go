package util

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func ErrIncompatible(dstType string, src any) error {
	return fmt.Errorf("incompatible conv '%v' (%T) to %s", src, src, dstType)
}

var StandardTimeNow func() time.Time = time.Now

// ToTime converts to time.Time
//
// ex) "now" converts into current time,
//
// ex) "now + 1s", "now - 1h"
func ToTime(one any) (time.Time, error) {
	switch val := one.(type) {
	case time.Time:
		return val, nil
	case *time.Time:
		return *val, nil
	case string:
		return ParseTime(val, "", nil)
	case *string:
		return ParseTime(*val, "", nil)
	case float64:
		return time.Unix(0, int64(val)), nil
	case *float64:
		return time.Unix(0, int64(*val)), nil
	case int64:
		return time.Unix(0, val), nil
	case *int64:
		return time.Unix(0, *val), nil
	case int32:
		return time.Unix(0, int64(val)), nil
	case *int32:
		return time.Unix(0, int64(*val)), nil
	case int16:
		return time.Unix(0, int64(val)), nil
	case *int16:
		return time.Unix(0, int64(*val)), nil
	case int8:
		return time.Unix(0, int64(val)), nil
	case *int8:
		return time.Unix(0, int64(*val)), nil
	case int:
		return time.Unix(0, int64(val)), nil
	case *int:
		return time.Unix(0, int64(*val)), nil
	default:
		return time.Time{}, ErrIncompatible("time.Time", val)
	}
}

func ParseTime(strval string, format string, location *time.Location) (time.Time, error) {
	var baseTime time.Time
	strval = strings.TrimSpace(strval)
	if strings.HasPrefix(strval, "now") {
		baseTime = StandardTimeNow()
		sig := time.Duration(1)
		remain := strings.TrimSpace(strval[3:])
		if len(remain) == 0 {
			return baseTime, nil
		}
		if strings.HasPrefix(remain, "+") {
			remain = strings.TrimSpace(remain[1:])
		} else if strings.HasPrefix(remain, "-") {
			sig = time.Duration(-1)
			remain = strings.TrimSpace(remain[1:])
		} else {
			return baseTime, ErrIncompatible("time.Time", strval)
		}
		dur, err := ToDuration(remain)
		if err != nil {
			return baseTime, fmt.Errorf("incompatible conv '%s', %s", strval, err.Error())
		}
		baseTime = baseTime.Add(dur * sig)
		return baseTime, nil
	}
	if format == "" {
		return baseTime, ErrIncompatible("time.Time", strval)
	}

	timeLayout := GetTimeformat(format)
	var ts int64
	var err error
	switch timeLayout {
	case "s":
		if ts, err = strconv.ParseInt(strval, 10, 64); err != nil {
			return time.Time{}, errors.Wrap(err, "unable parse time in timeformat")
		}
		return time.Unix(ts, 0), nil
	case "ms":
		var ts int64
		if ts, err = strconv.ParseInt(strval, 10, 64); err != nil {
			return time.Time{}, errors.Wrap(err, "unable parse time in timeformat")
		}
		return time.Unix(0, ts*int64(time.Millisecond)), nil
	case "us":
		var ts int64
		if ts, err = strconv.ParseInt(strval, 10, 64); err != nil {
			return time.Time{}, errors.Wrap(err, "unable parse time in timeformat")
		}
		return time.Unix(0, ts*int64(time.Microsecond)), nil
	case "ns": // "ns"
		var ts int64
		if ts, err = strconv.ParseInt(strval, 10, 64); err != nil {
			return time.Time{}, errors.Wrap(err, "unable parse time in timeformat")
		}
		return time.Unix(0, ts), nil
	default:
		baseTime, err = time.ParseInLocation(timeLayout, strval, location)
		if err != nil {
			return baseTime, errors.Wrap(err, ErrIncompatible("time.Time", strval).Error())
		}
	}
	return baseTime, nil
}

func ToDuration(one any) (time.Duration, error) {
	switch val := one.(type) {
	case string:
		return ParseDuration(val)
	case *string:
		return ParseDuration(*val)
	case float64:
		return time.Duration(int64(val)), nil
	case *float64:
		return time.Duration(int64(*val)), nil
	case float32:
		return time.Duration(int64(val)), nil
	case *float32:
		return time.Duration(int64(*val)), nil
	case int64:
		return time.Duration(val), nil
	case *int64:
		return time.Duration(*val), nil
	case int32:
		return time.Duration(val), nil
	case *int32:
		return time.Duration(*val), nil
	case int16:
		return time.Duration(val), nil
	case *int16:
		return time.Duration(*val), nil
	case int8:
		return time.Duration(val), nil
	case *int8:
		return time.Duration(*val), nil
	case int:
		return time.Duration(val), nil
	case *int:
		return time.Duration(*val), nil
	default:
		return 0, ErrIncompatible("time.Duration", val)
	}
}

func ParseDuration(val string) (time.Duration, error) {
	if i := strings.IndexRune(val, 'd'); i > 0 {
		var day time.Duration = 0
		digit := val[0:i]
		str := val[i+1:]
		d, err := strconv.ParseInt(digit, 10, 64)
		if err != nil {
			return 0, ErrIncompatible("time.Duration", val)
		}
		day = time.Duration(d) * 24 * time.Hour
		if len(str) > 0 {
			if dur, err := time.ParseDuration(str); err != nil {
				return 0, ErrIncompatible("time.Duration", val)
			} else if day >= 0 {
				return day + dur, nil
			} else {
				return day - dur, nil
			}
		} else {
			return day, nil
		}
	}
	if d, err := time.ParseDuration(val); err != nil {
		return 0, err
	} else {
		return d, nil
	}
}

func ToFloat32(one any) (float32, error) {
	switch val := one.(type) {
	case string:
		return ParseFloat32(val)
	case *string:
		return ParseFloat32(*val)
	case float32:
		return val, nil
	case *float32:
		return *val, nil
	case float64:
		return float32(val), nil
	case *float64:
		return float32(*val), nil
	case int:
		return float32(val), nil
	case *int:
		return float32(*val), nil
	default:
		return 0, ErrIncompatible("float32", val)
	}
}

func ParseFloat32(val string) (float32, error) {
	if val, err := strconv.ParseFloat(val, 32); err != nil {
		return 0, fmt.Errorf("incompatible conv '%v' (%T) to float32, %s", val, val, err.Error())
	} else {
		return float32(val), nil
	}
}

func ToFloat64(one any) (float64, error) {
	switch val := one.(type) {
	case string:
		return ParseFloat64(val)
	case *string:
		return ParseFloat64(*val)
	case float32:
		return float64(val), nil
	case *float32:
		return float64(*val), nil
	case float64:
		return val, nil
	case *float64:
		return *val, nil
	case int:
		return float64(val), nil
	case *int:
		return float64(*val), nil
	default:
		return 0, ErrIncompatible("float64", val)
	}
}

func ParseFloat64(val string) (float64, error) {
	if val, err := strconv.ParseFloat(val, 64); err != nil {
		return 0, fmt.Errorf("incompatible conv '%v' (%T) to float64, %s", val, val, err.Error())
	} else {
		return val, nil
	}
}

func ParseInt(val string) (int, error) {
	d, err := strconv.ParseInt(val, 10, 32)
	return int(d), err
}

func ParseInt8(val string) (int8, error) {
	d, err := strconv.ParseInt(val, 10, 8)
	return int8(d), err
}

func ParseInt16(val string) (int16, error) {
	d, err := strconv.ParseInt(val, 10, 16)
	return int16(d), err
}

func ParseInt32(val string) (int32, error) {
	d, err := strconv.ParseInt(val, 10, 32)
	return int32(d), err
}

func ParseInt64(val string) (int64, error) {
	return strconv.ParseInt(val, 10, 64)
}

func ParseIP(val string) (net.IP, error) {
	addr := net.ParseIP(val)
	if addr == nil {
		return nil, fmt.Errorf("incompatible conv '%v' (%T) to IP", val, val)
	}
	return addr, nil
}