package tql

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/nums/fft"
	"github.com/machbase/neo-server/v8/mods/nums/kalman"
	"github.com/machbase/neo-server/v8/mods/nums/kalman/models"
	"github.com/machbase/neo-server/v8/mods/nums/opensimplex"
	"github.com/paulmach/orb/geojson"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/interp"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

func enableModuleRegistry(ctx *JSContext) {
	registry := require.NewRegistry(require.WithLoader(jsSourceLoad))
	registry.RegisterNativeModule("system", ctx.nativeModuleSystem)
	registry.RegisterNativeModule("generator", ctx.nativeModuleGenerator)
	registry.RegisterNativeModule("filter", ctx.nativeModuleFilter)
	registry.RegisterNativeModule("analysis", ctx.nativeModuleAnalysis)
	registry.RegisterNativeModule("spatial", ctx.nativeModuleSpatial)
	registry.RegisterNativeModule("opcua", ctx.nativeModuleOpcua)
	registry.Enable(ctx.vm)
}

func (ctx *JSContext) nativeModuleSystem(r *js.Runtime, module *js.Object) {
	// m = require("system")
	o := module.Get("exports").(*js.Object)
	// m.free_os_memory()
	o.Set("free_os_memory", func() {
		debug.FreeOSMemory()
	})
	// m.gc()
	o.Set("gc", func() {
		runtime.GC()
	})
	// m.now()
	o.Set("now", func() js.Value {
		return ctx.vm.ToValue(time.Now())
	})
	// m.parseTime(value)
	o.Set("parseTime", func(value js.Value) js.Value {
		if t, ok := value.Export().(time.Time); ok {
			return ctx.vm.ToValue(t)
		}
		if t, ok := value.Export().(string); ok {
			if t, err := time.Parse(time.RFC3339, t); err == nil {
				return ctx.vm.ToValue(t)
			}
			if t, err := time.Parse(time.RFC3339Nano, t); err == nil {
				return ctx.vm.ToValue(t)
			}
		}
		if t, ok := value.Export().(int64); ok {
			return ctx.vm.ToValue(time.Unix(0, t*int64(time.Millisecond)))
		}
		if t, ok := value.Export().(float64); ok {
			return ctx.vm.ToValue(time.Unix(0, int64(t*float64(time.Millisecond))))
		}
		return ctx.vm.NewGoError(fmt.Errorf("toTime: invalid time value %q", value.ExportType()))
	})
	// m.inflight()
	o.Set("inflight", func() js.Value {
		ret := ctx.vm.NewObject()
		ret.Set("set", func(name string, value js.Value) js.Value {
			if inf := ctx.node.Inflight(); inf != nil {
				inf.SetVariable(name, value.Export())
			}
			return js.Undefined()
		})
		ret.Set("get", func(name string) js.Value {
			if inf := ctx.node.Inflight(); inf != nil {
				if v, err := inf.GetVariable("$" + name); err != nil {
					return ctx.vm.NewGoError(fmt.Errorf("SCRIPT %s", err.Error()))
				} else {
					return ctx.vm.ToValue(v)
				}
			}
			return js.Undefined()
		})
		return ret
	})
	// m.publisher({bridge:"name"})
	o.Set("publisher", func(optObj map[string]any) js.Value {
		var cname string
		if len(optObj) > 0 {
			// parse db options `$.publisher({bridge: "name"})`
			if br, ok := optObj["bridge"]; ok {
				cname = br.(string)
			}
		}
		br, err := bridge.GetBridge(cname)
		if err != nil || br == nil {
			return ctx.vm.NewGoError(fmt.Errorf("publisher: bridge '%s' not found", cname))
		}

		ret := ctx.vm.NewObject()
		if mqttC, ok := br.(*bridge.MqttBridge); ok {
			ret.Set("publish", func(topic string, payload any) js.Value {
				flag, err := mqttC.Publish(topic, payload)
				if err != nil {
					return ctx.vm.NewGoError(fmt.Errorf("publisher: %s", err.Error()))
				}
				return ctx.vm.ToValue(flag)
			})
		} else if natsC, ok := br.(*bridge.NatsBridge); ok {
			ret.Set("publish", func(subject string, payload any) js.Value {
				flag, err := natsC.Publish(subject, payload)
				if err != nil {
					return ctx.vm.NewGoError(fmt.Errorf("publisher: %s", err.Error()))
				}
				return ctx.vm.ToValue(flag)
			})
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("publisher: bridge '%s' not supported", cname))
		}
		return ret
	})
	// m.statz("1m", ...keys)
	o.Set("statz", func(samplingInterval string, keyFilters ...string) js.Value {
		var interval = api.MetricShortTerm
		switch strings.ToLower(samplingInterval) {
		case "short":
			interval = api.MetricShortTerm
		case "mid":
			interval = api.MetricMidTerm
		case "long":
			interval = api.MetricLongTerm
		default:
			if dur, err := time.ParseDuration(samplingInterval); err == nil {
				interval = dur
			}
		}
		statz := api.QueryStatz(interval, api.QueryStatzFilter(keyFilters))
		if statz.Err != nil {
			return ctx.vm.NewGoError(statz.Err)
		}
		ret := []map[string]any{}
		for _, row := range statz.Rows {
			m := map[string]any{
				"time":   row.Timestamp,
				"values": row.Values,
			}
			for i, v := range row.Values {
				m[strings.ReplaceAll(keyFilters[i], ":", "_")] = v
			}
			ret = append(ret, m)
			//ret = append(ret, append([]any{row.Timestamp}, row.Values...))
		}
		return ctx.vm.ToValue(ret)
	})
}

