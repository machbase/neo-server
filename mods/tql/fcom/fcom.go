package fcom

import (
	"math"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/conv"
)

var Functions = map[string]expression.Function{
	"len":       to_len,    // len( array| string)
	"count":     count,     // count(V)
	"time":      to_time,   // time(ts [, delta])
	"roundTime": roundTime, // roundTime(time, duration)
	"element":   element,   // element(list, idx)
}

type Definition struct {
	Name string
	Func any
}

var Definitions = []Definition{
	{"sin", math.Sin},
	{"cos", math.Cos},
	{"tan", math.Tan},
	{"exp", math.Exp},
	{"exp2", math.Exp2},
	{"log", math.Log},
	{"log10", math.Log10},
	{"round", Round},
	{"linspace", Linspace},
	{"meshgrid", Meshgrid},
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

// `round(number, number)`
func Round(num int64, mod int64) float64 {
	if mod == 0 {
		return math.NaN()
	}
	return float64((num / mod) * mod)
}
