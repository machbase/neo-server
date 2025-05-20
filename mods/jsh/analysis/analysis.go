package analysis

import (
	"context"
	"fmt"
	"slices"
	"time"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/machbase/neo-server/v8/mods/nums/fft"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/interp"
	"gonum.org/v1/gonum/stat"
)

func NewModuleLoader(context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/analysis")
		o := module.Get("exports").(*js.Object)
		// arr = m.sort(arr)
		o.Set("sort", func(arr []float64) js.Value {
			slices.Sort(arr)
			return rt.ToValue(arr)
		})
		// s = m.sum(arr)
		o.Set("sum", func(arr []float64) float64 {
			return floats.Sum(arr)
		})
		// m.cdf(x, weight)
		// x should be sorted, weight should be the same length as x
		o.Set("cdf", func(p float64, x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("cdf: x and weight should be the same length"))
			}
			return rt.ToValue(stat.CDF(p, stat.Empirical, x, weight))
		})
		// m.circularMean(x, weight)
		// weight should be the same length as x
		o.Set("circularMean", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("circularMean: x and weight should be the same length"))
			}
			return rt.ToValue(stat.CircularMean(x, weight))
		})
		// m.correlation(x, y, weight)
		// weight should be the same length as x and y
		o.Set("correlation", func(x, y, weight []float64) js.Value {
			if len(x) != len(y) {
				panic(rt.ToValue("correlation: x and y should be the same length"))
			}
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("correlation: x, y and weight should be the same length"))
			}
			return rt.ToValue(stat.Correlation(x, y, weight))
		})
		// m.covariance(x, y, weight)
		o.Set("covariance", func(x, y, weight []float64) js.Value {
			if len(x) != len(y) {
				panic(rt.ToValue("covariance: x and y should be the same length"))
			}
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("covariance: x, y and weight should be the same length"))
			}
			return rt.ToValue(stat.Covariance(x, y, weight))
		})
		// m.entropy(p)
		o.Set("entropy", func(p []float64) float64 {
			return stat.Entropy(p)
		})
		// m.geometricMean(array)
		o.Set("geometricMean", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("geometricMean: x and weight should be the same length"))
			}
			return rt.ToValue(stat.GeometricMean(x, weight))
		})
		// m.mean(x, weight)
		o.Set("mean", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("mean: x and weight should be the same length"))
			}
			return rt.ToValue(stat.Mean(x, weight))
		})
		// m.harmonicMean(x, weight)
		o.Set("harmonicMean", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("harmonicMean: x and weight should be the same length"))
			}
			return rt.ToValue(stat.HarmonicMean(x, weight))
		})
		// m.median(x, weight)
		o.Set("median", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("median: x and weight should be the same length"))
			}
			return rt.ToValue(stat.Quantile(0.5, stat.Empirical, x, weight))
		})
		// m.medianInterp(x, weight)
		o.Set("medianInterp", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("median: x and weight should be the same length"))
			}
			return rt.ToValue(stat.Quantile(0.5, stat.LinInterp, x, weight))
		})
		// m.variance(x, weight)
		o.Set("variance", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("variance: x, y and weight should be the same length"))
			}
			return rt.ToValue(stat.Variance(x, weight))
		})
		// m.meanVariance(x, weight)
		o.Set("meanVariance", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("meanVariance: x and weight should be the same length"))
			}
			m, v := stat.MeanVariance(x, weight)
			return rt.ToValue(map[string]any{"mean": m, "variance": v})
		})
		// m.stdDev(x, weight)
		o.Set("stdDev", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("stdDev: x and weight should be the same length"))
			}
			return rt.ToValue(stat.StdDev(x, weight))
		})
		// m.meanStdDev(x, weight)
		o.Set("meanStdDev", func(x, weight []float64) js.Value {
			if weight != nil && len(x) != len(weight) {
				panic(rt.ToValue("meanStdDev: x and weight should be the same length"))
			}
			m, std := stat.MeanStdDev(x, weight)
			return rt.ToValue(map[string]any{"mean": m, "stdDev": std})
		})
		// m.stdErr(std, sampleSize)
		o.Set("stdErr", func(std, sampleSize float64) float64 {
			return stat.StdErr(std, sampleSize)
		})
		// m.mode(array)
		o.Set("mode", func(arr []float64) js.Value {
			slices.Sort(arr)
			v, c := stat.Mode(arr, nil)
			return rt.ToValue(map[string]any{"value": v, "count": c})
		})
		// m.moment(array)
		o.Set("moment", func(moment float64, arr []float64) float64 {
			return stat.Moment(moment, arr, nil)
		})
		// m.quantile(p, array)
		o.Set("quantile", func(p float64, arr []float64) float64 {
			slices.Sort(arr)
			return stat.Quantile(p, stat.Empirical, arr, nil)
		})
		// m.quantileInterp(p, array)
		o.Set("quantileInterp", func(p float64, arr []float64) float64 {
			slices.Sort(arr)
			return stat.Quantile(p, stat.LinInterp, arr, nil)
		})
		// m.linearRegression(x, y)
		o.Set("linearRegression", func(x, y []float64) js.Value {
			// y = alpha + beta*x
			alpha, beta := stat.LinearRegression(x, y, nil, false)
			return rt.ToValue(map[string]any{"intercept": alpha, "slope": beta})
		})
		// m.fft(times, values)
		o.Set("fft", make_fft(rt))
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
}

