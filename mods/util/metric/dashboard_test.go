package metric

import (
	"testing"

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
