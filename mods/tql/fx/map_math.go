package fx

import (
	"math"

	"github.com/machbase/neo-server/mods/nums"
)

var MathDefinitions = []Definition{
	{"// math", nil}, // math
	{"sin", math.Sin},
	{"cos", math.Cos},
	{"tan", math.Tan},
	{"exp", math.Exp},
	{"exp2", math.Exp2},
	{"log", math.Log},
	{"log10", math.Log10},
	{"// nums", nil}, // nums
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
	// other
}
