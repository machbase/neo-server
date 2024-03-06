package models

import (
	"time"

	"gonum.org/v1/gonum/mat"
)

type BrownianModelConfig struct {
	InitialVariance     float64
	ProcessVariance     float64
	ObservationVariance float64
}

type BrownianModel struct {
	initialState State
	dims         int

	transition            *mat.Dense
	observationModel      *mat.Dense
	observationCovariance *mat.Dense

	cfg BrownianModelConfig
}

func NewBrownianModel(initialTime time.Time, initialState mat.Vector, cfg BrownianModelConfig) *BrownianModel {
	dims := initialState.Len()

	transition := mat.NewDense(dims, dims, nil)
	for i := 0; i < dims; i++ {
		transition.Set(i, i, 1.0)
	}

	initialCovariance := mat.NewDense(dims, dims, nil)
	for i := 0; i < dims; i++ {
		initialCovariance.Set(i, i, cfg.InitialVariance)
	}

	observationModel := mat.NewDense(dims, dims, nil)
	for i := 0; i < dims; i++ {
		observationModel.Set(i, i, 1.0)
	}

	observationCovariance := mat.NewDense(dims, dims, nil)
	for i := 0; i < dims; i++ {
		observationCovariance.Set(i, i, cfg.ObservationVariance)
	}

	return &BrownianModel{
		dims: dims,
		initialState: State{
			Time:       initialTime,
			State:      initialState,
			Covariance: initialCovariance,
		},
		transition:            transition,
		observationModel:      observationModel,
		observationCovariance: observationCovariance,
		cfg:                   cfg,
	}
}

func (m *BrownianModel) InitialState() State {
	return m.initialState
}

func (m *BrownianModel) Transition(dt time.Duration) mat.Matrix {
	return m.transition
}

func (m *BrownianModel) CovarianceTransition(dt time.Duration) mat.Matrix {
	result := mat.NewDense(m.dims, m.dims, nil)

	v := m.cfg.ProcessVariance * dt.Seconds()
	for i := 0; i < m.dims; i++ {
		result.Set(i, i, v)
	}

	return result
}

func (s *BrownianModel) NewMeasurement(value mat.Vector) *Measurement {
	return &Measurement{
		Value:            value,
		Covariance:       s.observationCovariance,
		ObservationModel: s.observationModel,
	}
}

func (s *BrownianModel) Value(state mat.Vector) mat.Vector {
	return state
}
