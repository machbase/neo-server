package metric

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetric(t *testing.T) {
	now := time.Unix(1756251515, 0)
	nowFunc = func() time.Time {
		now = now.Add(time.Second)
		return now
	}
	var wg sync.WaitGroup
	var out string
	var cnt int
	wg.Add(3)
	c := NewCollector(
		WithCollectInterval(time.Second),
		WithSeriesListener("1m/1s", time.Second, 60, func(pd ProducedData) {
			defer wg.Done()
			out = fmt.Sprintf("%s:%s %s %v %s %s",
				pd.Measure, pd.Field, pd.Series, pd.Time.Format(time.TimeOnly), pd.Value.String(), pd.Type)
			if cnt++; cnt == 1 {
				require.Equal(t, `m1:f1 1m/1s 08:38:37 {"samples":1,"value":1} counter`, out)
			} else if cnt == 2 {
				require.Equal(t, `m1:f1 1m/1s 08:38:38 {"samples":1,"value":1} counter`, out)
			} else if cnt == 3 {
				require.Equal(t, `m1:f1 1m/1s 08:38:39 {"samples":1,"value":1} counter`, out)
			}
		}),
	)
	c.AddInputFunc(func() (Measurement, error) {
		m := Measurement{Name: "m1"}
		m.AddField(Field{Name: "f1", Value: 1.0, Type: CounterType(UnitShort)})
		return m, nil
	})
	c.Start()
	wg.Wait()
	c.Stop()
}
