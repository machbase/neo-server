package chart_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/chart"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func HTMLEq(t *testing.T, expect string, actual string) bool {
	matched := false
	t.Helper()
	re := strings.Split(actual, "\n")
	ex := strings.Split(expect, "\n")
	if len(re) == len(ex) {
		for i := range re {
			if strings.TrimSpace(re[i]) != strings.TrimSpace(ex[i]) {
				t.Logf("Expect: %s", strings.TrimSpace(ex[i]))
				t.Logf("Actual: %s", strings.TrimSpace(re[i]))
				goto mismatched
			}
		}
		matched = true
	}
mismatched:
	return matched
}

func TestLine(t *testing.T) {
	for _, output := range []string{"json", "html"} {
		buffer := &bytes.Buffer{}
		c := chart.NewChart()
		c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
		c.SetChartJson(output == "json")
		c.SetChartId("WejMYXCGcYNL")
		c.SetTheme("white")
		c.SetChartOption(`{
			"xAxis": { "type": "time", "data": value(0 ) },
			"yAxis": { "type": "value"},
			"series": [
				{ "type": "line", "data": value( 1) }
			]
		}`)
		if output == "json" {
			require.Equal(t, "application/json", c.ContentType())
		} else {
			require.Equal(t, "text/html", c.ContentType())
		}

		tick := time.Unix(0, 1692670838086467000)

		c.Open()
		c.AddRow([]any{tick.Add(0 * time.Second), 0.0})
		c.AddRow([]any{tick.Add(1 * time.Second), 1.0})
		c.AddRow([]any{tick.Add(2 * time.Second), 2.0})
		c.Close()

		expect, err := os.ReadFile(filepath.Join("test", fmt.Sprintf("test_line.%s", output)))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr := string(expect)
		if output == "json" {
			require.JSONEq(t, expectStr, buffer.String(), "json result unmatched\n%s", buffer.String())
		} else {
			if !HTMLEq(t, expectStr, buffer.String()) {
				require.Equal(t, expectStr, buffer.String(), "html result unmatched\n%s", buffer.String())
			}
		}
	}
}

func TestScatter(t *testing.T) {
	for _, output := range []string{"json", "html"} {
		buffer := &bytes.Buffer{}
		c := chart.NewChart()
		c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
		c.SetChartJson(output == "json")
		c.ChartID = "WejMYXCGcYNL"
		c.Theme = "white"
		c.SetChartOption(`{
			"xAxis": { "type": "time", "data": value(0) },
			"yAxis": { "type": "value"},
			"series": [
				{ "type": "scatter", "data": value( 1	) }
			]
		}`)
		if output == "json" {
			require.Equal(t, "application/json", c.ContentType())
		} else {
			require.Equal(t, "text/html", c.ContentType())
		}

		tick := time.Unix(0, 1692670838086467000)

		c.Open()
		c.AddRow([]any{tick.Add(0 * time.Second), 0.0})
		c.AddRow([]any{tick.Add(1 * time.Second), 1.0})
		c.AddRow([]any{tick.Add(2 * time.Second), 2.0})
		c.Close()

		expect, err := os.ReadFile(filepath.Join("test", fmt.Sprintf("test_scatter.%s", output)))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr := string(expect)
		if output == "json" {
			require.JSONEq(t, expectStr, buffer.String(), "json result unmatched\n%s", buffer.String())
		} else {
			if !HTMLEq(t, expectStr, buffer.String()) {
				require.Equal(t, expectStr, buffer.String(), "html result unmatched\n%s", buffer.String())
			}
		}
	}
}

func TestTangentialPolarBar(t *testing.T) {
	for _, output := range []string{"json", "html"} {
		buffer := &bytes.Buffer{}
		c := chart.NewChart()
		c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
		c.SetChartJson(output == "json")
		c.ChartID = "WejMYXCGcYNL"
		c.Theme = "dark"
		c.SetChartOption(`{
			"polar": { "radius": ["30", "80%"] },
			"angleAxis": { "max": 4, "startAngle": 75 },
			"radiusAxis": {
				"type": "category",
				"data": value(0)
			},
			"tooltip": {},
			"series": [
				{
					"type":"bar",
					"data": value(1),
					"coordinateSystem": "polar",
					"label": {
						"show": true,
						"position": "middle"
					}	
				}
			]
		}`)
		if output == "json" {
			require.Equal(t, "application/json", c.ContentType())
		} else {
			require.Equal(t, "text/html", c.ContentType())
		}

		c.Open()
		c.AddRow([]any{"a", 2.0})
		c.AddRow([]any{"b", 1.2})
		c.AddRow([]any{"c", 2.4})
		c.AddRow([]any{"d", 3.6})
		c.Close()

		expect, err := os.ReadFile(filepath.Join("test", fmt.Sprintf("tangential_polar_bar.%s", output)))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr := string(expect)
		if output == "json" {
			require.JSONEq(t, expectStr, buffer.String(), "json result unmatched\n%s", buffer.String())
		} else {
			if !HTMLEq(t, expectStr, buffer.String()) {
				require.Equal(t, expectStr, buffer.String(), "html result unmatched\n%s", buffer.String())
			}
		}
	}
}

