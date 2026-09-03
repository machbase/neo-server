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
	seriesID, err := NewSeriesID("METRIC_1M", "1m/1s", time.Second, 60)
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
	pd := sn["METRIC_1M"]
	require.NotNil(t, pd)
	require.Equal(t, "m1:f1", pd.Name)
	require.Equal(t, int64(1), int64(pd.Value.(*CounterValue).Value))
	require.Equal(t, int64(1), int64(pd.Value.(*CounterValue).Samples))
	require.Equal(t, "counter", pd.Type)
	c.Stop()
}

func TestCollectorSendNonBlockingWhenBufferFull(t *testing.T) {
	c := NewCollector(WithInputBuffer(1))

	// Fill the channel without starting collector so recvCh remains full.
	c.Send(Measure{Name: "m1", Value: 1, Type: GaugeType(UnitShort)})

	done := make(chan struct{})
	go func() {
		c.Send(Measure{Name: "m2", Value: 2, Type: GaugeType(UnitShort)})
		close(done)
	}()

	select {
	case <-done:
		// pass
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Collector.Send blocked when recvCh was full")
	}

	require.Equal(t, uint64(1), c.DroppedCount())
}

func TestCollectorPublishesInflightProduct(t *testing.T) {
	seriesID, err := NewSeriesID("METRIC_1S", "1s", time.Second, 10)
	require.NoError(t, err)

	var products []Product
	c := NewCollector(WithSeries(seriesID), WithInflightOutput())
	c.AddOutputFunc(func(product Product) error {
		products = append(products, product)
		return nil
	})

	timestamp := time.Date(2026, time.January, 1, 1, 0, 0, 0, time.UTC)
	c.receive(&Gather{ts: timestamp, measures: []Measure{{Name: "requests", Value: 1, Type: CounterType(UnitShort)}}})
	c.receive(&Gather{ts: timestamp.Add(time.Second), measures: []Measure{{Name: "requests", Value: 1, Type: CounterType(UnitShort)}}})

	require.Len(t, products, 3)
	require.True(t, products[0].Inflight)
	require.False(t, products[1].Inflight)
	require.True(t, products[2].Inflight)
	require.Equal(t, timestamp, products[0].Time)
	require.Equal(t, timestamp, products[1].Time)
	require.Equal(t, timestamp.Add(time.Second), products[2].Time)
}