func (ctx *JSContext) nativeModuleGenerator(r *js.Runtime, module *js.Object) {
	// m = require("generator")
	o := module.Get("exports").(*js.Object)
	// m.random()
	o.Set("random", func() js.Value {
		return ctx.vm.ToValue(rand.Float64())
	})
	// m.simplex(seed)
	o.Set("simplex", func(seed int64) js.Value {
		var gen *opensimplex.Generator
		ret := ctx.vm.NewObject()
		ret.Set("eval", func(dim ...float64) float64 {
			if gen == nil {
				gen = opensimplex.New(seed)
			}
			return gen.Eval(dim...)
		})
		return ret
	})
	// m.uuid(version)
	o.Set("uuid", func(version int) js.Value {
		if slices.Contains([]int{1, 4, 6, 7}, version) {
			var gen uuid.Generator
			ret := ctx.vm.NewObject()
			ret.Set("eval", func() js.Value {
				if gen == nil {
					gen = uuid.NewGen()
				}
				var uid uuid.UUID
				switch version {
				case 1:
					uid, _ = gen.NewV1()
				case 4:
					uid, _ = gen.NewV4()
				case 6:
					uid, _ = gen.NewV6()
				case 7:
					uid, _ = gen.NewV7()
				}
				return ctx.vm.ToValue(uid.String())
			})
			return ret
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("uuid: unsupported version %d", version))
		}
	})
	// m.arrange(begin, end, step)
	o.Set("arrange", func(start, stop, step float64) js.Value {
		if step == 0 {
			return ctx.vm.NewGoError(fmt.Errorf("arrange: step must not be 0"))
		}
		if start == stop {
			return ctx.vm.NewGoError(fmt.Errorf("arrange: start and stop must not be equal"))
		}
		if start < stop && step < 0 {
			return ctx.vm.NewGoError(fmt.Errorf("arrange: step must be positive"))
		}
		if start > stop && step > 0 {
			return ctx.vm.NewGoError(fmt.Errorf("arrange: step must be negative"))
		}
		length := int(math.Abs((stop-start)/step)) + 1
		arr := make([]float64, length)
		for i := 0; i < length; i++ {
			arr[i] = start + float64(i)*step
		}
		return ctx.vm.ToValue(arr)
	})
	// m.linspace(begin, end, count)
	o.Set("linspace", func(start, stop float64, count int) js.Value {
		return ctx.vm.ToValue(nums.Linspace(start, stop, count))
	})
	// m.meshgrid(arr1, arr2)
	o.Set("meshgrid", func(arr1, arr2 []float64) js.Value {
		len_x := len(arr1)
		len_y := len(arr2)
		arr := make([][]float64, len_x*len_y)
		for x, v1 := range arr1 {
			for y, v2 := range arr2 {
				arr[x*len_y+y] = []float64{v1, v2}
			}
		}
		return ctx.vm.ToValue(arr)
	})
}

