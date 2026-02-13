package filter

import (
	"math"
	"time"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/nums/kalman"
	"github.com/machbase/neo-server/v8/mods/nums/kalman/models"
	"gonum.org/v1/gonum/mat"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/filter")
	o := module.Get("exports").(*goja.Object)

	// avg = new Avg();
	// newValue = avg.eval(value);
	o.Set("avg", new_avg(rt))

	// movAvg = new MovAvg(windowSize);
	// newValue = movAvg.eval(value);
	//
	// windowsSize should be larger than 1
	o.Set("movavg", new_movavg(rt))

	// lowpass = new Lowpass(alpha);
	// newValue = lowpass.eval(value);
	//
	// alpha should be 0 < alpha < 1
	o.Set("lowpass", new_lowpass(rt))

	// kalman = new m.Kalman(initialVariance, processVariance, ObservationVariance);
	// or
	// kalman = new m.Kalman({initialVariance: 1.0, processVariance: 1.0, observationVariance: 2.0});
	// newValue = kalman.eval(time, ...vector);
	o.Set("kalman", new_kalman(rt))

	// smoother = new m.KalmanSmoother(initialVariance, processVariance, ObservationVariance);
	// or
	// smoother = new m.KalmanSmoother({initialVariance: 1.0, processVariance: 1.0, observationVariance: 2.0});
	// newValue = smoother.eval(time, ...vector);
	o.Set("kalmanSmoother", new_kalman_smoother(rt))
}

func new_avg(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		ret := rt.NewObject()
		count := 0
		sum := 0.0
		ret.Set("eval", func(call goja.FunctionCall) goja.Value {
			var value float64
			if len(call.Arguments) == 0 {
				panic(rt.ToValue("avg: no argument"))
			}
			if err := rt.ExportTo(call.Arguments[0], &value); err != nil {
				panic(rt.ToValue("avg: invalid argument"))
			}
			if math.IsNaN(value) {
				return rt.ToValue(math.NaN())
			}
			count++
			sum += value
			return rt.ToValue(sum / float64(count))
		})
		return ret
	}
}

func new_movavg(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		windowSize := 0
		if len(call.Arguments) == 0 {
			panic(rt.ToValue("movavg: no argument"))
		}
		if err := rt.ExportTo(call.Arguments[0], &windowSize); err != nil {
			panic(rt.ToValue("movavg: invalid argument"))
		}
		if windowSize <= 1 {
			panic(rt.ToValue("movavg: windowSize should be larger than 1"))
		}
		ret := rt.NewObject()
		count := 0
		sum := 0.0
		window := make([]float64, windowSize)
		ret.Set("eval", func(call goja.FunctionCall) goja.Value {
			var value float64
			if len(call.Arguments) == 0 {
				panic(rt.ToValue("movavg: no argument"))
			}
			if err := rt.ExportTo(call.Arguments[0], &value); err != nil {
				panic(rt.ToValue("movavg: invalid argument"))
			}
			if math.IsNaN(value) {
				return rt.ToValue(math.NaN())
			}
			count++
			sum += value
			if count > windowSize {
				sum -= window[count%windowSize]
				window[count%windowSize] = value
				return rt.ToValue(sum / float64(windowSize))
			} else {
				window[count%windowSize] = value
				return rt.ToValue(sum / float64(count))
			}
		})
		return ret
	}
}

func new_lowpass(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		alpha := 0.0
		if len(call.Arguments) == 0 {
			panic(rt.ToValue("lowpass: no argument"))
		}
		if err := rt.ExportTo(call.Arguments[0], &alpha); err != nil {
			panic(rt.ToValue("lowpass: invalid argument"))
		}
		if alpha <= 0 || alpha >= 1 {
			panic(rt.ToValue("lowpass: alpha should be 0 < alpha < 1"))
		}
		ret := rt.NewObject()
		prev := float64(math.MaxInt64)
		ret.Set("eval", func(call goja.FunctionCall) goja.Value {
			var value float64
			if len(call.Arguments) == 0 {
				panic(rt.ToValue("lowpass: no argument"))
			}
			if err := rt.ExportTo(call.Arguments[0], &value); err != nil {
				panic(rt.ToValue("lowpass: invalid argument"))
			}
			if math.IsNaN(value) {
				return rt.ToValue(math.NaN())
			}
			if prev == math.MaxInt64 {
				prev = value
			} else {
				prev = (1-alpha)*prev + alpha*value
			}
			return rt.ToValue(prev)
		})
		return ret
	}
}

