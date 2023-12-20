package chart

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ChartW struct {
	*Chart
	Type string

	globalOption string
	visualMap    string
	title        string
	subtitle     string
	dataZoom     string
	xAxisIdx     int
	xAxisLabel   string
	xAxisType    string
	yAxisIdx     int
	yAxisLabel   string
	yAxisType    string
	zAxisIdx     int
	zAxisLabel   string
	zAxisType    string
	timeformat   string

	toolboxSaveAsImage string
	toolboxDataZoom    string
	toolboxDataView    string

	markAreaList []string
	markLineList []string

	grid3D string
}

func NewRectChart(typ string) *ChartW {
	var ret *ChartW = &ChartW{Chart: NewChart(), Type: "line"}
	ret.xAxisIdx = 0
	ret.xAxisLabel = "x"
	ret.xAxisType = "time"
	ret.yAxisIdx = 1
	ret.yAxisLabel = "y"
	ret.yAxisType = "value"
	ret.zAxisIdx = -1
	ret.zAxisLabel = "z"
	ret.zAxisType = "value"
	switch typ {
	default: // case "line":
		ret.Type = "line"
	case "scatter", "bar":
		ret.Type = typ
	case "line3d", "scatter3d", "bar3d":
		ret.Type = typ
		ret.zAxisIdx = 2
	}
	return ret
}

func (w *ChartW) SetGlobalOptions(opt string) {
	if strings.HasPrefix(opt, "{") {
		opt = strings.TrimPrefix(opt, "{")
		opt = strings.TrimSuffix(opt, "}")
	}
	w.globalOption = opt
}

func (w *ChartW) SetDataZoom(typ string, minPercentage float32, maxPercentage float32) {
	if typ != "inside" && typ != "slider" {
		typ = "slider"
	}
	w.dataZoom = fmt.Sprintf(`"dataZoom":[{"type":%q, "start":%v, "end":%v}]`, typ, minPercentage, maxPercentage)
}

func (w *ChartW) SetVisualMap(min float64, max float64) {
	colors := []string{
		"#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
		"#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026",
	}
	w.SetVisualMapColor(min, max, colors...)
}

func (w *ChartW) SetVisualMapColor(min float64, max float64, colors ...string) {
	cls := []string{}
	for _, c := range colors {
		cls = append(cls, fmt.Sprintf("%q", c))
	}
	w.visualMap = fmt.Sprintf(`"visualMap":[{"type":"continuous", "calculable":true, "min":%v, "max":%v, "inRange":{"color":[%s]}}]`,
		min, max, strings.Join(cls, ","))
}

func (w *ChartW) SetXAxis(idx int, label string, typ ...string) {
	w.xAxisIdx = idx
	w.xAxisLabel = label
	if len(typ) > 0 {
		if typ[0] == "time" {
			w.xAxisType = "time"
		} else {
			w.xAxisType = "value"
		}
	}
}

func (w *ChartW) SetYAxis(idx int, label string, typ ...string) {
	w.yAxisIdx = idx
	w.yAxisLabel = label
	if len(typ) > 0 {
		if typ[0] == "time" {
			w.yAxisType = "time"
		} else {
			w.yAxisType = "value"
		}
	}
}

func (w *ChartW) SetZAxis(idx int, label string, typ ...string) {
	w.zAxisIdx = idx
	w.zAxisLabel = label
	if len(typ) > 0 {
		if typ[0] == "time" {
			w.zAxisType = "time"
		} else {
			w.zAxisType = "value"
		}
	}
}

func (w *ChartW) SetTitle(str string) {
	w.title = str
}

func (w *ChartW) SetSubtitle(str string) {
	w.subtitle = str
}

func (c *ChartW) SetGridSize(args ...float64) {
	c.grid3D = `"grid3D":{"boxWidth": 100, "boxHeight": 100, "boxDepth": 100}`

	widthHeightDepth := [3]float64{100, 100, 100}
	for i := 0; i < 3 && i < len(args); i++ {
		widthHeightDepth[i] = args[i]
	}
	c.grid3D = fmt.Sprintf(`"grid3D":{"boxWidth": %v, "boxHeight": %v, "boxDepth": %v}`,
		widthHeightDepth[0], widthHeightDepth[1], widthHeightDepth[2])
}

func (w *ChartW) SetOpacity(opacity float64) {
	// 3D only
}

func (w *ChartW) SetAutoRotate(speed float64) {
	// 3D only
	if speed < 0 {
		speed = 0
	}
	if speed > 180 {
		speed = 180
	}
}

func (w *ChartW) SetToolboxSaveAsImage(name string) {
	typ := "png"
	if strings.HasSuffix(name, ".jpeg") {
		typ = "jpeg"
		name = strings.TrimSuffix(name, ".jpeg")
	} else if strings.HasSuffix(name, ".svg") {
		typ = "svg"
		name = strings.TrimSuffix(name, ".svg")
	}
	w.toolboxSaveAsImage = fmt.Sprintf(`"saveAsImage":{"show":true, "type":%q, "name":%q, "title":"save"}`, typ, name)
}

func (w *ChartW) SetToolboxDataZoom() {
	w.toolboxDataZoom = `"dataZoom":{"show":true, "title":{"zoom":"zoom", "back":"back"}}`
}

func (w *ChartW) SetToolboxDataView() {
	w.toolboxDataView = `"dataView":{"show":true, "title":"view", "lang":["Data", "Close", "Refresh"]}`
}

