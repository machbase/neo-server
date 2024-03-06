package models

import (
	"time"

	"gonum.org/v1/gonum/mat"
)

type SimpleModelConfig struct {
	InitialVariance     float64
	ProcessVariance     float64
	ObservationVariance float64
}

// SimpleModel provides the most basic Kalman Filter example of modelling a Brownian time series
// in a single dimension. This is just a wrapper around the BrownianModel, with a simplified interface
// that operates directly on floating point values rather than on vectors.
type SimpleModel struct {
	model *BrownianModel
}

func NewSimpleModel(initialTime time.Time, initialValue float64, cfg SimpleModelConfig) *SimpleModel {
	return &SimpleModel{
		model: NewBrownianModel(
			initialTime,
			mat.NewVecDense(1, []float64{initialValue}),
			BrownianModelConfig{
				InitialVariance:     cfg.InitialVariance,
				ProcessVariance:     cfg.ProcessVariance,
				ObservationVariance: cfg.ObservationVariance,
			},
		),
	}
}

func (s *SimpleModel) InitialState() State {
	return s.model.initialState
}

func (s *SimpleModel) Transition(dt time.Duration) mat.Matrix {
	return s.model.Transition(dt)
}

func (s *SimpleModel) CovarianceTransition(dt time.Duration) mat.Matrix {
	return s.model.CovarianceTransition(dt)
}

func (s *SimpleModel) NewMeasurement(value float64) *Measurement {
	return s.model.NewMeasurement(mat.NewVecDense(1, []float64{value}))
}

// Value is a helper to extract the current value from the Kalman hidden state
func (*SimpleModel) Value(state mat.Vector) float64 {
	return state.AtVec(0)
}
