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
