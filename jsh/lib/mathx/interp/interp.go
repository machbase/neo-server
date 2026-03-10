package interp

import (
	_ "embed"
	"fmt"

	"github.com/dop251/goja"
	"gonum.org/v1/gonum/interp"
	"gonum.org/v1/gonum/stat"
)

//go:embed interp.js
var interp_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"mathx/interp.js": interp_js,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)

	// m.PiecewiseConstant()
	o.Set("PiecewiseConstant", func() Interpolator { return &interp.PiecewiseConstant{} })
	// m.PiecewiseLinear()
	o.Set("PiecewiseLinear", func() Interpolator { return &interp.PiecewiseLinear{} })
	// m.AkimaSpline()
	o.Set("AkimaSpline", func() Interpolator { return &interp.AkimaSpline{} })
	// m.FritschButland()
	o.Set("FritschButland", func() Interpolator { return &interp.FritschButland{} })
	// m.LinearRegression()
	o.Set("LinearRegression", func() Interpolator { return &LinearRegression{} })
	// m.ClampedCubic()
	o.Set("ClampedCubic", func() Interpolator { return &interp.ClampedCubic{} })
	// m.NaturalCubic()
	o.Set("NaturalCubic", func() Interpolator { return &interp.NaturalCubic{} })
	// m.NotAKnotCubic()
	o.Set("NotAKnotCubic", func() Interpolator { return &interp.NotAKnotCubic{} })
}

type Interpolator interface {
	Fit(xs, ys []float64) error
	Predict(x float64) float64
}

type LinearRegression struct {
	a, b float64
}

func (lr *LinearRegression) Fit(xs, ys []float64) error {
	if len(xs) != len(ys) {
		return fmt.Errorf("x and y should be the same length")
	}
	if len(xs) < 2 {
		return fmt.Errorf("x and y should have at least 2 points")
	}
	a, b := stat.LinearRegression(xs, ys, nil, false)
	if b != b {
		return fmt.Errorf("invalid regression")
	}
	lr.a = a
	lr.b = b
	return nil
}

func (lr *LinearRegression) Predict(x float64) float64 {
	if lr.b != lr.b {
		return 0
	}
	return lr.a + lr.b*x
}
