package fcom

import (
	"math"

	"github.com/machbase/neo-server/mods/tql/conv"
)

func sin(args ...any) (any, error) {
	if v, err := conv.Float64(args, 0, "sin", "float"); err != nil {
		return nil, err
	} else {
		return math.Sin(v), nil
	}
}

func cos(args ...any) (any, error) {
	if v, err := conv.Float64(args, 0, "cos", "float"); err != nil {
		return nil, err
	} else {
		return math.Cos(v), nil
	}
}

func tan(args ...any) (any, error) {
	if v, err := conv.Float64(args, 0, "tan", "float"); err != nil {
		return nil, err
	} else {
		return math.Tan(v), nil
	}
}

func exp(args ...any) (any, error) {
	if v, err := conv.Float64(args, 0, "exp", "float"); err != nil {
		return nil, err
	} else {
		return math.Exp(v), nil
	}
}

func exp2(args ...any) (any, error) {
	if v, err := conv.Float64(args, 0, "exp2", "float"); err != nil {
		return nil, err
	} else {
		return math.Exp2(v), nil
	}
}

func log(args ...any) (any, error) {
	if v, err := conv.Float64(args, 0, "log", "float"); err != nil {
		return nil, err
	} else {
		return math.Log(v), nil
	}
}

func log10(args ...any) (any, error) {
	if v, err := conv.Float64(args, 0, "log10", "float"); err != nil {
		return nil, err
	} else {
		return math.Log10(v), nil
	}
}

func linspace(args ...any) (any, error) {
	var start float64
	var stop float64
	num := 50

	if v, err := conv.Float64(args, 0, "linspace", "float"); err != nil {
		return nil, err
	} else {
		start = v
	}
	if v, err := conv.Float64(args, 1, "linspace", "float"); err != nil {
		return nil, err
	} else {
		stop = v
	}
	if len(args) >= 3 {
		if v, err := conv.Int(args, 2, "linspace", "int"); err != nil {
			return nil, err
		} else {
			num = v
		}
	}
	ret := make([]float64, num)
	step := (stop - start) / float64(num-1)
	for i := range ret {
		ret[i] = start + float64(i)*step
	}
	ret[len(ret)-1] = stop
	return ret, nil
}

func meshgrid(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, conv.ErrInvalidNumOfArgs("meshgrid", 2, len(args))
	}
	x, ok := args[0].([]float64)
	if !ok {
		return nil, conv.ErrWrongTypeOfArgs("meshgrid", 0, "[]float", args[0])
	}
	y, ok := args[1].([]float64)
	if !ok {
		return nil, conv.ErrWrongTypeOfArgs("meshgrid", 1, "[]float", args[1])
	}

	ret := make([][][]float64, len(x))

	for i := 0; i < len(x); i++ {
		ret[i] = make([][]float64, len(y))
		for n := 0; n < len(y); n++ {
			ret[i][n] = []float64{x[i], y[n]}
		}
	}
	return ret, nil
}
