package metric

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHistogram(t *testing.T) {
	h := NewHistogram(100)

	for i := 1; i <= 100; i++ {
		h.Add(float64(i))
	}

	require.Equal(t, 50.0, h.Quantile(0.50))
	require.Equal(t, 75.0, h.Quantile(0.75))
	require.Equal(t, 90.0, h.Quantile(0.90))
	require.Equal(t, 99.0, h.Quantile(0.99))
	require.Equal(t, 100.0, h.Quantile(0.999))
}

func TestHistogram50(t *testing.T) {
	h := NewHistogram(50)

	for i := 1; i <= 100; i++ {
		h.Add(float64(i))
	}

	require.Equal(t, 49.5, h.Quantile(0.50))
	require.Equal(t, 75.5, h.Quantile(0.75))
	require.Equal(t, 89.5, h.Quantile(0.90))
	require.Equal(t, 99.5, h.Quantile(0.99))
	require.Equal(t, 99.5, h.Quantile(0.999))
}

func TestHistogramQuantiles(t *testing.T) {
	h := NewHistogram(100)

	for i := 1; i <= 100; i++ {
		h.Add(float64(i))
	}

	require.Equal(t, []float64{75.0, 50.0, 90.0}, h.Quantiles(0.75, 0.50, 0.90))
}

func TestHistogramJSON(t *testing.T) {
	h := NewHistogram(10, 0.5, 0.7, 0.9)
	for i := 1; i <= 100; i++ {
		h.Add(float64(i))
	}

	data, err := json.Marshal(h)
	require.NoError(t, err)
	expected := `{"bins":[{"value":4.500000,"count":8.000000},{"value":12.500000,"count":8.000000},{"value":22.000000,"count":11.000000},{"value":31.000000,"count":7.000000},{"value":40.000000,"count":11.000000},{"value":52.500000,"count":14.000000},{"value":64.500000,"count":10.000000},{"value":74.500000,"count":10.000000},{"value":86.000000,"count":13.000000},{"value":96.500000,"count":8.000000}],"samples":100,"qs":[0.5,0.7,0.9]}`
	require.JSONEq(t, expected, string(data))

	var h2 Histogram
	err = json.Unmarshal(data, &h2)
	require.NoError(t, err)

	require.Equal(t, h.samples, h2.samples)
	require.Equal(t, h.qs, h2.qs)
	require.Equal(t, len(h.bins), len(h2.bins))
	for i := range h.bins {
		require.Equal(t, h.bins[i].value, h2.bins[i].value)
		require.Equal(t, h.bins[i].count, h2.bins[i].count)
	}
}
