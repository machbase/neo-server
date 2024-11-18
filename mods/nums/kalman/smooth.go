package kalman

import (
	"time"

	"github.com/machbase/neo-server/v8/mods/nums/kalman/models"
	"gonum.org/v1/gonum/mat"
)

type kalmanStateChange struct {
	// The transition used to advance the model from the previous
	// aPosteriori estimate to the current a Priori estimate.
	// x_{k|k-1} = F_k x_{k-1}
	modelTransition mat.Matrix

	// State before measurement taken, x_{k|k-1}, P_{k|k-1}
	aPrioriState      mat.Vector
	aPrioriCovariance mat.Matrix

	// State after measurement taken, x_{k|k}, P_{k|k}
	APoseterioriState     mat.Vector
	aPosterioriCovariance mat.Matrix
}

// KalmanSmoother implements Rauch–Tung–Striebel smoothing.
type KalmanSmoother struct {
	model models.LinearModel
}

// NewKalmanSmoother creates a new smoother for the given model.
func NewKalmanSmoother(model models.LinearModel) *KalmanSmoother {
	return &KalmanSmoother{
		model: model,
	}
}

// MeasurementAtTime represents a measurement taken at a given time.
type MeasurementAtTime struct {
	models.Measurement
	Time time.Time
}

// NewMeasurementAtTime is a helper for initializing measurement at time structs.
func NewMeasurementAtTime(t time.Time, m *models.Measurement) *MeasurementAtTime {
	return &MeasurementAtTime{
		Time:        t,
		Measurement: *m,
	}
}

// Smooth computes optimal estimates of the model states by using all measurements.
// This is done by running a regular Kalman Filter and then performing a backwards pass
// using the Rauch–Tung–Striebel algorithm.
// Better results can be achieved since each state is estimated based on the entire history
// of the process, including the future and past observations.
func (kf *KalmanSmoother) Smooth(measurements ...*MeasurementAtTime) ([]models.State, error) {
	n := len(measurements)
	if n == 0 {
		return make([]models.State, 0), nil
	}

	ss, err := kf.computeForwardsStateChanges(measurements...)
	if err != nil {
		return nil, err
	}

	dims := ss[0].aPrioriState.Len()
	C := mat.NewDense(dims, dims, nil)
	aPrioriCovarianceInv := mat.NewDense(dims, dims, nil)

	result := make([]models.State, n)
	result[n-1].State = ss[n-1].APoseterioriState
	result[n-1].Covariance = ss[n-1].aPosterioriCovariance

	x := mat.NewVecDense(dims, nil)
	P := mat.NewDense(dims, dims, nil)

	for i := n - 2; i >= 0; i-- {
		err = aPrioriCovarianceInv.Inverse(ss[i+1].aPrioriCovariance)
		if err != nil {
			panic(err)
		}

		C.Product(
			ss[i].aPosterioriCovariance,
			ss[i+1].modelTransition.T(),
			aPrioriCovarianceInv,
		)

		x.SubVec(result[i+1].State, ss[i+1].aPrioriState)
		x.MulVec(C, x)
		x.AddVec(ss[i].APoseterioriState, x)

		P.Sub(result[i+1].Covariance, ss[i+1].aPrioriCovariance)
		P.Product(C, P, C.T())
		P.Add(ss[i].aPosterioriCovariance, P)

		result[i].State = mat.VecDenseCopyOf(x)
		result[i].Covariance = mat.DenseCopyOf(P)
	}

	return result, nil
}

// computeForwardsStateChanges runs the regular KalmanFilter for the given measurements.
func (kf *KalmanSmoother) computeForwardsStateChanges(measurements ...*MeasurementAtTime) ([]kalmanStateChange, error) {
	filter := NewKalmanFilter(kf.model)
	result := make([]kalmanStateChange, len(measurements))

	for i, m := range measurements {
		stateChange := &result[i]
		dt := m.Time.Sub(filter.Time())

		stateChange.modelTransition = mat.DenseCopyOf(kf.model.Transition(dt))
		err := filter.Predict(m.Time)
		if err != nil {
			return nil, err
		}

		stateChange.aPrioriState = mat.VecDenseCopyOf(filter.State())
		stateChange.aPrioriCovariance = mat.DenseCopyOf(filter.Covariance())

		err = filter.Update(m.Time, &m.Measurement)
		if err != nil {
			return nil, err
		}

		stateChange.APoseterioriState = mat.VecDenseCopyOf(filter.State())
		stateChange.aPosterioriCovariance = mat.DenseCopyOf(filter.Covariance())
	}

	return result, nil
}
