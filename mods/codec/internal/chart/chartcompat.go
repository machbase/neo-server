package chart

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/machbase/neo-server/v8/mods/util"
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

	legendData []string

	markAreaList []string
	markLineList []string

	gridWHD    [3]float64
	autoRotate float64
	opacity    float64
	lineWidth  float64
}

func NewRectChart(typ string) *ChartW {
	var ret *ChartW = &ChartW{Chart: NewChart(), Type: "line"}
	ret.Chart.isCompatibleMode = true
	ret.xAxisIdx = 0
	ret.xAxisLabel = "x"
	ret.xAxisType = "value"
	ret.yAxisIdx = 1
	ret.yAxisLabel = "y"
	ret.yAxisType = "value"
	ret.zAxisIdx = -1
	ret.zAxisLabel = "z"
	ret.zAxisType = "value"
	ret.gridWHD[0], ret.gridWHD[1] = 100, 100

	switch typ {
	default: // case "line":
		ret.Type = "line"
	case "scatter", "bar":
		ret.Type = typ
	case "line3D", "scatter3D", "bar3D", "surface3D":
		ret.Type = typ
		ret.zAxisIdx = 2
		ret.plugins = []string{"/web/echarts/echarts-gl.min.js"}
		ret.opacity = 1.0
		ret.lineWidth = 1.0
		ret.gridWHD[2] = 100
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

func (w *ChartW) SetSeriesLabels(args ...string) {
	w.legendData = args
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

// xAxis(idx int, label string, type string)
func (w *ChartW) SetXAxis(args ...any) {
	if len(args) != 2 && len(args) != 3 {
		if w.logger != nil {
			w.logger.LogError("xAxis(idx, label [, type]), got %d args", len(args))
		}
		return
	}
	if n, err := util.ToFloat64(args[0]); err != nil {
		if w.logger != nil {
			w.logger.LogError("xAxis() idx", err.Error())
		}
	} else {
		w.xAxisIdx = int(n)
	}
	if label, ok := args[1].(string); !ok {
		if w.logger != nil {
			w.logger.LogError("xAxis() label should be string, got %T", args[1])
		}
	} else {
		w.xAxisLabel = label
	}
	if len(args) == 3 {
		if typ, ok := args[2].(string); !ok {
			if w.logger != nil {
				w.logger.LogError("xAxis() type should be string, got %T", args[1])
			}
		} else {
			w.xAxisType = typ
		}
	}
}

// yAxis(idx int, label string, type string)
func (w *ChartW) SetYAxis(args ...any) {
	if len(args) != 2 && len(args) != 3 {
		if w.logger != nil {
			w.logger.LogError("yAxis(idx, label [, type]), got %d args", len(args))
		}
		return
	}
	if n, err := util.ToFloat64(args[0]); err != nil {
		if w.logger != nil {
			w.logger.LogError("yAxis() idx", err.Error())
		}
	} else {
		w.yAxisIdx = int(n)
	}
	if label, ok := args[1].(string); !ok {
		if w.logger != nil {
			w.logger.LogError("yAxis() label should be string, got %T", args[1])
		}
	} else {
		w.yAxisLabel = label
	}
	if len(args) == 3 {
		if typ, ok := args[2].(string); !ok {
			if w.logger != nil {
				w.logger.LogError("yAxis() type should be string, got %T", args[1])
			}
		} else {
			w.yAxisType = typ
		}
	}
}

// zAxis(idx int, label string, type string)
func (w *ChartW) SetZAxis(args ...any) {
	if len(args) != 2 && len(args) != 3 {
		if w.logger != nil {
			w.logger.LogError("zAxis(idx, label [, type]), got %d args", len(args))
		}
		return
	}
	if n, err := util.ToFloat64(args[0]); err != nil {
		if w.logger != nil {
			w.logger.LogError("zAxis() idx", err.Error())
		}
	} else {
		w.zAxisIdx = int(n)
	}
	if label, ok := args[1].(string); !ok {
		if w.logger != nil {
			w.logger.LogError("zAxis() label should be string, got %T", args[1])
		}
	} else {
		w.zAxisLabel = label
	}
	if len(args) == 3 {
		if typ, ok := args[2].(string); !ok {
			if w.logger != nil {
				w.logger.LogError("zAxis() type should be string, got %T", args[1])
			}
		} else {
			w.zAxisType = typ
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
	for i := 0; i < 3 && i < len(args); i++ {
		c.gridWHD[i] = args[i]
	}
}

func (w *ChartW) SetLineWidth(width float64) {
	w.lineWidth = width
}

func (w *ChartW) SetOpacity(opacity float64) {
	// 3D only
	w.opacity = opacity
}

func (w *ChartW) SetAutoRotate(speed float64) {
	// 3D only
	if speed < 0 {
		speed = 180
	}
	if speed > 180 {
		speed = 180
	}
	w.autoRotate = speed
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

func (w *ChartW) SetMarkLineXAxisCoord(xAxis any, name string) {
	val, _ := json.Marshal(convValue(xAxis))
	l := fmt.Sprintf(`{"name":%q, "xAxis":%s, "label":{"formatter":%q}}`, name, string(val), name)
	w.markLineList = append(w.markLineList, l)
}

func (w *ChartW) SetMarkLineYAxisCoord(yAxis any, name string) {
	val, _ := json.Marshal(convValue(yAxis))
	l := fmt.Sprintf(`{"name":%q, "yAxis":%s, "label":{"formatter":%q}}`, name, string(val), name)
	w.markLineList = append(w.markLineList, l)
}

func (w *ChartW) SetTimeformat(f string) {
	w.timeformat = f
}

func (w *ChartW) Close() {
	switch w.Type {
	default:
		w.Close2D()
	case "line3D", "scatter3D", "surface3D", "bar3D":
		w.Close3D()
	}
	w.Chart.Close()
}

func (w *ChartW) Close3D() {
	grid3D := fmt.Sprintf(`"grid3D":{"boxWidth":%v, "boxHeight":%v, "boxDepth":%v, "viewControl":{"projection": "orthographic", "autoRotate":%t,"speed":%v}},`,
		w.gridWHD[0], w.gridWHD[1], w.gridWHD[2], w.autoRotate != 0, w.autoRotate)

	xAxis := fmt.Sprintf(`"xAxis3D":{"name":%q,"type":%q,"show":true},`, w.xAxisLabel, w.xAxisType)
	yAxis := fmt.Sprintf(`"yAxis3D":{"name":%q,"type":%q,"show":true},`, w.yAxisLabel, w.yAxisType)
	zAxis := fmt.Sprintf(`"zAxis3D":{"name":%q,"type":%q,"show":true},`, w.zAxisLabel, w.zAxisType)

	series := []string{}
	series = append(series, `"series":[`)
	if len(w.Chart.data) > w.xAxisIdx && len(w.Chart.data) > w.yAxisIdx && len(w.Chart.data) > w.zAxisIdx {
		for i := range w.Chart.data {
			if i == w.xAxisIdx || i == w.yAxisIdx {
				continue
			}
			data := []string{}
			for n, d := range w.Chart.data[i] {
				elm := []any{w.Chart.data[w.xAxisIdx][n], w.Chart.data[w.yAxisIdx][n], d}
				marshal, err := json.Marshal(elm)
				if err != nil {
					marshal = []byte(err.Error())
				}
				data = append(data, string(marshal))
			}
			shading := "lambert" // "color", "realistic"
			style := fmt.Sprintf(`"itemStyle":{"opacity":%v}`, w.opacity)
			if w.Type == "line3D" {
				style = fmt.Sprintf(`"lineStyle":{"opacity":%v,"width":%v}`, w.opacity, w.lineWidth)
			}
			series = append(series, fmt.Sprintf(`{"type":%q,"coordinateSystem":"cartesian3D","data":[%s],"shading":%q,%s}`,
				w.Type, strings.Join(data, ","), shading, style))
		}
	}
	series = append(series, `]`)

	lines := []string{}
	lines = append(lines, xAxis, yAxis, zAxis)
	lines = append(lines, grid3D)
	if w.title != "" {
		if w.subtitle != "" {
			lines = append(lines, fmt.Sprintf(`"title":{"text":%q, "subtext":%q},`, w.title, w.subtitle))
		} else {
			lines = append(lines, fmt.Sprintf(`"title":{"text":%q},`, w.title))
		}
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

	lines = append(lines, series...)
	w.Chart.option = "{\n" + strings.Join(lines, "\n") + `}`
}

func (w *ChartW) Close2D() {
	if t, ok := w.typeHint[w.xAxisIdx]; ok && t == "time" {
		w.xAxisType = "time"
	}
	xAxis := fmt.Sprintf(`"xAxis":{"name":%q,"type":%q},`, w.xAxisLabel, w.xAxisType)
	yAxis := fmt.Sprintf(`"yAxis":{"name":%q,"type":%q},`, w.yAxisLabel, w.yAxisType)

	series := []string{}
	series = append(series, `"series":[`)
	seriesIdx := 0
	legendData := []string{}
	for i := range w.Chart.data {
		if i == w.xAxisIdx {
			continue
		}
		allMarkers := ""
		if seriesIdx == 0 {
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
		if seriesIdx != 0 {
			comma = ",\n"
		}
		seriesData := ""
		seriesName := fmt.Sprintf(`"column[%d]"`, i)
		if len(w.legendData) > seriesIdx {
			seriesName = fmt.Sprintf(`%q`, w.legendData[seriesIdx])
		}
		legendData = append(legendData, seriesName)

		data := []string{}
		for n, d := range w.Chart.data[i] {
			elm := []any{w.Chart.data[w.xAxisIdx][n], d}
			marshal, err := json.Marshal(elm)
			if err != nil {
				marshal = []byte(err.Error())
			}
			data = append(data, string(marshal))
		}
		seriesData = fmt.Sprintf(`"type":%q, "name":%s, "data":[%s]`, w.Type, seriesName, strings.Join(data, ","))
		if allMarkers != "" {
			series = append(series, fmt.Sprintf("    %s{\n    %s,\n    %s\n    }", comma, seriesData, allMarkers))
		} else {
			series = append(series, fmt.Sprintf(`    %s{%s}`, comma, seriesData))
		}
		seriesIdx++
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
	if len(legendData) > 0 {
		lines = append(lines, fmt.Sprintf(`"legend":{"show":true,"data":[%s]},`, strings.Join(legendData, ",")))
	}
	if w.dataZoom != "" {
		lines = append(lines, w.dataZoom+",")
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
	lines = append(lines, series...)
	w.Chart.option = "{\n" + strings.Join(lines, "\n") + `}`
}
