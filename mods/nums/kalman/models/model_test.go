package models

import (
	"math"
	"testing"
	"time"

	"gonum.org/v1/gonum/mat"
)

func TestSimpleAndBrownianModels(t *testing.T) {
	initialTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	simple := NewSimpleModel(initialTime, 10, SimpleModelConfig{InitialVariance: 2, ProcessVariance: 3, ObservationVariance: 4})
	state := simple.InitialState()
	if !state.Time.Equal(initialTime) || simple.Value(state.State) != 10 || matrixValue(state.Covariance, 0, 0) != 2 {
		t.Fatalf("unexpected simple initial state: %+v", state)
	}
	if matrixValue(simple.Transition(time.Second), 0, 0) != 1 || matrixValue(simple.CovarianceTransition(2*time.Second), 0, 0) != 6 {
		t.Fatal("unexpected simple transition matrices")
	}
	measurement := simple.NewMeasurement(12)
	if measurement.Value.AtVec(0) != 12 || matrixValue(measurement.Covariance, 0, 0) != 4 || matrixValue(measurement.ObservationModel, 0, 0) != 1 {
		t.Fatalf("unexpected simple measurement: %+v", measurement)
	}

	initial := mat.NewVecDense(2, []float64{1, 2})
	brownian := NewBrownianModel(initialTime, initial, BrownianModelConfig{InitialVariance: 5, ProcessVariance: 2, ObservationVariance: 7})
	if brownian.InitialState().State != initial || brownian.Value(initial) != initial {
		t.Fatal("brownian model did not retain state vectors")
	}
	transition := brownian.Transition(time.Second)
	covariance := brownian.CovarianceTransition(1500 * time.Millisecond)
	if matrixValue(transition, 0, 0) != 1 || matrixValue(transition, 1, 1) != 1 || matrixValue(covariance, 0, 0) != 3 || matrixValue(covariance, 1, 1) != 3 {
		t.Fatal("unexpected brownian transition matrices")
	}
	brownianMeasurement := brownian.NewMeasurement(initial)
	if matrixValue(brownianMeasurement.Covariance, 0, 0) != 7 || matrixValue(brownianMeasurement.ObservationModel, 1, 1) != 1 {
		t.Fatal("unexpected brownian measurement matrices")
	}
}

func TestConstantVelocityModel(t *testing.T) {
	initialTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	model := NewConstantVelocityModel(initialTime, mat.NewVecDense(2, []float64{10, 20}), ConstantVelocityModelConfig{InitialVariance: 4, ProcessVariance: 3})
	state := model.InitialState()
	if !state.Time.Equal(initialTime) || !vectorEqual(state.State, []float64{10, 20, 0, 0}) || matrixValue(state.Covariance, 3, 3) != 4 {
		t.Fatalf("unexpected initial state: %+v", state)
	}
	transition := model.Transition(2 * time.Second)
	if matrixValue(transition, 0, 0) != 1 || matrixValue(transition, 0, 2) != 2 || matrixValue(transition, 1, 3) != 2 {
		t.Fatal("unexpected constant velocity transition")
	}
	covariance := model.CovarianceTransition(2 * time.Second)
	if matrixValue(covariance, 0, 0) != 6 || matrixValue(covariance, 3, 3) != 6 {
		t.Fatal("unexpected constant velocity covariance")
	}
	measurement := model.NewPositionMeasurement(mat.NewVecDense(2, []float64{11, 22}), 8)
	if matrixValue(measurement.Covariance, 0, 0) != 8 || matrixValue(measurement.ObservationModel, 0, 0) != 1 || matrixValue(measurement.ObservationModel, 1, 1) != 1 {
		t.Fatal("unexpected position measurement")
	}
	vector := mat.NewVecDense(4, []float64{11, 22, 3, 4})
	if !vectorEqual(model.Position(vector), []float64{11, 22}) || !vectorEqual(model.Velocity(vector), []float64{3, 4}) {
		t.Fatal("position or velocity extraction failed")
	}

	assertPanics(t, func() { model.NewPositionMeasurement(mat.NewVecDense(1, nil), 1) })
	assertPanics(t, func() { model.Position(mat.NewVecDense(1, nil)) })
	assertPanics(t, func() { model.Velocity(mat.NewVecDense(1, nil)) })
}

func matrixValue(matrix mat.Matrix, row, column int) float64 {
	return matrix.At(row, column)
}

func vectorEqual(vector mat.Vector, want []float64) bool {
	if vector.Len() != len(want) {
		return false
	}
	for index, value := range want {
		if math.Abs(vector.AtVec(index)-value) > 1e-9 {
			return false
		}
	}
	return true
}

func assertPanics(t *testing.T, function func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("function did not panic")
		}
	}()
	function()
}
