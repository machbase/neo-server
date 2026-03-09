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
	o.Set("PiecewiseConstant", new_piecewiseConstant(rt))
	// m.PiecewiseLinear()
	o.Set("PiecewiseLinear", new_piecewiseLinear(rt))
	// m.AkimaSpline()
	o.Set("AkimaSpline", new_akimaSpline(rt))
	// m.FritschButland()
	o.Set("FritschButland", new_fritschButland(rt))
	// m.LinearRegression()
	o.Set("LinearRegression", new_linearRegression(rt))
	// m.ClampedCubic()
	o.Set("ClampedCubic", new_clampedCubic(rt))
	// m.NaturalCubic()
	o.Set("NaturalCubic", new_naturalCubic(rt))
	// m.NotAKnotCubic()
	o.Set("NotAKnotCubic", new_notAKnotCubic(rt))
}

func new_piecewiseConstant(rt *goja.Runtime) func(c goja.ConstructorCall) *goja.Object {
	return func(c goja.ConstructorCall) *goja.Object {
		return newInterpolator(rt, &interp.PiecewiseConstant{})
	}
}

func new_piecewiseLinear(rt *goja.Runtime) func(c goja.ConstructorCall) *goja.Object {
	return func(c goja.ConstructorCall) *goja.Object {
		return newInterpolator(rt, &interp.PiecewiseLinear{})
	}
}

func new_akimaSpline(rt *goja.Runtime) func(c goja.ConstructorCall) *goja.Object {
	return func(c goja.ConstructorCall) *goja.Object {
		return newInterpolator(rt, &interp.AkimaSpline{})
	}
}

func new_fritschButland(rt *goja.Runtime) func(c goja.ConstructorCall) *goja.Object {
	return func(c goja.ConstructorCall) *goja.Object {
		return newInterpolator(rt, &interp.FritschButland{})
	}
}

func new_linearRegression(rt *goja.Runtime) func(c goja.ConstructorCall) *goja.Object {
	return func(c goja.ConstructorCall) *goja.Object {
		return newInterpolator(rt, &LinearRegression{})
	}
}

func new_clampedCubic(rt *goja.Runtime) func(c goja.ConstructorCall) *goja.Object {
	return func(c goja.ConstructorCall) *goja.Object {
		return newInterpolator(rt, &interp.ClampedCubic{})
	}
}

func new_naturalCubic(rt *goja.Runtime) func(c goja.ConstructorCall) *goja.Object {
	return func(c goja.ConstructorCall) *goja.Object {
		return newInterpolator(rt, &interp.NaturalCubic{})
	}
}

func new_notAKnotCubic(rt *goja.Runtime) func(c goja.ConstructorCall) *goja.Object {
	return func(c goja.ConstructorCall) *goja.Object {
		return newInterpolator(rt, &interp.NotAKnotCubic{})
	}
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

type Interpolator struct {
	rt     *goja.Runtime
	interp interface {
		Fit(xs, ys []float64) error
		Predict(x float64) float64
	}
}

func newInterpolator(rt *goja.Runtime, interp interface {
	Fit(xs, ys []float64) error
	Predict(x float64) float64
}) *goja.Object {
	ip := &Interpolator{rt: rt, interp: interp}
	obj := rt.NewObject()
	obj.Set("fit", ip.Fit)
	obj.Set("predict", ip.Predict)
	if _, ok := interp.(interface {
		PredictDerivative(x float64) float64
	}); ok {
		obj.Set("predictDerivative", ip.PredictDerivative)
	}
	return obj
}

func (ip *Interpolator) Fit(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) != 2 {
		panic(ip.rt.ToValue("fit: x and y are required"))
	}
	var x, y []float64
	if err := ip.rt.ExportTo(call.Arguments[0], &x); err != nil {
		panic(ip.rt.ToValue(fmt.Sprintf("fit: %v", err)))
	}
	if err := ip.rt.ExportTo(call.Arguments[1], &y); err != nil {
		panic(ip.rt.ToValue(fmt.Sprintf("fit: %v", err)))
	}
	if len(x) != len(y) {
		panic(ip.rt.ToValue("fit: x and y should be the same length"))
	}
	ip.interp.Fit(x, y)
	return goja.Undefined()
}

func (ip *Interpolator) Predict(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) != 1 {
		panic(ip.rt.ToValue("predict: x is required"))
	}
	x := call.Arguments[0].ToFloat()
	return ip.rt.ToValue(ip.interp.Predict(x))
}

func (ip *Interpolator) PredictDerivative(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) != 1 {
		panic(ip.rt.ToValue("predictDerivative: x is required"))
	}
	x := call.Arguments[0].ToFloat()
	if derivative, ok := ip.interp.(interface {
		PredictDerivative(x float64) float64
	}); ok {
		return ip.rt.ToValue(derivative.PredictDerivative(x))
	}
	panic(ip.rt.ToValue("predictDerivative: not supported"))
}
