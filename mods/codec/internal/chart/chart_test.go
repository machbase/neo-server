package chart_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/chart"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func TestLine(t *testing.T) {
	buffer := &bytes.Buffer{}

	c := chart.NewRectChart()
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetChartJson(true)
	c.SetGlobal(`
		"chartId": "WejMYXCGcYNL",
		"theme": "white"
	`)
	c.SetSeries(`
		{ "type": "time" },
		{ "type": "line" }
	`)
	require.Equal(t, "application/json", c.ContentType())

	c.Open()
	tick := time.Unix(0, 1692670838086467000)
	c.AddRow([]any{tick.Add(0 * time.Second), 0.0})
	c.AddRow([]any{tick.Add(1 * time.Second), 1.0})
	c.AddRow([]any{tick.Add(2 * time.Second), 2.0})
	c.Close()

	expect, err := os.ReadFile(filepath.Join("test", "test_line.json"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	require.JSONEq(t, string(expect), buffer.String(), "json result unmatched", buffer.String())
}

func TestScatter(t *testing.T) {
	buffer := &bytes.Buffer{}

	c := chart.NewRectChart()
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetChartJson(true)
	c.SetGlobal(`
		"chartId": "WejMYXCGcYNL",
		"theme": "white"
	`)
	c.SetSeries(
		`{ "type": "time" }`,
		`{ "type": "scatter" }`,
	)
	require.Equal(t, "application/json", c.ContentType())

	c.Open()
	tick := time.Unix(0, 1692670838086467000)
	c.AddRow([]any{tick.Add(0 * time.Second), 0.0})
	c.AddRow([]any{tick.Add(1 * time.Second), 1.0})
	c.AddRow([]any{tick.Add(2 * time.Second), 2.0})
	c.Close()

	expect, err := os.ReadFile(filepath.Join("test", "test_scatter.json"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	require.JSONEq(t, string(expect), buffer.String(), "json result unmatched", buffer.String())
}

func TestTangentialPolarBar(t *testing.T) {
	buffer := &bytes.Buffer{}

	c := chart.NewRectChart()
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetChartJson(true)
	c.SetGlobal(`
		"chartId": "WejMYXCGcYNL",
		"theme": "dark",
        "polar": {"max": 4, "startAngle": 75},
        "radiusAxis": {
            "type": "category",
            "data": ["a", "b", "c", "d"]
        }
	`)
	c.SetSeries(
		`{	"type": "category"}`,
		`{	"type": "bar",
			"coordinateSystem": "polar",
            "label": {
				"show": true,
				"position": "middle"
			}
        }`,
	)
	require.Equal(t, "application/json", c.ContentType())

	c.Open()
	c.AddRow([]any{"a", 2.0})
	c.AddRow([]any{"b", 1.2})
	c.AddRow([]any{"c", 2.4})
	c.AddRow([]any{"d", 3.6})
	c.Close()

	expect, err := os.ReadFile(filepath.Join("test", "tangential_polar_bar.json"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	require.JSONEq(t, string(expect), buffer.String(), "json result unmatched\n%s", buffer.String())
}
