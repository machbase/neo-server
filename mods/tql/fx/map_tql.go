package fx

import (
	"math"

	"github.com/machbase/neo-server/mods/nums"
	"github.com/machbase/neo-server/mods/tql/maps"
)

var TqlDefinitions = []Definition{
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
	// other
}
