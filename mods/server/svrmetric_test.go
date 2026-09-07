package server

import (
	"context"
	"database/sql"
	"expvar"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

func gatherMeasureNames(g *metric.Gather) []string {
	v := reflect.ValueOf(g).Elem().FieldByName("measures")
	names := make([]string, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		names = append(names, v.Index(i).FieldByName("Name").String())
	}
	return names
}

func TestStopServerMetrics(t *testing.T) {
	startStatzWorker()
	require.NotPanics(t, func() {
		stopServerMetrics()
	})
}

func TestCollectMqttStatz(t *testing.T) {
	t.Run("missing_broker", func(t *testing.T) {
		g := &metric.Gather{}
		err := collectMqttStatz(&Server{})(g)
		require.Error(t, err)
		require.ErrorContains(t, err, "MQTT broker is not initialized")
	})

	t.Run("collects_metrics_from_broker_info", func(t *testing.T) {
		mqttd, err := NewMqtt()
		require.NoError(t, err)
		t.Cleanup(func() {
			mqttd.Stop()
		})

		mqttd.broker.Info.BytesReceived = 11
		mqttd.broker.Info.BytesSent = 12
		mqttd.broker.Info.MessagesReceived = 13
		mqttd.broker.Info.MessagesSent = 14
		mqttd.broker.Info.MessagesDropped = 15
		mqttd.broker.Info.PacketsSent = 16
		mqttd.broker.Info.PacketsReceived = 17
		mqttd.broker.Info.Retained = 18
		mqttd.broker.Info.Subscriptions = 19
		mqttd.broker.Info.ClientsTotal = 20
		mqttd.broker.Info.ClientsConnected = 21
		mqttd.broker.Info.ClientsDisconnected = 22
		mqttd.broker.Info.Inflight = 23
		mqttd.broker.Info.InflightDropped = 24

		g := &metric.Gather{}
		err = collectMqttStatz(&Server{mqttd: mqttd})(g)
		require.NoError(t, err)

		names := gatherMeasureNames(g)
		require.Len(t, names, 14)
		require.Contains(t, names, "mqtt:recv_bytes")
		require.Contains(t, names, "mqtt:clients_connected")
		require.Contains(t, names, "mqtt:inflight_dropped")
	})
}

func TestAddDefaultPoolStatz(t *testing.T) {
	g := &metric.Gather{}
	stat := sql.DBStats{
		MaxOpenConnections: 10,
		OpenConnections:    8,
		InUse:              3,
		Idle:               5,
		WaitCount:          7,
		WaitDuration:       12 * time.Millisecond,
		MaxIdleClosed:      1,
		MaxIdleTimeClosed:  2,
		MaxLifetimeClosed:  4,
	}

	addDefaultPoolStatz(g, stat)

	names := gatherMeasureNames(g)
	require.Len(t, names, 9)
	require.Contains(t, names, "sys:pool:max_open")
	require.Contains(t, names, "sys:pool:open")
	require.Contains(t, names, "sys:pool:in_use")
	require.Contains(t, names, "sys:pool:idle")
	require.Contains(t, names, "sys:pool:wait_count")
	require.Contains(t, names, "sys:pool:wait_duration")
	require.Contains(t, names, "sys:pool:max_idle_closed")
	require.Contains(t, names, "sys:pool:max_idletime_closed")
	require.Contains(t, names, "sys:pool:max_lifetime_closed")
}

func TestStatzKeys(t *testing.T) {
	prefix := spi.MetricsPrefix()
	suffix := fmt.Sprintf("unit_test_statz_keys_%d", time.Now().UnixNano())
	fullKey := prefix + ":" + suffix

	v := expvar.NewInt(fullKey)
	v.Set(1)

	keys := statzKeys([]string{suffix})
	require.Equal(t, []string{suffix}, keys)
}

func TestStatzQuery(t *testing.T) {
	t.Run("returns_rows_and_types", func(t *testing.T) {
		prefix := spi.MetricsPrefix()
		suffix := fmt.Sprintf("unit_test_statz_query_%d", time.Now().UnixNano())
		fullKey := prefix + ":" + suffix

		v := expvar.NewInt(fullKey)
		v.Set(42)

		result, err := statzQuery(2, []string{suffix})
		require.NoError(t, err)
		require.Equal(t, []string{"time", suffix}, result.Columns)
		require.Equal(t, []string{"datetime", "int64"}, result.Types)
		require.Len(t, result.Rows, 2)
		require.Len(t, result.Rows[0], 2)
		require.Equal(t, int64(42), result.Rows[0][1])
		require.Equal(t, int64(42), result.Rows[1][1])

		_, parseErr := time.Parse(time.RFC3339, result.Rows[0][0].(string))
		require.NoError(t, parseErr)
	})

	t.Run("returns_error_when_no_metrics_match", func(t *testing.T) {
		_, err := statzQuery(1, []string{fmt.Sprintf("missing_metric_%d", time.Now().UnixNano())})
		require.Error(t, err)
		require.ErrorContains(t, err, "no metrics found")
	})
}

func TestStatzViz(t *testing.T) {
	spi.StartMetrics()
	t.Cleanup(func() {
		spi.StopMetrics()
	})

	result, err := statzViz([]string{"sys:session:count", "sys:rollup#name=example"})
	require.Error(t, err)
	require.ErrorContains(t, err, "no metrics found")
	require.Nil(t, result)
}

func TestExtractStatzRecords(t *testing.T) {
	now := time.Now()

	t.Run("counter", func(t *testing.T) {
		pd := metric.Product{
			Name:     "req_count",
			Time:     now,
			SeriesID: spi.SERIES_ID_FINEST,
			Value:    &metric.CounterValue{Value: 10, Samples: 1},
		}
		recs := extractStatzRecords(pd)
		require.Len(t, recs, 1)
		require.Equal(t, "req_count", recs[0].Name)
		require.Equal(t, 10.0, recs[0].Value)
		require.Equal(t, now.UnixNano(), recs[0].Time)
	})

	t.Run("gauge", func(t *testing.T) {
		pd := metric.Product{
			Name:     "mem_gauge",
			Time:     now,
			SeriesID: spi.SERIES_ID_FINEST,
			Value:    &metric.GaugeValue{Value: 2048, Samples: 1},
		}
		recs := extractStatzRecords(pd)
		require.Len(t, recs, 1)
		require.Equal(t, "mem_gauge", recs[0].Name)
		require.Equal(t, 2048.0, recs[0].Value)
	})

	t.Run("meter", func(t *testing.T) {
		pd := metric.Product{
			Name:     "latency",
			Time:     now,
			SeriesID: spi.SERIES_ID_FINEST,
			Value:    &metric.MeterValue{Min: 1.0, Max: 10.0, Sum: 20.0, Samples: 4},
		}
		recs := extractStatzRecords(pd)
		require.Len(t, recs, 3)
		require.Equal(t, "latency:min", recs[0].Name)
		require.Equal(t, 1.0, recs[0].Value)
		require.Equal(t, "latency:max", recs[1].Name)
		require.Equal(t, 10.0, recs[1].Value)
		require.Equal(t, "latency:avg", recs[2].Name)
		require.Equal(t, 5.0, recs[2].Value)
	})

	t.Run("histogram", func(t *testing.T) {
		pd := metric.Product{
			Name:     "exec_hist",
			Time:     now,
			SeriesID: spi.SERIES_ID_FINEST,
			Value:    &metric.HistogramValue{Samples: 5, P: []float64{0.5, 0.99}, Values: []float64{12.3, 45.6}},
		}
		recs := extractStatzRecords(pd)
		require.Len(t, recs, 3)
		require.Equal(t, "exec_hist", recs[0].Name)
		require.Equal(t, 5.0, recs[0].Value)
		require.Equal(t, "exec_hist:p50", recs[1].Name)
		require.Equal(t, 12.3, recs[1].Value)
		require.Equal(t, "exec_hist:p99", recs[2].Name)
		require.Equal(t, 45.6, recs[2].Value)
	})

	t.Run("odometer", func(t *testing.T) {
		pd := metric.Product{
			Name:     "io_odo",
			Time:     now,
			SeriesID: spi.SERIES_ID_FINEST,
			Value:    &metric.OdometerValue{First: 0, Last: 100, Samples: 1},
		}
		recs := extractStatzRecords(pd)
		require.Len(t, recs, 1)
		require.Equal(t, "io_odo", recs[0].Name)
		require.Equal(t, 100.0, recs[0].Value)
	})

	t.Run("skip_coarser_series", func(t *testing.T) {
		pd := metric.Product{
			Name:     "req_count",
			Time:     now,
			SeriesID: spi.SERIES_ID_FINE,
			Value:    &metric.CounterValue{Value: 10, Samples: 1},
		}
		recs := extractStatzRecords(pd)
		require.Nil(t, recs)
	})

	t.Run("skip_zero_samples", func(t *testing.T) {
		pd := metric.Product{
			Name:     "req_count",
			Time:     now,
			SeriesID: spi.SERIES_ID_FINEST,
			Value:    &metric.CounterValue{Value: 0, Samples: 0},
		}
		recs := extractStatzRecords(pd)
		require.Nil(t, recs)
	})

	t.Run("unknown_type", func(t *testing.T) {
		pd := metric.Product{
			Name:     "custom_metric",
			Time:     now,
			SeriesID: spi.SERIES_ID_FINEST,
			Value:    customMetricValue{},
		}
		recs := extractStatzRecords(pd)
		require.Nil(t, recs)
	})
}

type customMetricValue struct{}

func (c customMetricValue) String() string {
	return "custom"
}

func TestStoreStatzEdgeCases(t *testing.T) {
	t.Run("empty_records", func(t *testing.T) {
		pd := metric.Product{
			Name:     "empty",
			Time:     time.Now(),
			SeriesID: spi.SERIES_ID_FINEST,
			Value:    &metric.CounterValue{Samples: 0},
		}
		err := storeStatz(pd)
		require.NoError(t, err)
	})

	t.Run("queue_nil", func(t *testing.T) {
		stopStatzWorker()
		pd := metric.Product{
			Name:     "test_gauge",
			Time:     time.Now(),
			SeriesID: spi.SERIES_ID_FINEST,
			Value:    &metric.GaugeValue{Value: 1.0, Samples: 1},
		}
		err := storeStatz(pd)
		require.NoError(t, err)
	})

	t.Run("queue_full_drops_gracefully", func(t *testing.T) {
		startStatzWorker()
		defer stopStatzWorker()

		for i := 0; i < statzQueueCapacity+100; i++ {
			_ = storeStatz(metric.Product{
				Name:     "burst_metric",
				Time:     time.Now(),
				SeriesID: spi.SERIES_ID_FINEST,
				Value:    &metric.GaugeValue{Value: float64(i), Samples: 1},
			})
		}
	})
}

func TestFlushStatzBatchAndEnsureTable(t *testing.T) {
	t.Run("flush_empty_batch", func(t *testing.T) {
		require.NotPanics(t, func() {
			flushStatzBatch(nil)
			flushStatzBatch([]statzRecord{})
		})
	})

	t.Run("ensure_table_cached", func(t *testing.T) {
		statzStoreExists = true
		t.Cleanup(func() {
			statzStoreExists = false
		})
		err := ensureStatzTable(context.Background(), nil)
		require.NoError(t, err)
	})

	t.Run("flush_batch_with_records", func(t *testing.T) {
		now := time.Now()
		records := []statzRecord{
			{
				Name:  "test_metric_flush",
				Time:  now.UnixNano(),
				Value: 123.456,
			},
		}
		require.NotPanics(t, func() {
			flushStatzBatch(records)
		})
	})
}

func TestCollectTqlCacheStatz(t *testing.T) {
	g := &metric.Gather{}
	err := collectTqlCacheStatz(g)
	require.NoError(t, err)

	names := gatherMeasureNames(g)
	require.Len(t, names, 5)
	require.Contains(t, names, "tql:cache:evictions")
	require.Contains(t, names, "tql:cache:insertions")
	require.Contains(t, names, "tql:cache:hits")
	require.Contains(t, names, "tql:cache:misses")
	require.Contains(t, names, "tql:cache:items")
}

func TestStatzWorkerLifecycle(t *testing.T) {
	startStatzWorker()
	t.Cleanup(func() {
		stopStatzWorker()
	})

	pd := metric.Product{
		Name:     "unit_test_lifecycle",
		Time:     time.Now(),
		SeriesID: spi.SERIES_ID_FINEST,
		Value:    &metric.GaugeValue{Value: 123.45, Samples: 1},
	}
	err := storeStatz(pd)
	require.NoError(t, err)

	stopStatzWorker()
	require.Nil(t, statzQueue)
}
