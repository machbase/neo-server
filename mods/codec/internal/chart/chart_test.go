package chart_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/echart"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func TestLine(t *testing.T) {
	buffer := &bytes.Buffer{}

	line := echart.NewRectChart(echart.LINE)
	opts := []opts.Option{
		opts.OutputStream(stream.NewOutputStreamWriter(buffer)),
		opts.ChartJson(true),
		opts.TimeLocation(time.UTC),
		opts.XAxis(0, "time", "time"),
		opts.YAxis(1, "demo"),
		opts.Timeformat("15:04:05.999999999"),
		opts.DataZoom("slider", 0, 100),
		opts.SeriesLabels("test-data"),
		opts.Title("Title"),
		opts.Subtitle("substitle"),
		opts.Theme("westerose"),
		opts.Size("600px", "600px"),
	}
	for _, o := range opts {
		o(line)
	}

	require.Equal(t, "application/json", line.ContentType())

	line.Open()
	tick := time.Unix(0, 1692670838086467000)
	line.AddRow([]any{tick.Add(0 * time.Second), 0.0})
	line.AddRow([]any{tick.Add(1 * time.Second), 1.0})
	line.AddRow([]any{tick.Add(2 * time.Second), 2.0})
	line.Flush(false)
	line.Close()

	substr := `"xAxis":[{"name":"time","show":true,"data":["02:20:38.086467","02:20:39.086467","02:20:40.086467"]`
	require.True(t, strings.Contains(buffer.String(), substr))
}

func TestScatter(t *testing.T) {
	buffer := &bytes.Buffer{}

	line := echart.NewRectChart(echart.SCATTER)
	opts := []opts.Option{
		opts.OutputStream(stream.NewOutputStreamWriter(buffer)),
		opts.ChartJson(true),
		opts.TimeLocation(time.UTC),
		opts.XAxis(0, "time", "time"),
		opts.YAxis(1, "demo"),
		opts.Timeformat("15:04:05.999999999"),
		opts.DataZoom("slider", 0, 100),
		opts.SeriesLabels("test-data"),
	}
	for _, o := range opts {
		o(line)
	}

	require.Equal(t, "application/json", line.ContentType())

	line.Open()
	tick := time.Unix(0, 1692670838086467000)
	line.AddRow([]any{tick.Add(0 * time.Second), 0.0})
	line.AddRow([]any{tick.Add(1 * time.Second), 1.0})
	line.AddRow([]any{tick.Add(2 * time.Second), 2.0})
	line.Flush(false)
	line.Close()

	substr := `"xAxis":[{"name":"time","show":true,"data":["02:20:38.086467","02:20:39.086467","02:20:40.086467"]`
	require.True(t, strings.Contains(buffer.String(), substr))
}

func TestBar(t *testing.T) {
	buffer := &bytes.Buffer{}

	line := echart.NewRectChart(echart.BAR)
	opts := []opts.Option{
		opts.OutputStream(stream.NewOutputStreamWriter(buffer)),
		opts.ChartJson(true),
		opts.TimeLocation(time.UTC),
		opts.XAxis(0, "time", "time"),
		opts.YAxis(1, "demo"),
		opts.Timeformat("15:04:05.999999999"),
		opts.DataZoom("slider", 0, 100),
		opts.SeriesLabels("test-data"),
	}
	for _, o := range opts {
		o(line)
	}

	require.Equal(t, "application/json", line.ContentType())

	line.Open()
	tick := time.Unix(0, 1692670838086467000)
	line.AddRow([]any{tick.Add(0 * time.Second), 0.0})
	line.AddRow([]any{tick.Add(1 * time.Second), 1.0})
	line.AddRow([]any{tick.Add(2 * time.Second), 2.0})
	line.Flush(false)
	line.Close()

	substr := `"xAxis":[{"name":"time","show":true,"data":["02:20:38.086467","02:20:39.086467","02:20:40.086467"]`
	require.True(t, strings.Contains(buffer.String(), substr))
}

func TestLine3D(t *testing.T) {
	buffer := &bytes.Buffer{}

	line := echart.NewLine3D()
	opts := []opts.Option{
		opts.OutputStream(stream.NewOutputStreamWriter(buffer)),
		opts.ChartJson(true),
		opts.TimeLocation(time.UTC),
		opts.XAxis(0, "time", "time"),
		opts.YAxis(1, "demo"),
		opts.Timeformat("15:04:05.999999999"),
		opts.DataZoom("slider", 0, 100),
		opts.SeriesLabels("test-data"),
		opts.Title("Title"),
		opts.Subtitle("substitle"),
		opts.Theme("westerose"),
		opts.Size("600px", "600px"),
	}
	for _, o := range opts {
		o(line)
	}

	require.Equal(t, "application/json", line.ContentType())

	line.Open()
	tick := time.Unix(0, 1692670838086467000)
	line.AddRow([]any{tick.Add(0 * time.Second), 0.0, 0.0})
	line.AddRow([]any{tick.Add(1 * time.Second), 1.0, 1.0})
	line.AddRow([]any{tick.Add(2 * time.Second), 2.0, 2.0})
	line.Flush(false)
	line.Close()

	substr := `"data":[{"value":[1692670838086,0,0]}]}`
	require.True(t, strings.Contains(buffer.String(), substr))
	substr = `"data":[{"value":[1692670839086,1,1]}]}`
	require.True(t, strings.Contains(buffer.String(), substr))
	substr = `data":[{"value":[1692670840086,2,2]}]}]`
	require.True(t, strings.Contains(buffer.String(), substr))
}
