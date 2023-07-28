//go:generate go run generate.go

package fcom

import (
	"math"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/conv"
)

var GenFunctions = map[string]expression.Function{
	"sin":      gen_sin,
	"cos":      gen_cos,
	"tan":      gen_tan,
	"exp":      gen_exp,
	"exp2":     gen_exp2,
	"log":      gen_log,
	"log10":    gen_log10,
	"round":    gen_round,
	"linspace": gen_linspace,
	"meshgrid": gen_meshgrid,
}

// gen_sin
//
// syntax: sin(float64)
func gen_sin(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("sin", 1, len(args))
	}
	p0, err := conv.Float64(args, 0, "sin", "float64")
	if err != nil {
		return nil, err
	}
	ret := math.Sin(p0)
	return ret, nil
}

// gen_cos
//
// syntax: cos(float64)
func gen_cos(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("cos", 1, len(args))
	}
	p0, err := conv.Float64(args, 0, "cos", "float64")
	if err != nil {
		return nil, err
	}
	ret := math.Cos(p0)
	return ret, nil
}

// gen_tan
//
// syntax: tan(float64)
func gen_tan(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("tan", 1, len(args))
	}
	p0, err := conv.Float64(args, 0, "tan", "float64")
	if err != nil {
		return nil, err
	}
	ret := math.Tan(p0)
	return ret, nil
}

// gen_exp
//
// syntax: exp(float64)
func gen_exp(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("exp", 1, len(args))
	}
	p0, err := conv.Float64(args, 0, "exp", "float64")
	if err != nil {
		return nil, err
	}
	ret := math.Exp(p0)
	return ret, nil
}

// gen_exp2
//
// syntax: exp2(float64)
func gen_exp2(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("exp2", 1, len(args))
	}
	p0, err := conv.Float64(args, 0, "exp2", "float64")
	if err != nil {
		return nil, err
	}
	ret := math.Exp2(p0)
	return ret, nil
}

// gen_log
//
// syntax: log(float64)
func gen_log(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("log", 1, len(args))
	}
	p0, err := conv.Float64(args, 0, "log", "float64")
	if err != nil {
		return nil, err
	}
	ret := math.Log(p0)
	return ret, nil
}

// gen_log10
//
// syntax: log10(float64)
func gen_log10(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("log10", 1, len(args))
	}
	p0, err := conv.Float64(args, 0, "log10", "float64")
	if err != nil {
		return nil, err
	}
	ret := math.Log10(p0)
	return ret, nil
}

// gen_round
//
// syntax: round(int64, int64)
func gen_round(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, conv.ErrInvalidNumOfArgs("round", 2, len(args))
	}
	p0, err := conv.Int64(args, 0, "round", "int64")
	if err != nil {
		return nil, err
	}
	p1, err := conv.Int64(args, 1, "round", "int64")
	if err != nil {
		return nil, err
	}
	ret := Round(p0, p1)
	return ret, nil
}

// gen_linspace
//
// syntax: linspace(float64, float64, OptionInt)
func gen_linspace(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, conv.ErrInvalidNumOfArgs("linspace", 3, len(args))
	}
	p0, err := conv.Float64(args, 0, "linspace", "float64")
	if err != nil {
		return nil, err
	}
	p1, err := conv.Float64(args, 1, "linspace", "float64")
	if err != nil {
		return nil, err
	}
	p2 := conv.EmptyInt()
	if len(args) >= 3 {
		v, err := conv.Int(args, 2, "linspace", "OptionInt")
		if err != nil {
			return nil, err
		} else {
			p2 = conv.OptionInt{Value: v}
		}
	}
	return Linspace(p0, p1, p2)
}

// gen_meshgrid
//
// syntax: meshgrid([]float64, []float64)
func gen_meshgrid(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, conv.ErrInvalidNumOfArgs("meshgrid", 2, len(args))
	}
	p0, ok := args[0].([]float64)
	if !ok {
		return nil, conv.ErrWrongTypeOfArgs("meshgrid", 0, "[]float64", args[0])
	}
	p1, ok := args[1].([]float64)
	if !ok {
		return nil, conv.ErrWrongTypeOfArgs("meshgrid", 1, "[]float64", args[1])
	}
	return Meshgrid(p0, p1)
}
