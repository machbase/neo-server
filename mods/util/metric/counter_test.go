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
