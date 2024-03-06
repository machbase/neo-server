package models

import (
	"fmt"
	"time"

	"gonum.org/v1/gonum/mat"
)

// ConstantVelocityModelConfig is used to set the variance of the process and
// the variance of the first measurement.
// It is assumed that the covariance of the state is a scaled identity matrix,
// so that the variance of each component of the position and velocity are identical.
// Observation variances in this model are provided on a per-measurement basis.
type ConstantVelocityModelConfig struct {
	InitialVariance float64
	ProcessVariance float64
}

// ConstantVelocityModel models a particle moving over time with state modelled by position
// and velocity.
type ConstantVelocityModel struct {
	initialState State
	dims         int
	stateDims    int
	cfg          ConstantVelocityModelConfig
}

// NewConstantVelocityModel initialises a constant velocity model.
func NewConstantVelocityModel(initialTime time.Time, initialPosition mat.Vector, cfg ConstantVelocityModelConfig) *ConstantVelocityModel {
	dims := initialPosition.Len()
	stateDims := 2 * dims

	initialCovariance := mat.NewDense(stateDims, stateDims, nil)
	for i := 0; i < stateDims; i++ {
		initialCovariance.Set(i, i, cfg.InitialVariance)
	}

	initialState := mat.NewVecDense(stateDims, nil)
	for i := 0; i < dims; i++ {
		initialState.SetVec(i, initialPosition.AtVec(i))
	}

	return &ConstantVelocityModel{
		dims:      dims,
		stateDims: stateDims,
		initialState: State{
			Time:       initialTime,
			State:      initialState,
			Covariance: initialCovariance,
		},
		cfg: cfg,
	}
}

// InitialState initializes the model.
func (m *ConstantVelocityModel) InitialState() State {
	return m.initialState
}

// Transition returns the linear transformation that advances the model for the given time
// step.
func (m *ConstantVelocityModel) Transition(dt time.Duration) mat.Matrix {
	result := mat.NewDense(m.stateDims, m.stateDims, nil)
	for i := 0; i < m.stateDims; i++ {
		result.Set(i, i, 1.0)
	}

	dts := dt.Seconds()
	for i := 0; i < m.dims; i++ {
		result.Set(i, m.dims+i, dts)
	}

	return result
}

// CovarianceTransition returns the covariance of the process noise for the given time step.
// Note: This covariance is very simple, there are better ways to model the process noise
// for constant velocity models.
func (m *ConstantVelocityModel) CovarianceTransition(dt time.Duration) mat.Matrix {
	result := mat.NewDense(m.stateDims, m.stateDims, nil)
	v := dt.Seconds() * m.cfg.ProcessVariance

	for i := 0; i < m.stateDims; i++ {
		result.Set(i, i, v)
	}

	return result
}

// NewPositionMeasurement provides a new measurement for fusing into the model state.
// It is assumed the covariance of the measurement is a scaled identity matrix.
func (m *ConstantVelocityModel) NewPositionMeasurement(position mat.Vector, variance float64) *Measurement {
	if position.Len() != m.dims {
		panic(fmt.Sprintf("position vector has incorrect number of entries: %d (expected %d)", position.Len(), m.dims))
	}

	covariance := mat.NewDense(m.dims, m.dims, nil)
	for i := 0; i < m.dims; i++ {
		covariance.Set(i, i, variance)
	}

	observationModel := mat.NewDense(m.dims, m.stateDims, nil)
	for i := 0; i < m.dims; i++ {
		observationModel.Set(i, i, 1.0)
	}

	return &Measurement{
		Value:            position,
		Covariance:       covariance,
		ObservationModel: observationModel,
	}
}

// Position is a helper to read the position value frmo a state vector for this model.
func (m *ConstantVelocityModel) Position(state mat.Vector) mat.Vector {
	if state.Len() != m.stateDims {
		panic(fmt.Sprintf("state vector has incorrect number of entries: %d (expected %d)", state.Len(), m.stateDims))
	}

	result := mat.NewVecDense(m.dims, nil)
	for i := 0; i < m.dims; i++ {
		result.SetVec(i, state.AtVec(i))
	}

	return result
}

func (m *ConstantVelocityModel) Velocity(state mat.Vector) mat.Vector {
	if state.Len() != m.stateDims {
		panic(fmt.Sprintf("state vector has incorrect number of entries: %d (expected %d)", state.Len(), m.stateDims))
	}

	result := mat.NewVecDense(m.dims, nil)
	for i := 0; i < m.dims; i++ {
		result.SetVec(i, state.AtVec(i+m.dims))
	}

	return result
}
