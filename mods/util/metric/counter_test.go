package metric

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCounterJSON(t *testing.T) {
	c := NewCounter()
	c.Add(1.1)
	c.Add(2.2)
	c.Add(3.3)

	data, err := json.Marshal(c)
	require.NoError(t, err)

	expected := `{"samples":3,"value":6.6}`
	require.JSONEq(t, expected, string(data))

	var c2 Counter
	err = json.Unmarshal(data, &c2)
	require.NoError(t, err)

	require.Equal(t, c.samples, c2.samples)
	require.Equal(t, c.value, c2.value)
}

func TestCounterConstructorsAndString(t *testing.T) {
	value := &CounterValue{Samples: 2, Value: 42.5}
	value.SetDerivedValue("copy", &CounterValue{Value: 1})
	counter := NewCounterWithValue(value)
	produced, ok := counter.Produce(false).(*CounterValue)
	require.True(t, ok)
	require.Equal(t, int64(2), produced.Samples)
	require.Equal(t, 42.5, produced.Value)
	require.Contains(t, counter.String(), `"value":42.5`)
	require.Contains(t, value.String(), `"derived"`)
}
