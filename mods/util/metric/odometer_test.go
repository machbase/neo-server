package metric

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOdometerJSON(t *testing.T) {
	om := NewOdometer()
	data, err := json.Marshal(om.Produce(true))
	require.NoError(t, err)

	expected := `{"first":0,"last":0, "empty":true}`
	require.JSONEq(t, expected, string(data))

	om = NewOdometer()
	om.Add(2.0)
	om.Add(7.0)
	om.Add(10.0)

	d, _ := om.Produce(false).(*OdometerValue)
	require.Equal(t, 8.0, d.Diff())

	data, err = json.Marshal(om)
	require.NoError(t, err)

	expected = `{"first":2,"last":10}`
	require.JSONEq(t, expected, string(data))

	var om2 Odometer
	err = json.Unmarshal(data, &om2)
	require.NoError(t, err)

	require.Equal(t, om.first, om2.first)
	require.Equal(t, om.last, om2.last)
	require.Equal(t, om.validFirst, om2.validFirst)
}
