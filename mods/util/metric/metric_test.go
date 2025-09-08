package metric

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	timeZone = time.UTC
	m.Run()
}

func TestMetric(t *testing.T) {
	var wg sync.WaitGroup
	var out string
	var cnt int
	var now time.Time
	wg.Add(3)
	c := NewCollector(
		WithSamplingInterval(time.Second),
		WithSeries("1m/1s", time.Second, 60),
	)
	c.AddOutputFunc(func(pd Product) {
		defer wg.Done()
		out = fmt.Sprintf("%s:%s %s %v %s %s",
			pd.Measure, pd.Field, pd.Series, pd.Time.Format(time.TimeOnly), pd.Value.String(), pd.Type)
		if cnt == 0 {
			now = pd.Time
		} else {
			now = now.Add(time.Second)
		}
		cnt++
		expect := fmt.Sprintf(`m1:f1 1m/1s %s {"samples":1,"value":1} counter`, now.Format(time.TimeOnly))
		require.Equal(t, expect, out)
	})
	c.AddInputFunc(func(g Gather) {
		m := Measurement{Name: "m1"}
		m.AddField(Field{Name: "f1", Value: 1.0, Type: CounterType(UnitShort)})
		g.AddMeasurement(m)
	})
	c.Start()
	wg.Wait()

	sn, err := c.Inflight("m1", "f1")
	require.NoError(t, err)
	pd := sn["1m/1s"]
	require.NotNil(t, pd)
	require.Equal(t, "m1", pd.Measure)
	require.Equal(t, "f1", pd.Field)
	require.Equal(t, int64(1), int64(pd.Value.(*CounterValue).Value))
	require.Equal(t, int64(1), int64(pd.Value.(*CounterValue).Samples))
	require.Equal(t, "counter", pd.Type)
	c.Stop()
}
