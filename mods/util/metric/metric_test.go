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
	seriesID, err := NewSeriesID("1m", "1m/1s", time.Second, 60)
	require.NoError(t, err)
	c := NewCollector(
		WithSamplingInterval(time.Second),
		WithSeries(seriesID),
	)
	c.AddOutputFunc(func(pd Product) error {
		defer wg.Done()
		out = fmt.Sprintf("%s %s %v %s %s",
			pd.Name, pd.SeriesTitle, pd.Time.Format(time.TimeOnly), pd.Value.String(), pd.Type)
		if cnt == 0 {
			now = pd.Time
		} else {
			now = now.Add(time.Second)
		}
		cnt++
		expect := fmt.Sprintf(`m1:f1 1m/1s %s {"samples":1,"value":1} counter`, now.Format(time.TimeOnly))
		require.Equal(t, expect, out)
		return nil
	})
	c.AddInputFunc(func(g *Gather) error {
		g.Add("m1:f1", 1.0, CounterType(UnitShort))
		return nil
	})
	c.Start()
	wg.Wait()

	sn, err := c.Inflight("m1:f1")
	require.NoError(t, err)
	// TODO: how to preserve the lowercase of series ID?
	pd := sn["1M"]
	require.NotNil(t, pd)
	require.Equal(t, "m1:f1", pd.Name)
	require.Equal(t, int64(1), int64(pd.Value.(*CounterValue).Value))
	require.Equal(t, int64(1), int64(pd.Value.(*CounterValue).Samples))
	require.Equal(t, "counter", pd.Type)
	c.Stop()
}