func new_kalman(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		ret := rt.NewObject()
		var kf *kalman.KalmanFilter
		var model *models.BrownianModel
		var iv, pv, ov float64
		if len(call.Arguments) == 1 {
			opt := struct {
				InitialVariance     float64 `json:"initialVariance"`
				ProcessVariance     float64 `json:"processVariance"`
				ObservationVariance float64 `json:"observationVariance"`
			}{}
			if err := rt.ExportTo(call.Arguments[0], &opt); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
			iv = opt.InitialVariance
			pv = opt.ProcessVariance
			ov = opt.ObservationVariance
		} else if len(call.Arguments) == 3 {
			if err := rt.ExportTo(call.Arguments[0], &iv); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
			if err := rt.ExportTo(call.Arguments[1], &pv); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
			if err := rt.ExportTo(call.Arguments[2], &ov); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
		} else {
			panic(rt.ToValue("kalman: invalid arguments"))
		}
		ret.Set("eval", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 2 {
				panic(rt.ToValue("kalman: invalid arguments"))
			}
			var ts time.Time
			var vec []float64

			if err := rt.ExportTo(call.Arguments[0], &ts); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
			for i := 1; i < len(call.Arguments); i++ {
				var value float64
				if err := rt.ExportTo(call.Arguments[i], &value); err != nil {
					panic(rt.ToValue("kalman: invalid argument"))
				}
				if math.IsNaN(value) {
					return rt.ToValue(math.NaN())
				}
				vec = append(vec, value)
			}

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
				if len(vec) == 1 {
					return rt.ToValue(vec[0])
				}
				return rt.ToValue(vec)
			} else {
				kf.Update(ts, model.NewMeasurement(mat.NewVecDense(len(vec), vec)))
				newVal := model.Value(kf.State())

				if dim := newVal.Len(); dim == 1 {
					return rt.ToValue(newVal.AtVec(0))
				} else {
					ret := make([]float64, newVal.Len())
					for i := range ret {
						ret[i] = newVal.AtVec(i)
					}
					return rt.ToValue(ret)
				}
			}
		})
		return ret
	}
}

func new_kalman_smoother(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		ret := rt.NewObject()
		var smoother *kalman.KalmanSmoother
		var model *models.BrownianModel
		var iv, pv, ov float64
		if len(call.Arguments) == 1 {
			opt := struct {
				InitialVariance     float64 `json:"initialVariance"`
				ProcessVariance     float64 `json:"processVariance"`
				ObservationVariance float64 `json:"observationVariance"`
			}{}
			if err := rt.ExportTo(call.Arguments[0], &opt); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
			iv = opt.InitialVariance
			pv = opt.ProcessVariance
			ov = opt.ObservationVariance
		} else if len(call.Arguments) == 3 {
			if err := rt.ExportTo(call.Arguments[0], &iv); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
			if err := rt.ExportTo(call.Arguments[1], &pv); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
			if err := rt.ExportTo(call.Arguments[2], &ov); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
		} else {
			panic(rt.ToValue("kalman: invalid arguments"))
		}
		ret.Set("eval", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 2 {
				panic(rt.ToValue("kalman: invalid arguments"))
			}
			var ts time.Time
			var vec []float64

			if err := rt.ExportTo(call.Arguments[0], &ts); err != nil {
				panic(rt.ToValue("kalman: invalid argument"))
			}
			for i := 1; i < len(call.Arguments); i++ {
				var value float64
				if err := rt.ExportTo(call.Arguments[i], &value); err != nil {
					panic(rt.ToValue("kalman: invalid argument"))
				}
				if math.IsNaN(value) {
					return rt.ToValue(math.NaN())
				}
				vec = append(vec, value)
			}

			if smoother == nil {
				model = models.NewBrownianModel(
					ts,
					mat.NewVecDense(len(vec), vec),
					models.BrownianModelConfig{
						InitialVariance:     iv,
						ProcessVariance:     pv,
						ObservationVariance: ov,
					},
				)
				smoother = kalman.NewKalmanSmoother(model)
				if len(vec) == 1 {
					return rt.ToValue(vec[0])
				}
				return rt.ToValue(vec)
			} else {
				states, err := smoother.Smooth(kalman.NewMeasurementAtTime(ts, model.NewMeasurement(mat.NewVecDense(len(vec), vec))))
				if err != nil {
					panic(rt.ToValue("kalman: " + err.Error()))
				}
				newVal := model.Value(states[0].State)

				if dim := newVal.Len(); dim == 1 {
					return rt.ToValue(newVal.AtVec(0))
				} else {
					ret := make([]float64, newVal.Len())
					for i := range ret {
						ret[i] = newVal.AtVec(i)
					}
					return rt.ToValue(ret)
				}
			}
		})
		return ret
	}
}