func (w *ChartW) SetMarkAreaNameCoord(from any, to any, label string, color string, opacity float64) {
	fromVal, _ := json.Marshal(convValue(from))
	toVal, _ := json.Marshal(convValue(to))
	l := fmt.Sprintf(`[{"name":%q, "itemStyle":{"color":%q, "opacity":%v}, "xAxis":%v}, {"xAxis":%v}]`,
		label, color, opacity, string(fromVal), string(toVal))
	w.markAreaList = append(w.markAreaList, l)
}

func (w *ChartW) SetMarkLineXAxisCoord(xaxis any, name string) {
	val, _ := json.Marshal(convValue(xaxis))
	l := fmt.Sprintf(`{"name":%q, "xAxis": %s}`, name, string(val))
	w.markLineList = append(w.markLineList, l)
}

func (w *ChartW) SetMarkLineYAxisCoord(yaxis any, name string) {
	val, _ := json.Marshal(convValue(yaxis))
	l := fmt.Sprintf(`{"name":%q, "yAxis": %s}`, name, string(val))
	w.markLineList = append(w.markLineList, l)
}

func (w *ChartW) SetTimeformat(f string) {
	w.timeformat = f
}

func (w *ChartW) Close() {
	xAxis := `"xAxis": {},`
	yAxis := fmt.Sprintf(`"yAxis": {"name":%q, "type": %q },`, w.yAxisLabel, w.yAxisType)
	zAxis := ""
	series := []string{}
	if w.zAxisIdx >= 0 {
		series = append(series, fmt.Sprintf(`"zAxis": { "type": %q },`, w.zAxisType))
	}
	series = append(series, `"series":[`)
	for i := range w.Chart.data {
		if i == w.xAxisIdx {
			if w.xAxisType != "time" {
				xAxis = fmt.Sprintf(`"xAxis": {"name":%q, "type":%q},`, w.xAxisLabel, w.xAxisType)
			} else {
				xAxis = fmt.Sprintf(`"xAxis": {"name":%q, "type":%q, "data":column(%d)},`, w.xAxisLabel, w.xAxisType, w.xAxisIdx)
			}
			continue
		}
		allMarkers := ""
		if len(series) == 1 {
			lines := []string{}
			if len(w.markAreaList) > 0 {
				areaData := []string{}
				areaData = append(areaData, `"markArea":{"data":[`)
				areaData = append(areaData, strings.Join(w.markAreaList, ","))
				areaData = append(areaData, `]}`)
				lines = append(lines, strings.Join(areaData, "\n    "))
			}
			if len(w.markLineList) > 0 {
				lineData := []string{}
				lineData = append(lineData, `"markLine":{"symbol":["none","none"], "data":[`)
				lineData = append(lineData, strings.Join(w.markLineList, ","))
				lineData = append(lineData, `]}`)
				lines = append(lines, strings.Join(lineData, "\n    "))
			}
			if len(lines) > 0 {
				allMarkers = strings.Join(lines, ",")
			}
		}
		comma := ""
		if i == 0 {
			comma = ",\n"
		}
		seriesData := ""
		if w.xAxisType == "time" {
			data := []string{}
			for n, d := range w.Chart.data[i] {
				elm := []any{w.Chart.data[w.xAxisIdx][n], d}
				marshal, err := json.Marshal(elm)
				if err != nil {
					marshal = []byte(err.Error())
				}
				data = append(data, string(marshal))
			}
			seriesData = fmt.Sprintf(`"type": %q, "data": [%s]`, w.Type, strings.Join(data, ","))
		} else {
			seriesData = fmt.Sprintf(`"type": %q, "data": column(%d)`, w.Type, i)
		}
		if allMarkers != "" {
			series = append(series, fmt.Sprintf("    %s{\n    %s,\n    %s\n    }", comma, seriesData, allMarkers))
		} else {
			series = append(series, fmt.Sprintf(`    %s{%s}`, comma, seriesData))
		}
	}
	series = append(series, `]`)

	lines := []string{}
	if w.title != "" {
		if w.subtitle != "" {
			lines = append(lines, fmt.Sprintf(`"title":{"text":%q, "subtext":%q},`, w.title, w.subtitle))
		} else {
			lines = append(lines, fmt.Sprintf(`"title":{"text":%q},`, w.title))
		}
	}
	if w.globalOption != "" {
		lines = append(lines, w.globalOption+",")
	}
	if w.dataZoom != "" {
		lines = append(lines, w.dataZoom+",")
	}
	if w.grid3D != "" {
		lines = append(lines, w.grid3D+",")
	}
	if w.visualMap != "" {
		lines = append(lines, w.visualMap+",")
	}
	if w.toolboxSaveAsImage != "" || w.toolboxDataZoom != "" || w.toolboxDataView != "" {
		lines = append(lines, `"toolbox":{ "feature":{`)
		features := []string{}
		if w.toolboxSaveAsImage != "" {
			features = append(features, "    "+w.toolboxSaveAsImage)
		}
		if w.toolboxDataZoom != "" {
			features = append(features, "    "+w.toolboxDataZoom)
		}
		if w.toolboxDataView != "" {
			features = append(features, "    "+w.toolboxDataView)
		}
		lines = append(lines, strings.Join(features, ",\n"))
		lines = append(lines, `}},`)
	}
	lines = append(lines, `"tooltip":{"show":true, "trigger":"axis"},`)
	lines = append(lines, xAxis, yAxis)
	if zAxis != "" {
		lines = append(lines, zAxis)
	}
	lines = append(lines, series...)
	w.Chart.option = "{\n" + strings.Join(lines, "\n") + `}`
	w.Chart.Close()
}
