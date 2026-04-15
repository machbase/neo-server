package metric

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGaugeJSON(t *testing.T) {
	g := NewGauge()
	g.Add(1.0)
	g.Add(2.0)
	g.Add(3.0)

	data, err := json.Marshal(g)
	require.NoError(t, err)

	expected := `{"samples":3,"sum":6,"value":3}`
	require.JSONEq(t, expected, string(data))

	var g2 Gauge
	err = json.Unmarshal(data, &g2)
	require.NoError(t, err)

	require.Equal(t, g.samples, g2.samples)
	require.Equal(t, g.sum, g2.sum)
	require.Equal(t, g.value, g2.value)
}

func TestGaugeConstructorsAndString(t *testing.T) {
	value := &GaugeValue{Samples: 2, Sum: 10, Value: 7}
	value.SetDerivedValue("copy", &GaugeValue{Value: 1})
	gauge := NewGaugeWithValue(value)
	produced, ok := gauge.Produce(false).(*GaugeValue)
	require.True(t, ok)
	require.Equal(t, int64(2), produced.Samples)
	require.Equal(t, 10.0, produced.Sum)
	require.Equal(t, 7.0, produced.Value)
	require.Contains(t, gauge.String(), `"sum":10`)
	require.Contains(t, value.String(), `"derived"`)
}