func (ctx *JSContext) nativeModuleFilter(r *js.Runtime, module *js.Object) {
	// m = require("filter")
	o := module.Get("exports").(*js.Object)
	// avg = m.avg();
	// newValue = avg.eval(value);
	o.Set("avg", func() js.Value {
		ret := ctx.vm.NewObject()
		count := 0
		sum := 0.0
		ret.Set("eval", func(value float64) float64 {
			count++
			sum += value
			return sum / float64(count)
		})
		return ret
	})
	// movAvg = m.movavg(windowSize);
	// newValue = movAvg.eval(value);
	o.Set("movavg", func(windowSize int) js.Value {
		if windowSize <= 1 {
			return ctx.vm.NewGoError(errors.New("windowSize should be > 1"))
		}
		ret := ctx.vm.NewObject()
		count := 0
		sum := 0.0
		window := make([]float64, windowSize)
		ret.Set("eval", func(value float64) float64 {
			count++
			sum += value
			if count > windowSize {
				sum -= window[count%windowSize]
				window[count%windowSize] = value
				return sum / float64(windowSize)
			} else {
				window[count%windowSize] = value
				return sum / float64(count)
			}
		})
		return ret
	})
	// lpf = m.lowpass(alpha);
	// newValue = lpf.eval(value);
	o.Set("lowpass", func(alpha float64) js.Value {
		if alpha <= 0 || alpha >= 1 {
			return ctx.vm.NewGoError(errors.New("alpha should be 0 < alpha < 1"))
		}
		ret := ctx.vm.NewObject()
		prev := float64(math.MaxInt64)
		ret.Set("eval", func(value float64) float64 {
			if prev == math.MaxInt64 {
				prev = value
			} else {
				prev = (1-alpha)*prev + alpha*value
			}
			return prev
		})
		return ret
	})
	// kalman = m.kalman(initalVariance, processVariance, ObservationVariance);
	// newValue = kalman.eval(time, ...vector);
	o.Set("kalman", func(iv, pv, ov float64) js.Value {
		var kf *kalman.KalmanFilter
		var model *models.BrownianModel
		ret := ctx.vm.NewObject()
		ret.Set("eval", func(ts time.Time, vec ...float64) js.Value {
			if kf == nil {
				model = models.NewBrownianModel(
					ts,
					mat.NewVecDense(len(vec), vec),
					models.BrownianModelConfig{
						InitialVariance:     iv,
						ProcessVariance:     pv,
						ObservationVariance: ov,
					},
				)
				kf = kalman.NewKalmanFilter(model)
			}
			kf.Update(ts, model.NewMeasurement(mat.NewVecDense(len(vec), vec)))

			newVal := model.Value(kf.State())
			if dim := newVal.Len(); dim == 1 {
				return ctx.vm.ToValue(newVal.AtVec(0))
			} else {
				ret := make([]float64, newVal.Len())
				for i := range ret {
					ret[i] = newVal.AtVec(i)
				}
				return ctx.vm.ToValue(ret)
			}
		})
		return ret
	})
}

