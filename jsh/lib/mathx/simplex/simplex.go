package simplex

import (
	"context"
	_ "embed"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/nums/opensimplex"
)

//go:embed simplex.js
var simplex_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"mathx/simplex.js": simplex_js,
	}
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)
	o.Set("seed", Seed)
}

type SimpleX struct {
	seed int64
	gen  *opensimplex.Generator
}

func Seed(seed int64) *SimpleX {
	return &SimpleX{
		seed: seed,
	}
}

func (s *SimpleX) Eval(dim ...float64) float64 {
	if s.gen == nil {
		s.gen = opensimplex.New(s.seed)
	}
	return s.gen.Eval(dim...)
}
