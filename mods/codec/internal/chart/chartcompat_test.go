package chart_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/chart"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func TestCompat(t *testing.T) {
	for _, output := range []string{"json", "html"} {
		buffer := &bytes.Buffer{}
		fsmock := &VolatileFileWriterMock{}
		c := chart.NewRectChart("line")
		c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
		c.SetVolatileFileWriter(fsmock)
		c.SetChartJson(output == "json")
		c.SetChartId("WejMYXCGcYNL")
		c.SetTheme("westeros")
		c.SetTitle("Title")
		c.SetSubtitle("subtitle")
		c.SetGlobalOptions(`{"animation":true, "color":["#80FFA5", "#00DDFF", "#37A2FF"]}`)
		c.SetSize("400px", "300px")
		c.SetDataZoom("slider", 0, 100)
		c.SetToolboxSaveAsImage("test.png")
		c.SetToolboxDataView()
		c.SetToolboxDataZoom()
		c.SetXAxis(0, "time", "time")
		c.SetVisualMapColor(-2.0, 2.0,
			"#a50026", "#d73027", "#f46d43", "#fdae61", "#e0f3f8",
			"#abd9e9", "#74add1", "#4575b4", "#313695", "#313695",
			"#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#fdae61",
			"#f46d43", "#d73027", "#a50026")
		if output == "json" {
			require.Equal(t, "application/json", c.ContentType())
		} else {
			require.Equal(t, "text/html", c.ContentType())
		}

		tick := time.Unix(0, 1692670838086467000)

		c.SetMarkAreaNameCoord(tick.Add(500*time.Millisecond), tick.Add(1*time.Second), "Area1", "#ff000033", 0.3)
		c.SetMarkAreaNameCoord(tick.Add(600*time.Millisecond), tick.Add(1200*time.Millisecond), "Area2", "#ff000033", 0.3)
		c.SetMarkLineXAxisCoord(tick.Add(200*time.Millisecond), "line-X")
		c.SetMarkLineYAxisCoord(0.5, "half")
		c.Open()
		c.AddRow([]any{tick.Add(0 * time.Second), -2.0})
		c.AddRow([]any{tick.Add(1 * time.Second), -1.0})
		c.AddRow([]any{tick.Add(2 * time.Second), 0.0})
		c.AddRow([]any{tick.Add(3 * time.Second), 1.0})
		c.AddRow([]any{tick.Add(4 * time.Second), 2.0})
		c.Close()

		expect, err := os.ReadFile(filepath.Join("test", fmt.Sprintf("compat_line.%s", output)))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr := string(expect)
		if output == "json" {
			require.JSONEq(t, expectStr, buffer.String(), "json result unmatched\n%s", buffer.String())
		} else {
			if !StringsEq(t, expectStr, buffer.String()) {
				require.Equal(t, expectStr, buffer.String(), "html result unmatched\n%s", buffer.String())
			}
		}

		expect, err = os.ReadFile(filepath.Join("test", "compat_line.js"))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr = string(expect)
		if !StringsEq(t, expectStr, fsmock.buff.String()) {
			require.Equal(t, expectStr, fsmock.buff.String(), "js result unmatched\n%s", fsmock.buff.String())
		}
	}
}

func TestScatterCompat(t *testing.T) {
	buffer := &bytes.Buffer{}
	fsmock := &VolatileFileWriterMock{}

	line := chart.NewRectChart("scatter")
	opts := []opts.Option{
		opts.OutputStream(stream.NewOutputStreamWriter(buffer)),
		opts.VolatileFileWriter(fsmock),
		opts.ChartId("MjYwMjY0NTY1OTY2MTUxNjg_"),
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

	expect, err := os.ReadFile(filepath.Join("test", "compat_scatter.json"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	expectStr := string(expect)
	require.JSONEq(t, expectStr, buffer.String(), "json result unmatched\n%s", buffer.String())

	expect, err = os.ReadFile(filepath.Join("test", "compat_scatter.js"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	expectStr = string(expect)
	if !StringsEq(t, expectStr, fsmock.buff.String()) {
		require.Equal(t, expectStr, fsmock.buff.String(), "js result unmatched\n%s", fsmock.buff.String())
	}
}

func TestBarCompat(t *testing.T) {
	buffer := &bytes.Buffer{}
	fsmock := &VolatileFileWriterMock{}

	line := chart.NewRectChart("bar")
	opts := []opts.Option{
		opts.OutputStream(stream.NewOutputStreamWriter(buffer)),
		opts.VolatileFileWriter(fsmock),
		opts.ChartJson(true),
		opts.ChartId("MjYwMjY0NTY1OTY2MTUxNjg_"),
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

	expect, err := os.ReadFile(filepath.Join("test", "compat_bar.json"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	expectStr := string(expect)
	require.JSONEq(t, expectStr, buffer.String(), "json result unmatched\n%s", buffer.String())

	expect, err = os.ReadFile(filepath.Join("test", "compat_bar.js"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	expectStr = string(expect)
	if !StringsEq(t, expectStr, fsmock.buff.String()) {
		require.Equal(t, expectStr, fsmock.buff.String(), "js result unmatched\n%s", fsmock.buff.String())
	}
}

func TestLine3DCompat(t *testing.T) {
	buffer := &bytes.Buffer{}
	fsmock := &VolatileFileWriterMock{}

	line := chart.NewRectChart("line3D")
	opts := []opts.Option{
		opts.OutputStream(stream.NewOutputStreamWriter(buffer)),
		opts.VolatileFileWriter(fsmock),
		opts.ChartId("zmsXewYeZOqW"),
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

	expect, err := os.ReadFile(filepath.Join("test", fmt.Sprintf("compat_line3d.%s", "json")))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	expectStr := string(expect)
	require.JSONEq(t, expectStr, buffer.String(), "json result unmatched\n%s", buffer.String())

	expect, err = os.ReadFile(filepath.Join("test", "compat_line3d.js"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	expectStr = string(expect)
	if !StringsEq(t, expectStr, fsmock.buff.String()) {
		require.Equal(t, expectStr, fsmock.buff.String(), "js result unmatched\n%s", fsmock.buff.String())
	}
}