func (ctx *JSContext) nativeModuleAnalysis(r *js.Runtime, module *js.Object) {
	// m = require("analysis")
	o := module.Get("exports").(*js.Object)
	// arr = m.sort(arr)
	o.Set("sort", func(arr []float64) js.Value {
		slices.Sort(arr)
		return ctx.vm.ToValue(arr)
	})
	// s = m.sum(arr)
	o.Set("sum", func(arr []float64) float64 {
		return floats.Sum(arr)
	})
	// m.cdf(x, weight)
	// x should be sorted, weight should be the same length as x
	o.Set("cdf", func(p float64, x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("cdf: x and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.CDF(p, stat.Empirical, x, weight))
	})
	// m.circularMean(x, weight)
	// weight should be the same length as x
	o.Set("circularMean", func(x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("circularMean: x and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.CircularMean(x, weight))
	})
	// m.correlation(x, y, weight)
	// weight should be the same length as x and y
	o.Set("correlation", func(x, y, weight []float64) js.Value {
		if len(x) != len(y) {
			return ctx.vm.NewGoError(fmt.Errorf("correlation: x and y should be the same length"))
		}
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("correlation: x, y and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.Correlation(x, y, weight))
	})
	// m.covariance(x, y, weight)
	o.Set("covariance", func(x, y, weight []float64) js.Value {
		if len(x) != len(y) {
			return ctx.vm.NewGoError(fmt.Errorf("covariance: x and y should be the same length"))
		}
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("covariance: x, y and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.Covariance(x, y, weight))
	})
	// m.entropy(p)
	o.Set("entropy", func(p []float64) float64 {
		return stat.Entropy(p)
	})
	// m.geometricMean(array)
	o.Set("geometricMean", func(x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("geometricMean: x and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.GeometricMean(x, weight))
	})
	// m.mean(x, weight)
	o.Set("mean", func(x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("mean: x and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.Mean(x, weight))
	})
	// m.harmonicMean(x, weight)
	o.Set("harmonicMean", func(x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("harmonicMean: x and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.HarmonicMean(x, weight))
	})
	// m.median(x, weight)
	o.Set("median", func(x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("median: x and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.Quantile(0.5, stat.Empirical, x, weight))
	})
	// m.variance(x, weight)
	o.Set("variance", func(x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("variance: x, y and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.Variance(x, weight))
	})
	// m.meanVariance(x, weight)
	o.Set("meanVariance", func(x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("meanVariance: x and weight should be the same length"))
		}
		m, v := stat.MeanVariance(x, weight)
		return ctx.vm.ToValue(map[string]any{"mean": m, "variance": v})
	})
	// m.stdDev(x, weight)
	o.Set("stdDev", func(x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("stdDev: x and weight should be the same length"))
		}
		return ctx.vm.ToValue(stat.StdDev(x, weight))
	})
	// m.meanStdDev(x, weight)
	o.Set("meanStdDev", func(x, weight []float64) js.Value {
		if weight != nil && len(x) != len(weight) {
			return ctx.vm.NewGoError(fmt.Errorf("meanStdDev: x and weight should be the same length"))
		}
		m, std := stat.MeanStdDev(x, weight)
		return ctx.vm.ToValue(map[string]any{"mean": m, "stdDev": std})
	})
	// m.stdErr(std, sampleSize)
	o.Set("stdErr", func(std, sampleSize float64) float64 {
		return stat.StdErr(std, sampleSize)
	})
	// m.mode(array)
	o.Set("mode", func(arr []float64) js.Value {
		slices.Sort(arr)
		v, c := stat.Mode(arr, nil)
		return ctx.vm.ToValue(map[string]any{"value": v, "count": c})
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
	// m.linearRegression(x, y)
	o.Set("linearRegression", func(x, y []float64) js.Value {
		// y = alpha + beta*x
		alpha, beta := stat.LinearRegression(x, y, nil, false)
		return ctx.vm.ToValue(map[string]any{"alpha": alpha, "beta": beta})
	})
	// m.fft(times, values)
	o.Set("fft", func(times []any, values []any) js.Value {
		ts := make([]time.Time, len(times))
		vs := make([]float64, len(values))
		for i, val := range times {
			switch v := val.(type) {
			case time.Time:
				ts[i] = v
			case *time.Time:
				ts[i] = *v
			default:
				return ctx.vm.NewGoError(fmt.Errorf("FFTError invalid %dth sample time, but %T", i, val))
			}
		}
		for i, val := range values {
			switch v := val.(type) {
			case float64:
				vs[i] = v
			case *float64:
				vs[i] = *v
			default:
				return ctx.vm.NewGoError(fmt.Errorf("FFTError invalid %dth sample value, but %T", i, val))
			}
		}
		xs, ys := fft.FastFourierTransform(ts, vs)
		return ctx.vm.ToValue(map[string]any{"x": xs, "y": ys})
	})
	// m.interpPiecewiseConstant(x, y)
	o.Set("interpPiecewiseConstant", func(x, y []float64) js.Value {
		var predicator = &interp.PiecewiseConstant{}
		predicator.Fit(x, y)
		ret := ctx.vm.NewObject()
		ret.Set("predict", func(x float64) float64 {
			return predicator.Predict(x)
		})
		return ret
	})
	// m.interpPiecewiseLinear(x, y)
	o.Set("interpPiecewiseLinear", func(x, y []float64) js.Value {
		var predicator = &interp.PiecewiseLinear{}
		predicator.Fit(x, y)
		ret := ctx.vm.NewObject()
		ret.Set("predict", func(x float64) float64 {
			return predicator.Predict(x)
		})
		return ret
	})
	// m.interpAkimaSpline(x, y)
	o.Set("interpAkimaSpline", func(x, y []float64) js.Value {
		var predicator = &interp.AkimaSpline{}
		predicator.Fit(x, y)
		ret := ctx.vm.NewObject()
		ret.Set("predict", func(x float64) float64 {
			return predicator.Predict(x)
		})
		return ret
	})
	// m.interpFritschButland(x, y)
	o.Set("interpFritschButland", func(x, y []float64) js.Value {
		var predicator = &interp.FritschButland{}
		predicator.Fit(x, y)
		ret := ctx.vm.NewObject()
		ret.Set("predict", func(x float64) float64 {
			return predicator.Predict(x)
		})
		return ret
	})
	// m.interpLinearregression(x, y)
	o.Set("interpLinearregression", func(x, y []float64) js.Value {
		a, b := stat.LinearRegression(x, y, nil, false)
		if b != b {
			return ctx.vm.NewGoError(fmt.Errorf("predictLinearregression: invalid regression"))
		}
		ret := ctx.vm.NewObject()
		ret.Set("predict", func(x float64) float64 {
			return a + b*x
		})
		return ret
	})
}

func (ctx *JSContext) nativeModuleSpatial(r *js.Runtime, module *js.Object) {
	// m = require("spatial")
	o := module.Get("exports").(*js.Object)
	// m.haversine(lat1, lon1, lat2, lon2)
	// m.haversine([lat1, lon1], [lat2, lon2])
	// m.haversine({radius: 1000, coordinates: [[lat1, lon1], [lat2, lon2]]})
	o.Set("haversine", ctx.saptial_haversine)
	// m.parseGeoJSON(value)
	o.Set("parseGeoJSON", ctx.spatial_parseGeoJSON)
	// m.simplify(tolerance, [lat1, lon1], [lat2, lon2], ...)
	o.Set("simplify", ctx.spatial_simplify)
}

func (ctx *JSContext) saptial_haversine(call js.FunctionCall) js.Value {
	// EarthRadius is the radius of the earth in meters.
	// To keep things consistent, this value matches WGS84 Web Mercator (EPSG:3867).
	EarthRadius := 6378137.0 // meters
	degreesToRadians := func(d float64) float64 { return d * math.Pi / 180 }
	var lat1, lon1, lat2, lon2, diffLat, diffLon, a, c float64
	var err error
	if len(call.Arguments) == 1 {
		arg := struct {
			Radius float64      `json:"radius"`
			Coord  [][2]float64 `json:"coordinates"`
		}{}
		if err = ctx.vm.ExportTo(call.Arguments[0], &arg); err != nil {
			goto invalid_arguments
		}
		if len(arg.Coord) != 2 {
			goto invalid_arguments
		}
		if arg.Radius > 0 {
			EarthRadius = arg.Radius
		}
		lat1, lon1 = arg.Coord[0][0], arg.Coord[0][1]
		lat2, lon2 = arg.Coord[1][0], arg.Coord[1][1]
	} else if len(call.Arguments) == 2 {
		var loc1, loc2 []float64
		if err = ctx.vm.ExportTo(call.Arguments[0], &loc1); err != nil {
			goto invalid_arguments
		}
		if err = ctx.vm.ExportTo(call.Arguments[1], &loc2); err != nil {
			goto invalid_arguments
		}
		lat1, lon1 = loc1[0], loc1[1]
		lat2, lon2 = loc2[0], loc2[1]
	} else if len(call.Arguments) == 4 {
		if err = ctx.vm.ExportTo(call.Arguments[0], &lat1); err != nil {
			goto invalid_arguments
		}
		if err = ctx.vm.ExportTo(call.Arguments[1], &lon1); err != nil {
			goto invalid_arguments
		}
		if err = ctx.vm.ExportTo(call.Arguments[2], &lat2); err != nil {
			goto invalid_arguments
		}
		if err = ctx.vm.ExportTo(call.Arguments[3], &lon2); err != nil {
			goto invalid_arguments
		}
	}
	diffLat = degreesToRadians(lat2) - degreesToRadians(lat1)
	diffLon = degreesToRadians(lon2) - degreesToRadians(lon1)
	a = math.Pow(math.Sin(diffLat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(diffLon/2), 2)
	c = 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return ctx.vm.ToValue(c * EarthRadius)
invalid_arguments:
	if err != nil {
		return ctx.vm.NewGoError(fmt.Errorf("haversine: invalid arguments %s", err.Error()))
	}
	return ctx.vm.NewGoError(fmt.Errorf("haversine: invalid arguments %v", call.Arguments))
}

// Ram-Douglas-Peucker simplify
func (ctx *JSContext) spatial_simplify(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 3 {
		return ctx.vm.NewGoError(fmt.Errorf("simplify: invalid arguments %v", call.Arguments))
	}
	var tolerance float64
	if err := ctx.vm.ExportTo(call.Arguments[0], &tolerance); err != nil {
		return ctx.vm.NewGoError(fmt.Errorf("simplify: invalid arguments %s", err.Error()))
	}
	points := make([]nums.Point, len(call.Arguments)-1)
	for i := 1; i < len(call.Arguments); i++ {
		var pt [2]float64
		if err := ctx.vm.ExportTo(call.Arguments[i], &pt); err != nil {
			return ctx.vm.NewGoError(fmt.Errorf("simplify: invalid arguments %s", err.Error()))
		}
		// nums.Point is a [Lng, Lat] 2D point of
		points[i-1] = nums.Point([2]float64{pt[1], pt[0]})
	}
	simplified := nums.SimplifyPath(points, tolerance)
	ret := make([][]float64, len(simplified))
	for i, p := range simplified {
		ret[i] = []float64{p[1], p[0]}
	}
	return ctx.vm.ToValue(ret)
}

func (ctx *JSContext) spatial_parseGeoJSON(value js.Value) js.Value {
	obj := value.ToObject(ctx.vm)
	if obj == nil {
		return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError requires a GeoJSON object, but got %q", value.ExportType()))
	}
	typeString := obj.Get("type")
	if typeString == nil {
		return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError missing a GeoJSON type"))
	}
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
	}
	var geoObj any
	switch typeString.String() {
	case "FeatureCollection":
		if geo, err := geojson.UnmarshalFeatureCollection(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
	case "Feature":
		if geo, err := geojson.UnmarshalFeature(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
	case "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection":
		if geo, err := geojson.UnmarshalGeometry(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
	default:
		return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", "unsupported GeoJSON type"))
	}
	var _ = geoObj
	return obj
}
