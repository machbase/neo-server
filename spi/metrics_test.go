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

func TestHandlePrometheusMetrics(t *testing.T) {
	metricKey := "machbase:test:" + uuid.Must(uuid.NewV4()).String()
	metricValue := expvar.NewInt(metricKey)
	metricValue.Set(42)

	prevToken := PrometheusBearerToken()
	t.Cleanup(func() {
		SetPrometheusBearerToken(prevToken)
	})

	t.Run("exports prometheus text format", func(t *testing.T) {
		SetPrometheusBearerToken("")
		req := httptest.NewRequest(http.MethodGet, "/debug/metrics?interval=not-a-duration&keys="+url.QueryEscape(metricKey), nil)
		writer := httptest.NewRecorder()

		HandlePrometheusMetrics(writer, req)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Contains(t, writer.Header().Get("Content-Type"), "text/plain")
		require.Contains(t, writer.Body.String(), "# HELP test_")
		require.Contains(t, writer.Body.String(), "# TYPE test_")
		require.Contains(t, writer.Body.String(), "42")
	})

	t.Run("enforces bearer token when configured", func(t *testing.T) {
		SetPrometheusBearerToken("prom-token")

		t.Run("without token", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/debug/metrics?keys="+url.QueryEscape(metricKey), nil)
			writer := httptest.NewRecorder()

			HandlePrometheusMetrics(writer, req)

			require.Equal(t, http.StatusUnauthorized, writer.Code)
		})

		t.Run("with wrong token", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/debug/metrics?keys="+url.QueryEscape(metricKey), nil)
			req.Header.Set("Authorization", "Bearer wrong-token")
			writer := httptest.NewRecorder()

			HandlePrometheusMetrics(writer, req)

			require.Equal(t, http.StatusUnauthorized, writer.Code)
		})

		t.Run("with matching token", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/debug/metrics?keys="+url.QueryEscape(metricKey), nil)
			req.Header.Set("Authorization", "Bearer prom-token")
			writer := httptest.NewRecorder()

			HandlePrometheusMetrics(writer, req)

			require.Equal(t, http.StatusOK, writer.Code)
			require.Contains(t, writer.Body.String(), "test_")
		})
	})
}

func TestPrometheusHelperFunctions(t *testing.T) {
	t.Run("sanitizePromMetricName", func(t *testing.T) {
		require.Equal(t, "neo_metric", sanitizePromMetricName(""))
		require.Equal(t, "neo_1abc", sanitizePromMetricName("1abc"))
		require.Equal(t, "cpu_usage", sanitizePromMetricName("machbase:cpu-usage"))
		require.Equal(t, "neo_metric", sanitizePromMetricName("!!!"))
	})

	t.Run("inferPromMetricType", func(t *testing.T) {
		require.Equal(t, "counter", inferPromMetricType("request_total"))
		require.Equal(t, "counter", inferPromMetricType("request_count"))
		require.Equal(t, "counter", inferPromMetricType("recv_bytes"))
		require.Equal(t, "gauge", inferPromMetricType("cpu_usage"))
	})

	t.Run("toPromFloat", func(t *testing.T) {
		tests := []struct {
			name    string
			input   any
			value   float64
			success bool
		}{
			{name: "float64", input: float64(1.25), value: 1.25, success: true},
			{name: "float32", input: float32(1.5), value: 1.5, success: true},
			{name: "int", input: int(3), value: 3, success: true},
			{name: "int8", input: int8(-8), value: -8, success: true},
			{name: "int16", input: int16(-16), value: -16, success: true},
			{name: "int32", input: int32(7), value: 7, success: true},
			{name: "int64", input: int64(64), value: 64, success: true},
			{name: "uint", input: uint(4), value: 4, success: true},
			{name: "uint8", input: uint8(8), value: 8, success: true},
			{name: "uint16", input: uint16(16), value: 16, success: true},
			{name: "uint32", input: uint32(32), value: 32, success: true},
			{name: "uint64", input: uint64(9), value: 9, success: true},
			{name: "string", input: "not-number", value: 0, success: false},
			{name: "struct", input: struct{ N int }{N: 1}, value: 0, success: false},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f, ok := toPromFloat(tc.input)
				require.Equal(t, tc.success, ok)
				if tc.success {
					require.Equal(t, tc.value, f)
				}
			})
		}
	})
}

func TestAllowPrometheusRequest(t *testing.T) {
	prevToken := PrometheusBearerToken()
	t.Cleanup(func() {
		SetPrometheusBearerToken(prevToken)
	})

	t.Run("no token configured", func(t *testing.T) {
		SetPrometheusBearerToken("")
		req := httptest.NewRequest(http.MethodGet, "/debug/metrics", nil)
		require.True(t, allowPrometheusRequest(req))
	})

	t.Run("token configured", func(t *testing.T) {
		SetPrometheusBearerToken("token-123")

		reqNoHeader := httptest.NewRequest(http.MethodGet, "/debug/metrics", nil)
		require.False(t, allowPrometheusRequest(reqNoHeader))

		reqWrong := httptest.NewRequest(http.MethodGet, "/debug/metrics", nil)
		reqWrong.Header.Set("Authorization", "Bearer wrong")
		require.False(t, allowPrometheusRequest(reqWrong))

		reqCaseInsensitive := httptest.NewRequest(http.MethodGet, "/debug/metrics", nil)
		reqCaseInsensitive.Header.Set("Authorization", "bearer token-123")
		require.True(t, allowPrometheusRequest(reqCaseInsensitive))

		reqMalformed := httptest.NewRequest(http.MethodGet, "/debug/metrics", nil)
		reqMalformed.Header.Set("Authorization", "Bearer")
		require.False(t, allowPrometheusRequest(reqMalformed))
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
		_, values := mts[0].All()
		samples := make([]float64, 0, len(values))
		for _, raw := range values {
			v, ok := raw.(*metric.CounterValue)
			if !ok || v.Samples == 0 {
				continue
			}
			samples = append(samples, v.Value)
		}
		if len(samples) < 2 {
			return false
		}
		return samples[len(samples)-2] == 1 && samples[len(samples)-1] == 2
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
	points := make([][]any, 0, len(spec.Series[0].Data))
	for _, raw := range spec.Series[0].Data {
		item, ok := raw.([]any)
		if !ok || len(item) < 2 || item[1] == nil {
			continue
		}
		points = append(points, item)
	}
	require.GreaterOrEqual(t, len(points), 2)
	require.Equal(t, float64(1), points[len(points)-2][1])
	require.Equal(t, float64(2), points[len(points)-1][1])

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
	rows := make([][]any, 0, len(blocks[3].Rows))
	for _, raw := range blocks[3].Rows {
		row, ok := raw.([]any)
		if !ok || len(row) < 2 || row[1] == nil {
			continue
		}
		rows = append(rows, row)
	}
	require.GreaterOrEqual(t, len(rows), 2)
	firstRow := rows[len(rows)-2]
	secondRow := rows[len(rows)-1]
	require.Equal(t, float64(1), firstRow[1])
	require.Equal(t, float64(2), secondRow[1])
}

func mustSeriesID(t *testing.T, id, title string, period time.Duration, maxCount int) metric.SeriesID {
	t.Helper()
	seriesID, err := metric.NewSeriesID(id, title, period, maxCount)
	require.NoError(t, err)
	return seriesID
}
