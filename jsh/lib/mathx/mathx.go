package mathx

import (
	_ "embed"
	"fmt"
	"slices"
	"time"

	"github.com/dop251/goja"
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

func Module(_ *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)
	// arr = m.sort(arr)
	o.Set("sort", func(arr []float64) []float64 {
		slices.Sort(arr)
		return arr
	})
	// s = m.sum(arr)
	o.Set("sum", func(arr []float64) float64 {
		return floats.Sum(arr)
	})
	// m.cdf(x, weight)
	// x should be sorted, weight should be the same length as x
	o.Set("cdf", func(q float64, x, weight []float64) float64 {
		return stat.CDF(q, stat.Empirical, x, weight)
	})
	// m.circularMean(x, weight)
	// weight should be the same length as x
	o.Set("circularMean", func(x, weight []float64) float64 {
		return stat.CircularMean(x, weight)
	})
	// m.correlation(x, y, weight)
	// weight should be the same length as x and y
	o.Set("correlation", func(x, y, weight []float64) float64 {
		return stat.Correlation(x, y, weight)
	})
	// m.covariance(x, y, weight)
	o.Set("covariance", func(x, y, weight []float64) float64 {
		return stat.Covariance(x, y, weight)
	})
	// m.entropy(p)
	o.Set("entropy", func(p []float64) float64 {
		return stat.Entropy(p)
	})
	// m.geometricMean(array)
	o.Set("geometricMean", func(x, weight []float64) float64 {
		return stat.GeometricMean(x, weight)
	})
	// m.mean(x, weight)
	o.Set("mean", func(x, weight []float64) float64 {
		return stat.Mean(x, weight)
	})
	// m.harmonicMean(x, weight)
	o.Set("harmonicMean", func(x, weight []float64) float64 {
		return stat.HarmonicMean(x, weight)
	})
	// m.median(x, weight)
	o.Set("median", func(x, weight []float64) float64 {
		return stat.Quantile(0.5, stat.Empirical, x, weight)
	})
	// m.medianInterp(x, weight)
	o.Set("medianInterp", func(x, weight []float64) float64 {
		return stat.Quantile(0.5, stat.LinInterp, x, weight)
	})
	// m.variance(x, weight)
	o.Set("variance", func(x, weight []float64) float64 {
		return stat.Variance(x, weight)
	})
	// m.meanVariance(x, weight)
	o.Set("meanVariance", func(x, weight []float64) map[string]float64 {
		m, v := stat.MeanVariance(x, weight)
		return map[string]float64{"mean": m, "variance": v}
	})
	// m.stdDev(x, weight)
	o.Set("stdDev", func(x, weight []float64) float64 {
		return stat.StdDev(x, weight)
	})
	// m.meanStdDev(x, weight)
	o.Set("meanStdDev", func(x, weight []float64) map[string]float64 {
		m, std := stat.MeanStdDev(x, weight)
		return map[string]float64{"mean": m, "stdDev": std}
	})
	// m.stdErr(std, sampleSize)
	o.Set("stdErr", func(std, sampleSize float64) float64 {
		return stat.StdErr(std, sampleSize)
	})
	// m.mode(array)
	o.Set("mode", func(arr []float64) map[string]float64 {
		v, c := stat.Mode(arr, nil)
		return map[string]float64{"value": v, "count": c}
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
	o.Set("linearRegression", func(x, y []float64) map[string]float64 {
		// y = alpha + beta*x
		alpha, beta := stat.LinearRegression(x, y, nil, false)
		return map[string]float64{"intercept": alpha, "slope": beta}
	})
	// m.fft(times, values)
	o.Set("fft", make_fft)
}

func make_fft(times []any, values []any) (map[string][]float64, error) {
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
			return nil, fmt.Errorf("fft invalid %dth sample time, but %T", i, val)
		}
	}
	for i, val := range values {
		switch v := val.(type) {
		case float64:
			vs[i] = v
		case *float64:
			vs[i] = *v
		default:
			return nil, fmt.Errorf("fft invalid %dth sample value, but %T", i, val)
		}
	}
	xs, ys := fft.FastFourierTransform(ts, vs)
	return map[string][]float64{"x": xs, "y": ys}, nil
}
