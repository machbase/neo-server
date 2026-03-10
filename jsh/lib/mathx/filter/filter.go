package filter

import (
	_ "embed"
	"errors"
	"math"
	"time"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/nums/kalman"
	"github.com/machbase/neo-server/v8/mods/nums/kalman/models"
	"gonum.org/v1/gonum/mat"
)

//go:embed filter.js
var filter_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"mathx/filter.js": filter_js,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/mathx/filter")
	o := module.Get("exports").(*goja.Object)

	// avg = new Avg();
	// newValue = avg.eval(value);
	o.Set("Avg", newAvg)

	// movAvg = new MovAvg(windowSize);
	// newValue = movAvg.eval(value);
	//
	// windowsSize should be larger than 1
	o.Set("MovAvg", newMovAvg)

	// lowpass = new Lowpass(alpha);
	// newValue = lowpass.eval(value);
	//
	// alpha should be 0 < alpha < 1
	o.Set("Lowpass", newLowpass)

	// kalman = new Kalman(initialVariance, processVariance, ObservationVariance);
	// newValue = kalman.eval(time, ...vector);
	o.Set("Kalman", newKalman)

	// smoother = new KalmanSmoother(initialVariance, processVariance, ObservationVariance);
	// newValue = smoother.eval(time, ...vector);
	o.Set("KalmanSmoother", newKalmanSmoother)
}

type Avg struct {
	count float64
	sum   float64
}

func newAvg() *Avg {
	return &Avg{}
}

func (a *Avg) Eval(value float64) float64 {
	if math.IsNaN(value) {
		return math.NaN()
	}
	a.count++
	a.sum += value
	return a.sum / a.count
}

type MovAvg struct {
	count      int
	sum        float64
	window     []float64
	windowSize int
}

func newMovAvg(windowSize int) *MovAvg {
	return &MovAvg{
		windowSize: windowSize,
		window:     make([]float64, windowSize),
	}
}

func (m *MovAvg) Eval(value float64) (float64, error) {
	if math.IsNaN(value) {
		return math.NaN(), nil
	}
	m.count++
	m.sum += value
	if m.count > m.windowSize {
		m.sum -= m.window[m.count%m.windowSize]
		m.window[m.count%m.windowSize] = value
		return m.sum / float64(m.windowSize), nil
	} else {
		m.window[m.count%m.windowSize] = value
		return m.sum / float64(m.count), nil
	}
}

type Lowpass struct {
	prev  float64
	alpha float64
}

func newLowpass(alpha float64) *Lowpass {
	return &Lowpass{
		prev:  math.MaxInt64,
		alpha: alpha,
	}
}

func (l *Lowpass) Eval(value float64) (float64, error) {
	if math.IsNaN(value) {
		return math.NaN(), nil
	}
	if l.prev == math.MaxInt64 {
		l.prev = value
	} else {
		l.prev = (1-l.alpha)*l.prev + l.alpha*value
	}
	return l.prev, nil
}

type Kalman struct {
	kf    *kalman.KalmanFilter
	model *models.BrownianModel
	iv    float64
	pv    float64
	ov    float64
}

// InitialVariance, ProcessVariance, ObservationVariance
func newKalman(initVariance, processVariance, observationVariance float64) *Kalman {
	return &Kalman{
		iv: initVariance,
		pv: processVariance,
		ov: observationVariance,
	}
}

func (k *Kalman) Eval(ts time.Time, vec ...float64) []float64 {
	if k.kf == nil {
		k.model = models.NewBrownianModel(
			ts,
			mat.NewVecDense(len(vec), vec),
			models.BrownianModelConfig{
				InitialVariance:     k.iv,
				ProcessVariance:     k.pv,
				ObservationVariance: k.ov,
			},
		)
		k.kf = kalman.NewKalmanFilter(k.model)
		return vec
	} else {
		k.kf.Update(ts, k.model.NewMeasurement(mat.NewVecDense(len(vec), vec)))
		newVal := k.model.Value(k.kf.State())
		ret := make([]float64, newVal.Len())
		for i := range ret {
			ret[i] = newVal.AtVec(i)
		}
		return ret
	}
}

type KalmanSmoother struct {
	smoother *kalman.KalmanSmoother
	model    *models.BrownianModel
	iv       float64
	pv       float64
	ov       float64
}

// InitialVariance, ProcessVariance, ObservationVariance
func newKalmanSmoother(initVariance, processVariance, observationVariance float64) *KalmanSmoother {
	return &KalmanSmoother{
		iv: initVariance,
		pv: processVariance,
		ov: observationVariance,
	}
}

func (k *KalmanSmoother) Eval(ts time.Time, vec ...float64) ([]float64, error) {
	if k.smoother == nil {
		k.model = models.NewBrownianModel(
			ts,
			mat.NewVecDense(len(vec), vec),
			models.BrownianModelConfig{
				InitialVariance:     k.iv,
				ProcessVariance:     k.pv,
				ObservationVariance: k.ov,
			},
		)
		k.smoother = kalman.NewKalmanSmoother(k.model)
		return vec, nil
	} else {
		states, err := k.smoother.Smooth(kalman.NewMeasurementAtTime(ts, k.model.NewMeasurement(mat.NewVecDense(len(vec), vec))))
		if err != nil {
			return nil, errors.New("kalman: " + err.Error())
		}
		newVal := k.model.Value(states[0].State)

		ret := make([]float64, newVal.Len())
		for i := range ret {
			ret[i] = newVal.AtVec(i)
		}
		return ret, nil
	}
}
