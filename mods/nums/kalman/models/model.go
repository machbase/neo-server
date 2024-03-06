package models

import (
	"time"

	"gonum.org/v1/gonum/mat"
)

type State struct {
	Time       time.Time
	State      mat.Vector
	Covariance mat.Matrix
}

type Measurement struct {
	Covariance       mat.Matrix
	Value            mat.Vector
	ObservationModel mat.Matrix
}

// LinearModel is used to initialize hidden states in the model and
// provide transition matrices to the filter.
// kalman/models provides commonly used models.
type LinearModel interface {
	InitialState() State
	Transition(dt time.Duration) mat.Matrix
	CovarianceTransition(dt time.Duration) mat.Matrix
}
