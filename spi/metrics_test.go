package spi

import (
	"encoding/json"
	"expvar"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/jsh/viz"
	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/stretchr/testify/require"
)

func TestHandleStatz(t *testing.T) {
	metricKey := "custom:" + uuid.Must(uuid.NewV4()).String()
	metricValue := expvar.NewInt(metricKey)
	metricValue.Set(42)

	t.Run("json with invalid interval fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/debug/statz?interval=not-a-duration&keys="+url.QueryEscape(metricKey), nil)
		writer := httptest.NewRecorder()

		HandleStatz(writer, req)

		require.Equal(t, http.StatusOK, writer.Code)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(writer.Body.Bytes(), &payload))
		require.EqualValues(t, 42, payload[metricKey])
	})

	t.Run("html renders included metric", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/debug/statz?format=html&keys="+url.QueryEscape(metricKey), nil)
		writer := httptest.NewRecorder()

		HandleStatz(writer, req)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Contains(t, writer.Body.String(), "<table>")
		require.Contains(t, writer.Body.String(), metricKey)
		require.Contains(t, writer.Body.String(), "42")
	})
}

func TestVizSpecGenerator(t *testing.T) {
	seriesID := mustSeriesID(t, "CPU_USAGE", "CPU Usage", time.Second, 8)
	collector := metric.NewCollector(
		metric.WithSamplingInterval(time.Second),
		metric.WithSeries(seriesID),
	)
	collector.Start()
	t.Cleanup(collector.Stop)

	collector.Send(metric.Measure{Name: "cpu:usage", Value: 1, Type: metric.CounterType(metric.UnitScalar)})
	time.Sleep(1100 * time.Millisecond)
	collector.Send(metric.Measure{Name: "cpu:usage", Value: 2, Type: metric.CounterType(metric.UnitScalar)})

	require.Eventually(t, func() bool {
		mts := collector.Timeseries("cpu:usage")
		if len(mts) == 0 {
			return false
		}
		times, _ := mts[0].All()
		return len(times) >= 2
	}, 3*time.Second, 50*time.Millisecond)

	gen := NewVizSpecGenerator(collector)
	require.NoError(t, gen.AddChart(metric.Chart{ID: "cpu", Title: "CPU", MetricNames: []string{"cpu:usage"}}))

	spec, err := gen.Generate("cpu", 0)
	require.NoError(t, err)
	require.Equal(t, 1, spec.Version)
	require.Equal(t, "time", spec.Domain.Kind)
	require.Equal(t, "ns", spec.Domain.Timeformat)
	require.Equal(t, "time", spec.Axes.X.Type)
	require.Len(t, spec.Series, 1)
	require.Equal(t, "time-bucket-value", spec.Series[0].Representation.Kind)
	require.Equal(t, "cpu_usage", spec.Series[0].Axis)
	require.GreaterOrEqual(t, len(spec.Series[0].Data), 2)
	require.Equal(t, float64(1), spec.Series[0].Data[len(spec.Series[0].Data)-2].([]any)[1])
	require.Equal(t, float64(2), spec.Series[0].Data[len(spec.Series[0].Data)-1].([]any)[1])

	blocks, err := viz.ToTUIBlocks(spec)
	for _, block := range blocks {
		t.Logf("Block Type: %s, Rows: %v", block.Type, block.Rows)
		t.Log(strings.Join(block.Lines, "\n"))
	}
	require.NoError(t, err)
	require.Len(t, blocks, 4)
	require.Equal(t, "summary", blocks[0].Type)
	require.Equal(t, "series-summary", blocks[1].Type)
	require.Equal(t, "sparkline", blocks[2].Type)
	require.Equal(t, "table", blocks[3].Type)
	require.Len(t, blocks[2].Lines, 1)
	require.Equal(t, "▁█", blocks[2].Lines[0])
	require.GreaterOrEqual(t, len(blocks[3].Rows), 2)
	firstRow := blocks[3].Rows[len(blocks[3].Rows)-2].([]any)
	secondRow := blocks[3].Rows[len(blocks[3].Rows)-1].([]any)
	require.Equal(t, float64(1), firstRow[1])
	require.Equal(t, float64(2), secondRow[1])
}

func mustSeriesID(t *testing.T, id, title string, period time.Duration, maxCount int) metric.SeriesID {
	t.Helper()
	seriesID, err := metric.NewSeriesID(id, title, period, maxCount)
	require.NoError(t, err)
	return seriesID
}
