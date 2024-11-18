// Package kalman implements estimation for time series with non-uniform time steps.
// Implementations of the Kalman Filter and Kalman Smoother are provided,
// along with several built-in models for modelling common dynamic systems,
// such as the constant-velocity model and a Brownian model.
package kalman

import (
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/mods/nums/kalman/models"
	"gonum.org/v1/gonum/mat"
)

// KalmanFilter is responsible for prediction and filtering
// of a given linear model. It is assumed that the process being modelled
// is a time series, and that the time steps are non-uniform and specified
// for each update and prediction operation.
type KalmanFilter struct {
	model models.LinearModel

	dims       int
	t          time.Time
	state      *mat.VecDense
	covariance *mat.Dense
}

// NewKalmanFilter returns a new KalmanFilter for the given linear model.
func NewKalmanFilter(model models.LinearModel) *KalmanFilter {
	initial := model.InitialState()

	return &KalmanFilter{
		model:      model,
		dims:       initial.State.Len(),
		t:          initial.Time,
		state:      mat.VecDenseCopyOf(initial.State),
		covariance: mat.DenseCopyOf(initial.Covariance),
	}

}

// State returns the current hidden state of the KalmanFilter.
// Example models provided with this package often provide functions
// to extract meaningful information from the state vector, such as
// .Velocity() for the provided constant velocity model.
func (kf *KalmanFilter) State() mat.Vector {
	return kf.state
}

// Covariance returns the current covaraince of the model.
func (kf *KalmanFilter) Covariance() mat.Matrix {
	return kf.covariance
}

// SetCovariance resets the covariance of the Kalman Filter to the given value.
func (kf *KalmanFilter) SetCovariance(covariance mat.Matrix) {
	kf.covariance = mat.DenseCopyOf(covariance)
}

// SetState resets the state of the Kalman Filter to the given value.
func (kf *KalmanFilter) SetState(state mat.Vector) {
	kf.state = mat.VecDenseCopyOf(state)
}

// Time returns the time for which the current hidden state is an estimate.
// The time is monotone increasing.
func (kf *KalmanFilter) Time() time.Time {
	return kf.t
}

// Predict advances the KalmanFilter from the internal current time
// to the given time using the built-in linear model.
// The state of the filter is updated and the current time is updated.
// Each time can be no earlier than the current time of the filter.
func (kf *KalmanFilter) Predict(t time.Time) error {
	if t.Before(kf.t) {
		return fmt.Errorf("can't predict past: %s", t)
	}

	if t.Equal(kf.t) {
		return nil
	}

	dt := t.Sub(kf.t)
	kf.t = t

	T := kf.model.Transition(dt)
	Q := kf.model.CovarianceTransition(dt)
	P := kf.covariance

	kf.state.MulVec(T, kf.state)

	newCovariance := mat.NewDense(kf.dims, kf.dims, nil)
	newCovariance.Product(T, P, T.T())
	kf.covariance.Add(newCovariance, Q)

	return nil
}

// Update is used to take a new measurement from a sensor and fuse it to the model.
// The time field must be no earlier than the current time of the filter.
func (kf *KalmanFilter) Update(t time.Time, m *models.Measurement) error {
	if t.Before(kf.t) {
		return fmt.Errorf("can't predict past: %s", t)
	}

	if t.After(kf.t) {
		err := kf.Predict(t)
		if err != nil {
			return err
		}
	}

	z := m.Value
	R := m.Covariance
	H := m.ObservationModel
	P := kf.covariance

	preFitResidual := mat.NewVecDense(z.Len(), nil)
	preFitResidual.MulVec(H, kf.state)
	preFitResidual.SubVec(z, preFitResidual)

	preFitResidualCov := mat.NewDense(z.Len(), z.Len(), nil)
	preFitResidualCov.Product(H, P, H.T())
	preFitResidualCov.Add(preFitResidualCov, R)

	preFitResidualCovInv := mat.NewDense(z.Len(), z.Len(), nil)
	preFitResidualCovInv.Inverse(preFitResidualCov)

	gain := mat.NewDense(kf.dims, z.Len(), nil)
	gain.Product(P, H.T(), preFitResidualCovInv)

	newState := mat.NewVecDense(kf.dims, nil)
	newState.MulVec(gain, preFitResidual)
	newState.AddVec(kf.state, newState)

	newCovariance := mat.NewDense(kf.dims, kf.dims, nil)
	newCovariance.Mul(gain, H)
	newCovariance.Sub(eye(kf.dims), newCovariance)
	newCovariance.Mul(newCovariance, P)

	kf.covariance = newCovariance
	kf.state = newState
	kf.t = t

	return nil
}

func eye(n int) *mat.Dense {
	result := mat.NewDense(n, n, nil)
	for i := 0; i < n; i++ {
		result.Set(i, i, 1.0)
	}
	return result
}
