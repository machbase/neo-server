package kalman

import (
	"math"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/nums/kalman/models"
	"github.com/stretchr/testify/require"
)

func TestKalman(t *testing.T) {
	var ts time.Time
	values := []float64{1.3, 10.2, 5.0, 3.4}

	model := models.NewSimpleModel(ts, values[0], models.SimpleModelConfig{
		InitialVariance:     1.0,
		ProcessVariance:     1.0,
		ObservationVariance: 2.0,
	})
	filter := NewKalmanFilter(model)

	var result []float64
	for _, v := range values {
		ts = ts.Add(time.Second)
		filter.Update(ts, model.NewMeasurement(v))
		result = append(result, math.Round(model.Value(filter.State())*100000)/100000)
	}
	expect := []float64{1.3, 5.75, 5.375, 4.3875}
	require.EqualValues(t, expect, result)
}

func TestKalmanSmoother(t *testing.T) {
	var ts time.Time
	values := []float64{1.3, 10.2, 5.0, 3.4}

	model := models.NewSimpleModel(ts, values[0], models.SimpleModelConfig{
		InitialVariance:     1.0,
		ProcessVariance:     1.0,
		ObservationVariance: 2.0,
	})
	smoother := NewKalmanSmoother(model)

	var result []float64
	for _, v := range values {
		ts = ts.Add(time.Second)
		states, _ := smoother.Smooth(NewMeasurementAtTime(ts, model.NewMeasurement(v)))
		result = append(result, math.Round(model.Value(states[0].State)*100000)/100000)
	}
	expect := []float64{1.3, 6.64, 3.76667, 2.8}
	require.EqualValues(t, expect, result)
}
