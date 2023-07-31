package fx

import (
	"math"

	"github.com/machbase/neo-server/mods/tql/fcom"
)

var MathDefinitions = []Definition{
	// math
	{"sin", math.Sin},
	{"cos", math.Cos},
	{"tan", math.Tan},
	{"exp", math.Exp},
	{"exp2", math.Exp2},
	{"log", math.Log},
	{"log10", math.Log10},
	{"round", fcom.Round},
	{"linspace", fcom.Linspace},
	{"meshgrid", fcom.Meshgrid},
	// time
	{"roundTime", fcom.RoundTime},
	// other
}
