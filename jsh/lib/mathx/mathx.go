package mathx

import (
	_ "embed"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/nums/fft"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

//go:embed mathx.js
var mathx_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"mathx.js": mathx_js,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)
	// m.arrange(begin, end, step) => returns []float64
	o.Set("arrange", func(start, stop, step float64) goja.Value {
		if step == 0 {
			return rt.NewGoError(fmt.Errorf("arrange: step must not be 0"))
		}
		if start == stop {
			return rt.NewGoError(fmt.Errorf("arrange: start and stop must not be equal"))
		}
		if start < stop && step < 0 {
			return rt.NewGoError(fmt.Errorf("arrange: step must be positive"))
		}
		if start > stop && step > 0 {
			return rt.NewGoError(fmt.Errorf("arrange: step must be negative"))
		}
		length := int(math.Abs((stop-start)/step)) + 1
		arr := make([]float64, length)
		for i := 0; i < length; i++ {
			arr[i] = start + float64(i)*step
		}
		return rt.ToValue(arr)
	})
	// m.linspace(begin, end, count) => returns []float64
	o.Set("linspace", func(start, stop float64, count int) goja.Value {
		return rt.ToValue(nums.Linspace(start, stop, count))
	})
	// m.meshgrid(arr1, arr2) => returns [][]float64
	o.Set("meshgrid", func(arr1, arr2 []float64) goja.Value {
		len_x := len(arr1)
		len_y := len(arr2)
		arr := make([][]float64, len_x*len_y)
		for x, v1 := range arr1 {
			for y, v2 := range arr2 {
				arr[x*len_y+y] = []float64{v1, v2}
			}
		}
		return rt.ToValue(arr)
	})
	// arr = m.sort(arr)
	o.Set("sort", func(arr []float64) goja.Value {
		slices.Sort(arr)
		return rt.ToValue(arr)
	})
	// s = m.sum(arr)
	o.Set("sum", func(arr []float64) float64 {
		return floats.Sum(arr)
	})
	// m.cdf(x, weight)
	// x should be sorted, weight should be the same length as x
	o.Set("cdf", func(q float64, x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("cdf: x and weight should be the same length"))
		}
		return rt.ToValue(stat.CDF(q, stat.Empirical, x, weight))
	})
	// m.circularMean(x, weight)
	// weight should be the same length as x
	o.Set("circularMean", func(x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("circularMean: x and weight should be the same length"))
		}
		return rt.ToValue(stat.CircularMean(x, weight))
	})
	// m.correlation(x, y, weight)
	// weight should be the same length as x and y
	o.Set("correlation", func(x, y, weight []float64) goja.Value {
		if len(x) != len(y) {
			panic(rt.ToValue("correlation: x and y should be the same length"))
		}
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("correlation: x, y and weight should be the same length"))
		}
		return rt.ToValue(stat.Correlation(x, y, weight))
	})
	// m.covariance(x, y, weight)
	o.Set("covariance", func(x, y, weight []float64) goja.Value {
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
	o.Set("geometricMean", func(x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("geometricMean: x and weight should be the same length"))
		}
		return rt.ToValue(stat.GeometricMean(x, weight))
	})
	// m.mean(x, weight)
	o.Set("mean", func(x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("mean: x and weight should be the same length"))
		}
		return rt.ToValue(stat.Mean(x, weight))
	})
	// m.harmonicMean(x, weight)
	o.Set("harmonicMean", func(x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("harmonicMean: x and weight should be the same length"))
		}
		return rt.ToValue(stat.HarmonicMean(x, weight))
	})
	// m.median(x, weight)
	o.Set("median", func(x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("median: x and weight should be the same length"))
		}
		return rt.ToValue(stat.Quantile(0.5, stat.Empirical, x, weight))
	})
	// m.medianInterp(x, weight)
	o.Set("medianInterp", func(x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("median: x and weight should be the same length"))
		}
		return rt.ToValue(stat.Quantile(0.5, stat.LinInterp, x, weight))
	})
	// m.variance(x, weight)
	o.Set("variance", func(x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("variance: x, y and weight should be the same length"))
		}
		return rt.ToValue(stat.Variance(x, weight))
	})
	// m.meanVariance(x, weight)
	o.Set("meanVariance", func(x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("meanVariance: x and weight should be the same length"))
		}
		m, v := stat.MeanVariance(x, weight)
		return rt.ToValue(map[string]any{"mean": m, "variance": v})
	})
	// m.stdDev(x, weight)
	o.Set("stdDev", func(x, weight []float64) goja.Value {
		if weight != nil && len(x) != len(weight) {
			panic(rt.ToValue("stdDev: x and weight should be the same length"))
		}
		return rt.ToValue(stat.StdDev(x, weight))
	})
	// m.meanStdDev(x, weight)
	o.Set("meanStdDev", func(x, weight []float64) goja.Value {
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
	o.Set("mode", func(arr []float64) goja.Value {
		v, c := stat.Mode(arr, nil)
		return rt.ToValue(map[string]any{"value": v, "count": c})
	})
	// m.moment(array)
	o.Set("moment", func(moment float64, arr []float64) float64 {
		return stat.Moment(moment, arr, nil)
	})
	// m.quantile(p, array)
	o.Set("quantile", func(p float64, arr []float64) float64 {
		return stat.Quantile(p, stat.Empirical, arr, nil)
	})
	// m.quantileInterp(p, array)
	o.Set("quantileInterp", func(p float64, arr []float64) float64 {
		return stat.Quantile(p, stat.LinInterp, arr, nil)
	})
	// m.linearRegression(x, y)
	o.Set("linearRegression", func(x, y []float64) goja.Value {
		// y = alpha + beta*x
		alpha, beta := stat.LinearRegression(x, y, nil, false)
		return rt.ToValue(map[string]any{"intercept": alpha, "slope": beta})
	})
	// m.fft(times, values)
	o.Set("fft", make_fft(rt))
}

func make_fft(rt *goja.Runtime) func(times []any, values []any) goja.Value {
	return func(times []any, values []any) goja.Value {
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