func TestAnscombeQuatet(t *testing.T) {
	buffer := &bytes.Buffer{}
	c := chart.NewChart()
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetChartJson(true)
	c.ChartID = "WejMYXCGcYNL"
	c.Theme = "dark"
	c.SetChartOption(`{
		"legend": {"show": false},
        "grid": [
            { "left":  "7%", "top": "7%", "width": "38%", "height": "38%" },
            { "right": "7%", "top": "7%", "width": "38%", "height": "38%" },
            { "left":  "7%", "bottom": "7%", "width": "38%", "height": "38%" },
            { "right": "7%", "bottom": "7%", "width": "38%", "height": "38%" }
        ],
		"xAxis": [
			{ "type": "time", "gridIndex": 0, "data": value(0), "min": 1701059598000, "max": 1701059614000 },
			{ "type": "time", "gridIndex": 1, "data": value(0), "min": 1701059598000, "max": 1701059614000 },
			{ "type": "time", "gridIndex": 2, "data": value(0), "min": 1701059598000, "max": 1701059614000 },
			{ "type": "time", "gridIndex": 3, "data": value(0), "min": 1701059598000, "max": 1701059614000 }
		],
		"yAxis": [
			{ "type": "value", "gridIndex": 0, "min": 0, "max": 15 },
			{ "type": "value", "gridIndex": 1, "min": 0, "max": 15 },
			{ "type": "value", "gridIndex": 2, "min": 0, "max": 15 },
			{ "type": "value", "gridIndex": 3, "min": 0, "max": 15 }
		],
		"series": [
			{
				"name": "I",
				"type": "scatter",
				"xAxisIndex": 0,
				"yAxisIndex": 0,
				"data": value(1),
				"markLine": {
					"data": [
						[ {"coord": [1701059598000, 3]}, {"coord": [1701059614000, 13]} ]
					]
				}
			},
			{
				"name": "II",
				"type": "scatter",
				"xAxisIndex": 1,
				"yAxisIndex": 1,
				"data": value(2),
				"markLine": {
					"data": [
						[ {"coord": [1701059598000, 3]}, {"coord": [1701059614000, 13]} ]
					]
				}	
			},
			{
				"name": "III",
				"type": "scatter",
				"xAxisIndex": 2,
				"yAxisIndex": 2,
				"data": value(3),
				"markLine": {
					"data": [
						[ {"coord": [1701059598000, 3]}, {"coord": [1701059614000, 13]} ]
					]
				}	
			},
			{
				"name": "IV",
				"type": "scatter",
				"xAxisIndex": 3,
				"yAxisIndex": 3,
				"data": value(4),
				"markLine": {
					"data": [
						[
							{"coord": [1701059598000, 3]},
							{"coord": [1701059614000, 13]}
						]
					]
				}	
			}
		]
	}`)
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

func TestMarkLine(t *testing.T) {
	buffer := &bytes.Buffer{}
	c := chart.NewChart()
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetChartJson(true)
	c.ChartID = "WejMYXCGcYNL"
	c.Theme = "dark"
	c.SetChartOption(`{
		"xAxis": { "type": "time", "data": value(0) },
		"yAxis": { "type": "value" },
		"series": [{ 
			"type": "line",
			"data": value(1),
			"color":"#5470C6",
			"markLine": [
				{
					"data": { "name": "xmark", "xAxis": 1701059605 },
					"itemStyle": {
						"color":"#ff0"
					}
				},
				{
					"data": { "name": "ymark", "yAxis": 6.0 },
					"itemStyle": {
						"color":"#ff0"
					}
				}
			]
		}]
	}`)

	require.Equal(t, "application/json", c.ContentType())

	c.Open()
	c.AddRow([]any{1701059601000000000, 4.26})
	c.AddRow([]any{1701059602000000000, 5.68})
	c.AddRow([]any{1701059603000000000, 7.24})
	c.AddRow([]any{1701059604000000000, 4.82})
	c.AddRow([]any{1701059605000000000, 6.95})
	c.AddRow([]any{1701059606000000000, 8.81})
	c.AddRow([]any{1701059607000000000, 8.04})
	c.AddRow([]any{1701059608000000000, 8.33})
	c.AddRow([]any{1701059609000000000, 10.84})
	c.AddRow([]any{1701059610000000000, 7.58})
	c.AddRow([]any{1701059611000000000, 9.96})
	c.Close()

	expect, err := os.ReadFile(filepath.Join("test", "mark_line.json"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	require.JSONEq(t, string(expect), buffer.String(), "json result unmatched\n%s", buffer.String())
}

func TestCandleStick(t *testing.T) {
	buffer := &bytes.Buffer{}
	c := chart.NewChart()
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetChartJson(true)
	c.ChartID = "WejMYXCGcYNL"
	c.Theme = "dark"
	c.SetChartOption(`{
		"legend":{"show": false},
		"series":[
			{
				"type": "category",
				"data": value(0)
			},
			{
				"type": "candlestick",
				"data": value(1)
			}
		]
	}`)
	require.Equal(t, "application/json", c.ContentType())

	c.Open()
	c.AddRow([]any{1508806800_000000000, []any{20, 34, 10, 38}})
	c.AddRow([]any{1508893200_000000000, []any{40, 35, 30, 50}})
	c.AddRow([]any{1508979600_000000000, []any{31, 38, 33, 44}})
	c.AddRow([]any{1509066000_000000000, []any{38, 15, 5, 42}})
	c.Close()

	expect, err := os.ReadFile(filepath.Join("test", "test_candlestick.json"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	require.JSONEq(t, string(expect), buffer.String(), "json result unmatched\n%s", buffer.String())
}
