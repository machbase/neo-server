package fcom

import (
	"fmt"
	"math"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/conv"
)

var Functions = map[string]expression.Function{
	"len":     to_len,  // len( array| string)
	"count":   count,   // count(V)
	"time":    to_time, // time(ts [, delta])
	"element": element, // element(list, idx)
}

func Mod(x, y float64) float64 {
	return math.Mod(x, y)
}

func Linspace(start float64, stop float64, optNum conv.OptionInt) (any, error) {
	num := optNum.Else(50)
	ret := make([]float64, num)
	step := (stop - start) / float64(num-1)
	for i := range ret {
		ret[i] = start + float64(i)*step
	}
	ret[len(ret)-1] = stop
	return ret, nil
}

func Meshgrid(x []float64, y []float64) (any, error) {
	ret := make([][][]float64, len(x))

	for i := 0; i < len(x); i++ {
		ret[i] = make([][]float64, len(y))
		for n := 0; n < len(y); n++ {
			ret[i][n] = []float64{x[i], y[n]}
		}
	}
	return ret, nil
}

func count(args ...any) (any, error) {
	return float64(len(args)), nil
}

// `round(number, number)`
func Round(num int64, mod int64) float64 {
	if mod == 0 {
		return math.NaN()
	}
	return float64((num / mod) * mod)
}

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
