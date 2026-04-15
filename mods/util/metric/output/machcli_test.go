package output

import (
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/stretchr/testify/require"
)

type unsupportedValue struct{}

func (unsupportedValue) String() string {
	return "unsupported"
}

func TestConvertProduct(t *testing.T) {
	ts := time.Unix(1710000000, 123)
	m := &MachCli{Prefix: "app"}

	tests := []struct {
		name    string
		product metric.Product
		expect  []StatRec
		err     string
	}{
		{
			name: "counter",
			product: metric.Product{Name: "requests", Time: ts, Value: &metric.CounterValue{
				Samples: 3,
				Value:   9,
			}},
			expect: []StatRec{{Name: "app:requests", Time: ts, Val: 9}},
		},
		{
			name: "gauge",
			product: metric.Product{Name: "cpu", Time: ts, Value: &metric.GaugeValue{
				Samples: 1,
				Value:   17.5,
			}},
			expect: []StatRec{{Name: "app:cpu", Time: ts, Val: 17.5}},
		},
		{
			name: "meter",
			product: metric.Product{Name: "latency", Time: ts, Value: &metric.MeterValue{
				Samples: 2,
				Sum:     30,
				Min:     10,
				Max:     20,
			}},
			expect: []StatRec{
				{Name: "app:latency:avg", Time: ts, Val: 15},
				{Name: "app:latency:max", Time: ts, Val: 20},
				{Name: "app:latency:min", Time: ts, Val: 10},
			},
		},
		{
			name: "timer",
			product: metric.Product{Name: "elapsed", Time: ts, Value: &metric.TimerValue{
				Samples: 2,
				Sum:     5 * time.Second,
				Min:     2 * time.Second,
				Max:     3 * time.Second,
			}},
			expect: []StatRec{
				{Name: "app:elapsed:avg", Time: ts, Val: float64(int64(5 * time.Second / 2))},
				{Name: "app:elapsed:max", Time: ts, Val: float64(3 * time.Second)},
				{Name: "app:elapsed:min", Time: ts, Val: float64(2 * time.Second)},
			},
		},
		{
			name: "histogram",
			product: metric.Product{Name: "size", Time: ts, Value: &metric.HistogramValue{
				Samples: 2,
				P:       []float64{0.5, 0.99},
				Values:  []float64{10, 99},
			}},
			expect: []StatRec{
				{Name: "app:size:p50", Time: ts, Val: 10},
				{Name: "app:size:p99", Time: ts, Val: 99},
			},
		},
		{
			name: "odometer",
			product: metric.Product{Name: "bytes", Time: ts, Value: &metric.OdometerValue{
				Samples: 2,
				First:   3,
				Last:    8,
			}},
			expect: []StatRec{{Name: "app:bytes", Time: ts, Val: 5}},
		},
		{
			name:    "unknown type",
			product: metric.Product{Name: "bad", Time: ts, Value: unsupportedValue{}},
			err:     "metrics unknown type: output.unsupportedValue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs, err := m.convertProduct(tc.product)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
				require.Nil(t, recs)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expect, recs)
		})
	}
}

func TestConvertProductSkipsZeroSamples(t *testing.T) {
	m := &MachCli{}
	ts := time.Unix(1710000000, 0)

	for _, value := range []metric.Value{
		&metric.CounterValue{},
		&metric.GaugeValue{},
		&metric.MeterValue{},
		&metric.TimerValue{},
		&metric.HistogramValue{},
		&metric.OdometerValue{},
	} {
		recs, err := m.convertProduct(metric.Product{Name: "zero", Time: ts, Value: value})
		require.NoError(t, err)
		require.Nil(t, recs)
	}
}

func TestProcessReturnsConnectionError(t *testing.T) {
	m := &MachCli{
		Host: "127.0.0.1",
		Port: 1,
		User: "sys",
		Pass: "manager",
	}

	err := m.Process(metric.Product{
		Name: "requests",
		Time: time.Unix(1710000000, 0),
		Value: &metric.CounterValue{
			Samples: 1,
			Value:   42,
		},
	})
	require.Error(t, err)
}
