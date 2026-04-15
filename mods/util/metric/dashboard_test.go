package metric

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrimSeriesNames(t *testing.T) {
	series := []Series{
		{Name: "cpu:cpu_user#avg"},
		{Name: "cpu:cpu_system#avg"},
		{Name: "cpu:cpu_idle#avg"},
	}
	trimSeriesNames(series)
	trimSeriesNames(series)
	require.Equal(t, "cpu_user", series[0].Name)
	require.Equal(t, "cpu_system", series[1].Name)
	require.Equal(t, "cpu_idle", series[2].Name)

	series2 := []Series{
		{Name: "mem:heap_inuse:10s"},
		{Name: "mem:heap_alloc:10s"},
		{Name: "mem:heap_idle:10s"},
	}
	trimSeriesNames(series2)
	require.Equal(t, "heap_inuse", series2[0].Name)
	require.Equal(t, "heap_alloc", series2[1].Name)
	require.Equal(t, "heap_idle", series2[2].Name)

	series3 := []Series{
		{Name: "go:goroutines"},
		{Name: "go:threads"},
	}
	trimSeriesNames(series3)
	require.Equal(t, "goroutines", series3[0].Name)
	require.Equal(t, "threads", series3[1].Name)
}

func TestDashboardHandlersAndOptions(t *testing.T) {
	collector := NewCollector(WithSamplingInterval(time.Second))
	dashboard := NewDashboard(collector)
	seriesID, err := NewSeriesID("CPU_USAGE", "CPU Usage", time.Second, 8)
	require.NoError(t, err)
	dashboard.Timeseries = []SeriesID{seriesID}
	dashboard.nameProvider = func() []string {
		return []string{"cpu:usage", "mem:alloc"}
	}
	dashboard.timeseriesProvider = func(name string) MultiTimeSeries {
		if name != "cpu:usage" {
			return nil
		}
		ts := NewTimeSeries(time.Second, 8, NewCounter(), WithMeta(SeriesInfo{
			MeasureName: name,
			MeasureType: CounterType(UnitShort),
			SeriesID:    seriesID,
		}))
		ts.Add(1)
		ts.Add(2)
		return MultiTimeSeries{ts}
	}

	require.NoError(t, dashboard.AddChart(Chart{ID: "cpu", Title: "CPU", MetricNames: []string{"cpu:*"}}))
	panels := dashboard.Panels()
	require.Len(t, panels, 1)
	require.Contains(t, panels[0].MetricNames, "cpu:usage")

	dashboard.SetTheme("light")
	dashboard.SetPanelHeight("240px")
	dashboard.SetPanelMinWidth("300px")
	dashboard.SetPanelMaxWidth("2fr")
	css := string(dashboard.Option.StyleCSS())
	require.Contains(t, css, "240px")
	require.Contains(t, css, "300px")
	require.Contains(t, css, "2fr")

	indexReq := httptest.NewRequest(http.MethodGet, "/?showRemains=true&tsIdx=0", nil)
	indexRec := httptest.NewRecorder()
	dashboard.ServeHTTP(indexRec, indexReq)
	require.Equal(t, http.StatusOK, indexRec.Code)
	require.Contains(t, indexRec.Header().Get("Content-Type"), "text/html")
	require.Contains(t, indexRec.Body.String(), "Metrics")

	dataReq := httptest.NewRequest(http.MethodGet, "/?id=cpu&tsIdx=0", nil)
	dataRec := httptest.NewRecorder()
	dashboard.ServeHTTP(dataRec, dataReq)
	require.Equal(t, http.StatusOK, dataRec.Code)
	require.Contains(t, dataRec.Header().Get("Content-Type"), "application/json")
	require.Contains(t, dataRec.Body.String(), "chartOption")
	require.Contains(t, dataRec.Body.String(), "CPU")
}
