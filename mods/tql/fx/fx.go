package fx

import (
	"math"

	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/nums"
	"github.com/machbase/neo-server/mods/tql/conv"
	"github.com/machbase/neo-server/mods/tql/maps"
)

type Definition struct {
	Name string
	Func any
}

func GetFunction(name string) expression.Function {
	return GenFunctions[name]
}

var FxDefinitions = []Definition{
	// math
	{"// math", nil},
	{"sin", math.Sin},
	{"cos", math.Cos},
	{"tan", math.Tan},
	{"exp", math.Exp},
	{"exp2", math.Exp2},
	{"log", math.Log},
	{"log10", math.Log10},
	// nums
	{"// nums", nil},
	{"count", "nums.Count"},
	{"len", "nums.Len"},
	{"element", "nums.Element"},
	{"round", nums.Round},
	{"linspace", nums.Linspace},
	{"linspace50", nums.Linspace50},
	{"meshgrid", nums.Meshgrid},
	{"roundTime", nums.RoundTime},
	{"time", nums.Time},
	{"timeAdd", nums.TimeAdd},
	// maps
	{"// maps", nil},
	{"TAKE", maps.Take},
	{"DROP", maps.Drop},
	// aliases
	{"// aliases", nil},
	{"markArea", "markArea"},
	{"markXAxis", "gen_markLineXAxisCoord"},
	{"markYAxis", "gen_markLineYAxisCoord"},
}

func markArea(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, conv.ErrInvalidNumOfArgs("markArea", 2, len(args))
	}
	var err error
	coord0 := args[0]
	coord1 := args[1]
	label := ""
	color := ""
	opacity := 1.0
	if len(args) >= 3 {
		if label, err = conv.String(args, 2, "markArea", "label"); err != nil {
			return nil, err
		}
	}
	if len(args) >= 4 {
		if color, err = conv.String(args, 3, "markArea", "color"); err != nil {
			return nil, err
		}
	}
	if len(args) >= 5 {
		if opacity, err = conv.Float64(args, 4, "markArea", "opacity"); err != nil {
			return nil, err
		}
	}
	return opts.MarkAreaNameCoord(coord0, coord1, label, color, opacity), nil
}
