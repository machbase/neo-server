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

func TestAnscombeQuatet(t *testing.T) {
	buffer := &bytes.Buffer{}
	c := chart.NewRectChart()
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetChartJson(true)
	c.SetGlobal(`
		"chartId": "WejMYXCGcYNL",
		"theme": "dark",
        "legend": {"show": false},
        "grid": [
            { "left":  "7%", "top": "7%", "width": "38%", "height": "38%" },
            { "right": "7%", "top": "7%", "width": "38%", "height": "38%" },
            { "left":  "7%", "bottom": "7%", "width": "38%", "height": "38%" },
            { "right": "7%", "bottom": "7%", "width": "38%", "height": "38%" }
        ]`)
	c.SetXAxis(
		` "type":"time", "gridIndex": 0, "min": 1701059598000, "max": 1701059614000 `,
		` "type":"time", "gridIndex": 1, "min": 1701059598000, "max": 1701059614000 `,
		` "type":"time", "gridIndex": 2, "min": 1701059598000, "max": 1701059614000 `,
		` "type":"time", "gridIndex": 3, "min": 1701059598000, "max": 1701059614000 `)
	c.SetYAxis(
		` "gridIndex": 0, "min": 0, "max": 15 `,
		` "gridIndex": 1, "min": 0, "max": 15 `,
		` "gridIndex": 2, "min": 0, "max": 15 `,
		` "gridIndex": 3, "min": 0, "max": 15 `)
	c.SetSeries(
		`   "type": "time" `,
		`   "name": "I",
            "type": "scatter",
            "xAxisIndex": 0,
            "yAxisIndex": 0,
            "markLine": {
                "data": [
                    [ {"coord": [1701059598000, 3]}, {"coord": [1701059614000, 13]} ]
                ]
            }
        `,
		`   "name": "II",
            "type": "scatter",
            "xAxisIndex": 1,
            "yAxisIndex": 1,
            "markLine": {
                "data": [
                    [ {"coord": [1701059598000, 3]}, {"coord": [1701059614000, 13]} ]
                ]
            }
        `,
		`
            "name": "III",
            "type": "scatter",
            "xAxisIndex": 2,
            "yAxisIndex": 2,
            "markLine": {
                "data": [
                    [ {"coord": [1701059598000, 3]}, {"coord": [1701059614000, 13]} ]
                ]
            }
        `,
		`
            "name": "IV",
            "type": "scatter",
            "xAxisIndex": 3,
            "yAxisIndex": 3,
            "markLine": {
                "data": [
                    [
                        {"coord": [1701059598000, 3]},
                        {"coord": [1701059614000, 13]}
                    ]
                ]
            }
        `)

	require.Equal(t, "application/json", c.ContentType())

	c.Open()
	c.AddRow([]any{1701059601000000000, 4.26, 3.1, 5.39, 12.5})
	c.AddRow([]any{1701059602000000000, 5.68, 4.74, 5.73, 6.89})
	c.AddRow([]any{1701059603000000000, 7.24, 6.13, 6.08, 5.25})
	c.AddRow([]any{1701059604000000000, 4.82, 7.26, 6.42, 7.91})
	c.AddRow([]any{1701059605000000000, 6.95, 8.14, 6.77, 5.76})
	c.AddRow([]any{1701059606000000000, 8.81, 8.77, 7.11, 8.84})
	c.AddRow([]any{1701059607000000000, 8.04, 9.14, 7.46, 6.58})
	c.AddRow([]any{1701059608000000000, 8.33, 9.26, 7.81, 8.47})
	c.AddRow([]any{1701059609000000000, 10.84, 9.13, 8.15, 5.56})
	c.AddRow([]any{1701059610000000000, 7.58, 8.74, 12.74, 7.71})
	c.AddRow([]any{1701059611000000000, 9.96, 8.1, 8.84, 7.04})
	c.Close()

	expect, err := os.ReadFile(filepath.Join("test", "anscombe_quartet.json"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	require.JSONEq(t, string(expect), buffer.String(), "json result unmatched\n%s", buffer.String())
}
