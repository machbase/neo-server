package kalman

import (
	"fmt"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/nums/kalman/models"
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

	for _, v := range values {
		ts = ts.Add(time.Second)
		filter.Update(ts, model.NewMeasurement(v))
		fmt.Printf("filtered value: %f\n", model.Value(filter.State()))
	}
}