func make_fft(rt *js.Runtime) func(times []any, values []any) js.Value {
	return func(times []any, values []any) js.Value {
		ts := make([]time.Time, len(times))
		vs := make([]float64, len(values))
		for i, val := range times {
			switch v := val.(type) {
			case time.Time:
				ts[i] = v
			case *time.Time:
				ts[i] = *v
			case int64:
				ts[i] = time.Unix(0, v)
			case *int64:
				ts[i] = time.Unix(0, *v)
			case float64:
				ts[i] = time.Unix(0, int64(v))
			case *float64:
				ts[i] = time.Unix(0, int64(*v))
			default:
				panic(rt.ToValue(fmt.Sprintf("fft invalid %dth sample time, but %T", i, val)))
			}
		}
		for i, val := range values {
			switch v := val.(type) {
			case float64:
				vs[i] = v
			case *float64:
				vs[i] = *v
			default:
				panic(rt.ToValue(fmt.Sprintf("fft invalid %dth sample value, but %T", i, val)))
			}
		}
		xs, ys := fft.FastFourierTransform(ts, vs)
		ret := rt.NewObject()
		ret.Set("x", rt.ToValue(xs))
		ret.Set("y", rt.ToValue(ys))
		return ret
	}
}

func new_piecewiseConstant(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
		return newInterpolator(rt, &interp.PiecewiseConstant{})
	}
}

func new_piecewiseLinear(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
		return newInterpolator(rt, &interp.PiecewiseLinear{})
	}
}

func new_akimaSpline(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
		return newInterpolator(rt, &interp.AkimaSpline{})
	}
}

func new_fritschButland(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
		return newInterpolator(rt, &interp.FritschButland{})
	}
}

func new_linearRegression(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
		return newInterpolator(rt, &LinearRegression{})
	}
}

func new_clampedCubic(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
		return newInterpolator(rt, &interp.ClampedCubic{})
	}
}

func new_naturalCubic(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
		return newInterpolator(rt, &interp.NaturalCubic{})
	}
}

func new_notAKnotCubic(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
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
	rt     *js.Runtime
	interp interface {
		Fit(xs, ys []float64) error
		Predict(x float64) float64
	}
}

func newInterpolator(rt *js.Runtime, interp interface {
	Fit(xs, ys []float64) error
	Predict(x float64) float64
}) *js.Object {
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

func (ip *Interpolator) Fit(call js.FunctionCall) js.Value {
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
	return js.Undefined()
}

func (ip *Interpolator) Predict(call js.FunctionCall) js.Value {
	if len(call.Arguments) != 1 {
		panic(ip.rt.ToValue("predict: x is required"))
	}
	x := call.Arguments[0].ToFloat()
	return ip.rt.ToValue(ip.interp.Predict(x))
}

func (ip *Interpolator) PredictDerivative(call js.FunctionCall) js.Value {
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
