package metric

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMeterJSON(t *testing.T) {
	m := NewMeter()
	m.Add(1.0)
	m.Add(2.0)
	m.Add(3.0)

	data, err := json.Marshal(m)
	require.NoError(t, err)

	expected := `{"first":1,"last":3,"min":1,"max":3,"sum":6,"samples":3}`
	require.JSONEq(t, expected, string(data))

	var m2 Meter
	err = json.Unmarshal(data, &m2)
	require.NoError(t, err)

	require.Equal(t, m.first, m2.first)
	require.Equal(t, m.last, m2.last)
	require.Equal(t, m.min, m2.min)
	require.Equal(t, m.max, m2.max)
	require.Equal(t, m.sum, m2.sum)
	require.Equal(t, m.samples, m2.samples)
}
