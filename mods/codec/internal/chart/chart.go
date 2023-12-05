package chart

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/machbase/neo-server/mods/codec/logger"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
)

const (
	LINE    string = "line"
	BAR     string = "bar"
	SCATTER string = "scatter"
)

type ChartBase struct {
	logger logger.Logger
	output spec.OutputStream

	toJsonOutput bool

	globalOptions ChartGlobalOptions
	multiSeries   map[int]*charts.SingleSeries

	domainColumnIdx  int
	domainColumnType string
	domainColumnData []any
	defaultChartType string

	markAreaXAxis []*MarkAreaXAxis
	markAreaYAxis []*MarkAreaYAxis
	markLineXAxis []*MarkLineXAxis
	markLineYAxis []*MarkLineYAxis

	charts.XYAxis

	onceInit sync.Once

	simpleColNames []string
}

type ChartGlobalOptions struct {
	PageTitle       string   `json:"pageTitle" default:"chart"` // HTML title
	Width           string   `json:"width" default:"600px"`     // Width of canvas
	Height          string   `json:"height" default:"600px"`    // Height of canvas
	BackgroundColor string   `json:"backgroundColor"`           // BackgroundColor of canvas
	ChartID         string   `json:"chartId"`                   // Chart unique ID
	AssetsHost      string   `json:"assetsHost" default:"https://go-echarts.github.io/go-echarts-assets/assets/"`
	Theme           string   `json:"theme" default:""`
	Colors          []string `json:"color"`
	Animation       bool     `json:"animation" default:"false"`

	DataZoomList  []opts.DataZoom  `json:"datazoom,omitempty"`
	VisualMapList []opts.VisualMap `json:"visualmap,omitempty"`
	GridList      []opts.Grid      `json:"grid,omitempty"`

	opts.Legend       `json:"legend"`
	opts.Tooltip      `json:"tooltip"`
	opts.Toolbox      `json:"toolbox"`
	opts.Title        `json:"title"`
	opts.Polar        `json:"polar"`
	opts.AngleAxis    `json:"angleAxis"`
	opts.RadiusAxis   `json:"radiusAxis"`
	opts.Brush        `json:"brush"`
	*opts.AxisPointer `json:"axisPointer"`

	opts.Grid3D `json:"grid3D"`
}

func NewRectChart() *ChartBase {
	ret := &ChartBase{}
	ret.defaultChartType = "line"
	ret.initialize()
	return ret
}

func (ex *ChartBase) ContentType() string {
	if ex.toJsonOutput {
		return "application/json"
	}
	return "text/html"
}

func (ex *ChartBase) SetLogger(l logger.Logger) {
	ex.logger = l
}

func (ex *ChartBase) SetOutputStream(o spec.OutputStream) {
	ex.output = o
}

func (ex *ChartBase) SetSize(width, height string) {
	ex.globalOptions.Width = width
	ex.globalOptions.Height = height
}

func (ex *ChartBase) SetTheme(theme string) {
	ex.globalOptions.Theme = theme
}

func (ex *ChartBase) SetTitle(title string) {
	ex.globalOptions.Title.Title = title
}

func (ex *ChartBase) SetSubtitle(subtitle string) {
	ex.globalOptions.Title.Subtitle = subtitle
}

func (ex *ChartBase) SetAssetHost(path string) {
	ex.globalOptions.AssetsHost = path
}

func (ex *ChartBase) SetToolboxSaveAsImage(name string) {
	if ex.globalOptions.Toolbox.Feature == nil {
		ex.globalOptions.Toolbox.Feature = &opts.ToolBoxFeature{}
	}
	if ex.globalOptions.Toolbox.Feature.SaveAsImage == nil {
		ex.globalOptions.Toolbox.Feature.SaveAsImage = &opts.ToolBoxFeatureSaveAsImage{}
	}
	typ := "png"
	if strings.HasSuffix(name, ".jpeg") {
		typ = "jpeg"
		name = strings.TrimSuffix(name, ".jpeg")
	} else if strings.HasSuffix(name, ".svg") {
		typ = "svg"
		name = strings.TrimSuffix(name, ".svg")
	}
	ex.globalOptions.Toolbox.Show = true
	ex.globalOptions.Toolbox.Feature.SaveAsImage.Show = true
	ex.globalOptions.Toolbox.Feature.SaveAsImage.Name = name
	ex.globalOptions.Toolbox.Feature.SaveAsImage.Type = typ
	ex.globalOptions.Toolbox.Feature.SaveAsImage.Title = "save"
}

func (ex *ChartBase) SetToolboxDataZoom() {
	if ex.globalOptions.Toolbox.Feature == nil {
		ex.globalOptions.Toolbox.Feature = &opts.ToolBoxFeature{}
	}
	if ex.globalOptions.Toolbox.Feature.DataZoom == nil {
		ex.globalOptions.Toolbox.Feature.DataZoom = &opts.ToolBoxFeatureDataZoom{}
	}
	ex.globalOptions.Toolbox.Show = true
	ex.globalOptions.Toolbox.Feature.DataZoom.Show = true
	ex.globalOptions.Toolbox.Feature.DataZoom.Title = map[string]string{"zoom": "zoom", "back": "back"}
}

func (ex *ChartBase) SetToolboxDataView() {
	if ex.globalOptions.Toolbox.Feature == nil {
		ex.globalOptions.Toolbox.Feature = &opts.ToolBoxFeature{}
	}
	if ex.globalOptions.Toolbox.Feature.DataView == nil {
		ex.globalOptions.Toolbox.Feature.DataView = &opts.ToolBoxFeatureDataView{}
	}
	ex.globalOptions.Toolbox.Show = true
	ex.globalOptions.Toolbox.Feature.DataView.Show = true
	ex.globalOptions.Toolbox.Feature.DataView.Title = "view"
	ex.globalOptions.Toolbox.Feature.DataView.Lang = []string{"Data", "Close", "Refresh"}
}

func (ex *ChartBase) SetDataZoom(typ string, start float32, end float32) {
	ex.globalOptions.DataZoomList = append(ex.globalOptions.DataZoomList,
		opts.DataZoom{
			Type:  typ,   // "inside", "slider"
			Start: start, // 0 ~ 100 %
			End:   end,   // 0 ~ 100 %
		})
}

func (ex *ChartBase) SetVisualMap(min float64, max float64) {
	colors := []string{
		"#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
		"#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026",
	}
	ex.SetVisualMapColor(min, max, colors...)
}

func (ex *ChartBase) SetVisualMapColor(min float64, max float64, colors ...string) {
	opt := opts.VisualMap{}
	util.SetDefaultValue(&opt)
	opt.Min = float32(min)
	opt.Max = float32(max)
	opt.Calculable = true
	opt.InRange = &opts.VisualMapInRange{
		Color: colors,
	}
	ex.globalOptions.VisualMapList = append(ex.globalOptions.VisualMapList, opt)
}

func (ex *ChartBase) SetChartJson(flag bool) {
	ex.toJsonOutput = flag
	if ex.globalOptions.Theme == "" {
		if flag {
			ex.globalOptions.Theme = "-" // client choose 'white' or 'dark'
		} else {
			ex.globalOptions.Theme = "white" // echarts default
		}
	}
}

func (ex *ChartBase) initialize() {
	ex.onceInit.Do(func() {
		util.SetDefaultValue(&ex.globalOptions)
		ex.globalOptions.AxisPointer = &opts.AxisPointer{
			Link:  []opts.AxisPointerLink{{XAxisIndex: []int{0}, YAxisIndex: []int{0}}},
			Label: &opts.Label{BackgroundColor: "#777"},
		}
		ex.globalOptions.Colors = []string{
			"#5470c6", "#91cc75", "#fac858", "#ee6666", "#73c0de",
			"#3ba272", "#fc8452", "#9a60b4", "#ea7ccc",
		}
		ex.globalOptions.Tooltip.Show = true
		ex.globalOptions.Tooltip.Trigger = "axis"
		ex.globalOptions.Tooltip.AxisPointer = &opts.AxisPointer{Type: "cross"}
	})
}

func (ex *ChartBase) SetGlobal(content string) {
	if !strings.HasPrefix(content, "{") {
		content = "{" + content + "}"
	}
	if err := json.Unmarshal([]byte(content), &ex.globalOptions); err != nil {
		if ex.logger != nil {
			ex.logger.LogWarn("invalid syntax of global(),", err.Error())
		}
		return
	}
}

func (ex *ChartBase) getGlobalOptions() []charts.GlobalOpts {
	if ex.globalOptions.Theme == "" {
		ex.globalOptions.Theme = "white"
	}
	ret := []charts.GlobalOpts{
		func(bc *charts.BaseConfiguration) {
			bc.Initialization = opts.Initialization{
				PageTitle:       ex.globalOptions.PageTitle,
				Width:           ex.globalOptions.Width,
				Height:          ex.globalOptions.Height,
				BackgroundColor: ex.globalOptions.BackgroundColor,
				ChartID:         ex.globalOptions.ChartID,
				AssetsHost:      ex.globalOptions.AssetsHost,
				Theme:           ex.globalOptions.Theme,
			}
			if bc.Initialization.Theme != "" &&
				bc.Initialization.Theme != "white" &&
				bc.Initialization.Theme != "dark" {
				bc.JSAssets.Add("themes/" + ex.globalOptions.Theme + ".js")
			}
			bc.Initialization.Validate()
		},
		func(bc *charts.BaseConfiguration) {
			bc.Title = ex.globalOptions.Title
			bc.Legend = ex.globalOptions.Legend
			bc.Tooltip = ex.globalOptions.Tooltip
			bc.Toolbox = ex.globalOptions.Toolbox
			bc.Title = ex.globalOptions.Title
			bc.Polar = ex.globalOptions.Polar
			bc.AngleAxis = ex.globalOptions.AngleAxis
			bc.RadiusAxis = ex.globalOptions.RadiusAxis
			bc.Brush = ex.globalOptions.Brush

			bc.Animation = ex.globalOptions.Animation
			bc.Colors = ex.globalOptions.Colors
			bc.XYAxis = ex.XYAxis
			bc.DataZoomList = ex.globalOptions.DataZoomList
			bc.VisualMapList = ex.globalOptions.VisualMapList
			bc.GridList = ex.globalOptions.GridList
			bc.Grid3D = ex.globalOptions.Grid3D
		},
	}
	return ret
}

func (ex *ChartBase) SetXAxis(args ...any) {
	for _, arg := range args {
		content, ok := arg.(string)
		if !ok {
			if ex.logger != nil {
				ex.logger.LogError("xAxis(string...) syntax error, xAxis(json) ")
			}
			continue
		}

		xaxis := opts.XAxis{
			Name: "x",
			Show: true,
			SplitLine: &opts.SplitLine{
				Show: true,
				LineStyle: &opts.LineStyle{
					Width:   0.8,
					Opacity: 0.3,
				},
			},
		}

		if !strings.HasPrefix(content, "{") {
			content = "{" + content + "}"
		}
		if err := json.Unmarshal([]byte(content), &xaxis); err != nil {
			if ex.logger != nil {
				ex.logger.LogWarn("xAxis()", err.Error())
				return
			}
		}
		ex.XYAxis.ExtendXAxis(xaxis)
	}
}

func (ex *ChartBase) SetYAxis(args ...any) {
	for _, arg := range args {
		content, ok := arg.(string)
		if !ok {
			if ex.logger != nil {
				ex.logger.LogError("yAxis(string...) syntax error, yAxis(json) ")
			}
			continue
		}

		yaxis := opts.YAxis{
			Name: "y",
			Show: true,
			SplitLine: &opts.SplitLine{
				Show: true,
				LineStyle: &opts.LineStyle{
					Width:   0.8,
					Opacity: 0.3,
				},
			},
		}
		if !strings.HasPrefix(content, "{") {
			content = "{" + content + "}"
		}
		if err := json.Unmarshal([]byte(content), &yaxis); err != nil {
			if ex.logger != nil {
				ex.logger.LogWarn("yAxis()", err.Error())
				return
			}
		}
		ex.XYAxis.ExtendYAxis(yaxis)
	}
}

func (ex *ChartBase) Open() error {
	return nil
}

func (ex *ChartBase) Flush(heading bool) {
}

func (ex *ChartBase) initMarkLines(ser *charts.SingleSeries) {
	if ser.MarkLines != nil {
		return
	}
	ser.MarkLines = &opts.MarkLines{
		MarkLineStyle: opts.MarkLineStyle{
			Symbol: []string{"none", "none"},
		},
	}
}

func (ex *ChartBase) initMarkAreas(ser *charts.SingleSeries) {
	if ser.MarkAreas != nil {
		return
	}
	ser.MarkAreas = &opts.MarkAreas{}
}

func (ex *ChartBase) writeMultiSeries(chart *charts.BaseConfiguration) {
	for seriesIdx := 0; seriesIdx < len(ex.multiSeries); seriesIdx++ {
		ser := ex.getSeries(seriesIdx, false)
		if ser == nil {
			continue
		}
		for _, mark := range ex.markAreaXAxis {
			if mark.SeriesIdx != seriesIdx {
				continue
			}
			ex.initMarkAreas(ser)
			ser.MarkAreas.Data = append(ser.MarkAreas.Data, mark)
		}
		for _, mark := range ex.markAreaYAxis {
			if mark.SeriesIdx != seriesIdx {
				continue
			}
			ex.initMarkAreas(ser)
			ser.MarkAreas.Data = append(ser.MarkAreas.Data, mark)
		}
		for _, mark := range ex.markLineXAxis {
			if mark.SeriesIdx != seriesIdx {
				continue
			}
			ex.initMarkLines(ser)
			ser.MarkLines.Data = append(ser.MarkLines.Data, mark)
		}
		for _, mark := range ex.markLineYAxis {
			if mark.SeriesIdx != seriesIdx {
				continue
			}
			ex.initMarkLines(ser)
			ser.MarkLines.Data = append(ser.MarkLines.Data, mark)
		}
		if ser.Type == "" {
			ser.Type = ex.defaultChartType
		}
		if ser.Name == "" {
			ser.Name = ex.getSeriesName(seriesIdx)
		}
		chart.MultiSeries = append(chart.MultiSeries, *ser)
	}
}
func (ex *ChartBase) Close() {
	var before = []func(){}
	var rect any
	if ex.globalOptions.RadiusAxis.Type != "" {
		chart := charts.NewBar()
		chart.EnablePolarType()
		chart.SetGlobalOptions(ex.getGlobalOptions()...)
		if chart.Polar.Center[0] == "" && chart.Polar.Center[1] == "" {
			chart.Polar.Center[0] = "50%"
			chart.Polar.Center[1] = "50%"
		}
		ex.writeMultiSeries(&chart.BaseConfiguration)
		rect = chart
	} else if len(ex.multiSeries) == 1 && ex.multiSeries[0].Type == "pie" {
		chart := charts.NewPie()
		chart.SetGlobalOptions(ex.getGlobalOptions()...)
		ex.writeMultiSeries(&chart.BaseConfiguration)
		rect = chart
	} else if len(ex.multiSeries) > 0 && ex.multiSeries[0].Type == "candlestick" {
		chart := charts.NewKLine()
		chart.SetGlobalOptions(ex.getGlobalOptions()...)
		ex.writeMultiSeries(&chart.BaseConfiguration)
		rect = chart
		if len(chart.XAxisList) == 0 {
			chart.XAxisList = append(chart.XAxisList, opts.XAxis{})
		}
		if len(chart.YAxisList) == 0 {
			chart.YAxisList = append(chart.YAxisList, opts.YAxis{})
		}
		before = []func(){chart.Validate, func() {
			for i := range chart.XAxisList {
				if chart.XAxisList[i].Data == nil {
					chart.XAxisList[i].Type = ex.domainColumnType
					if ex.domainColumnData != nil {
						chart.XAxisList[i].Data = ex.domainColumnData
					}
				}
			}
		}}
	} else {
		chart := charts.NewLine()
		chart.SetGlobalOptions(ex.getGlobalOptions()...)
		ex.writeMultiSeries(&chart.BaseConfiguration)
		rect = chart
		if len(chart.XAxisList) == 0 {
			chart.XAxisList = append(chart.XAxisList, opts.XAxis{})
		}
		if len(chart.YAxisList) == 0 {
			chart.YAxisList = append(chart.YAxisList, opts.YAxis{})
		}
		before = []func(){chart.Validate, func() {
			for i := range chart.XAxisList {
				if chart.XAxisList[i].Data == nil {
					chart.XAxisList[i].Type = ex.domainColumnType
					if ex.domainColumnData != nil {
						chart.XAxisList[i].Data = ex.domainColumnData
					}
				}
			}
		}}
	}

	var rndr Renderer
	if ex.toJsonOutput {
		rndr = newJsonRender(rect, before...)
	} else {
		rndr = newChartRender(rect, before...)
	}
	err := rndr.Render(ex.output)
	if err != nil {
		fmt.Println("ERR", err.Error())
	}
}

func (ex *ChartBase) AddRow(values []any) error {
	xAxisValue := values[ex.domainColumnIdx]
	if ex.domainColumnType == "time" {
		if vv, ok := xAxisValue.(time.Time); ok {
			xAxisValue = vv.UnixMilli()
		} else if vv, ok := xAxisValue.(*time.Time); ok {
			xAxisValue = vv.UnixMilli()
		}
	}
	seriesIdx := -1
	for n, v := range values {
		if n == ex.domainColumnIdx {
			continue
		} else {
			seriesIdx++
		}
		ser := ex.getSeries(seriesIdx, true)
		if vv, ok := v.(time.Time); ok {
			v = vv.UnixMilli()
		} else if vv, ok := v.(*time.Time); ok {
			v = vv.UnixMilli()
		}
		var data []any
		if ser.Data == nil {
			data = []any{}
		} else {
			data = ser.Data.([]any)
		}
		if ser.Type == "pie" {
			data = append(data, ChartData{Name: fmt.Sprintf("%v", xAxisValue), Value: v})
			ser.Data = data
		} else if ser.Type == "candlestick" {
			data = append(data, ChartData{Name: fmt.Sprintf("%v", xAxisValue), Value: v})
			ser.Data = data
			ex.domainColumnData = append(ex.domainColumnData, xAxisValue)
		} else {
			// hint: use ChartData instead of v for customizing a specefic item
			data = append(data, []any{xAxisValue, v})
			ser.Data = data
		}
	}
	return nil
}

type SeriesPeek struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

func (ex *ChartBase) SetSeries(args ...string) {
	if ex.multiSeries == nil {
		ex.multiSeries = map[int]*charts.SingleSeries{}
	}
	seriesIdx := -1
	for idx, content := range args {
		if !strings.HasPrefix(content, "{") {
			content = "{" + content + "}"
		}

		peek := SeriesPeek{}
		if err := json.Unmarshal([]byte(content), &peek); err != nil {
			if ex.logger != nil {
				ex.logger.LogErrorf("series() args[%d], %s", idx, err.Error())
			}
			return
		}
		switch peek.Type {
		case "time", "category", "value", "log":
			ex.domainColumnIdx = idx
			ex.domainColumnType = peek.Type
			continue
		default: // line, bar, scatter, ...
			seriesIdx++
		}

		ser := ex.getSeries(seriesIdx, true)
		err := json.Unmarshal([]byte(content), ser)
		if err != nil {
			if ex.logger != nil {
				ex.logger.LogErrorf("series() args[%d], %s", idx, err.Error())
			}
			return
		}
	}
}

func (ex *ChartBase) getSeries(idx int, createIfNotExists bool) *charts.SingleSeries {
	if ex.multiSeries == nil {
		ex.multiSeries = map[int]*charts.SingleSeries{}
	}
	if createIfNotExists {
		if ss, ok := ex.multiSeries[idx]; ok {
			return ss
		} else {
			ret := &charts.SingleSeries{
				// Line
				Smooth:       true,
				ConnectNulls: false,
				ShowSymbol:   true,
				//Symbol: "", //'circle', 'rect', 'roundRect', 'triangle', 'diamond', 'pin', 'arrow', 'none'

				// Scatter
				SymbolSize: 5,
			}
			ex.multiSeries[idx] = ret
			return ret
		}
	} else {
		return ex.multiSeries[idx]
	}
}

func (ex *ChartBase) SetColumns(cols ...string) {
	ex.simpleColNames = cols
}

func (ex *ChartBase) getSeriesName(idx int) string {
	var label string
	if ex.multiSeries != nil {
		if s, ok := ex.multiSeries[idx]; ok {
			label = s.Name
		}
	}

	if label == "" {
		if len(ex.simpleColNames) > idx+1 {
			label = ex.simpleColNames[idx+1]
		} else {
			label = fmt.Sprintf("column[%d]", idx)
		}
	}
	return label
}

func (ex *ChartBase) SetMarkAreaXAxis(seriesIdx int, from any, to any, args ...string) {
	var item MarkAreaXAxisItem
	if len(args) > 0 {
		content := args[0]
		if !strings.HasPrefix(content, "{") {
			content = "{" + content + "}"
		}
		err := json.Unmarshal([]byte(content), &item)
		if err != nil {
			if ex.logger != nil {
				ex.logger.LogWarnf("SetMarkAreaXAxis(...) %s", err.Error())
			}
			return
		}
	}
	if t, ok := from.(*time.Time); ok {
		from = t.UnixMilli()
	} else if t, ok := from.(time.Time); ok {
		from = t.UnixMilli()
	}
	if t, ok := to.(*time.Time); ok {
		to = t.UnixMilli()
	} else if t, ok := to.(time.Time); ok {
		to = t.UnixMilli()
	}
	item.XAxis = from
	ex.markAreaXAxis = append(ex.markAreaXAxis, &MarkAreaXAxis{
		SeriesIdx: seriesIdx,
		Items:     []MarkAreaXAxisItem{item, {XAxis: to}},
	})
}

func (ex *ChartBase) SetMarkAreaYAxis(seriesIdx int, from any, to any, args ...string) {
	var item MarkAreaYAxisItem
	if len(args) > 0 {
		content := args[0]
		if !strings.HasPrefix(content, "{") {
			content = "{" + content + "}"
		}
		err := json.Unmarshal([]byte(content), &item)
		if err != nil {
			if ex.logger != nil {
				ex.logger.LogWarnf("SetMarkAreaYAxis(...) %s", err.Error())
			}
			return
		}
	}
	item.YAxis = from
	ex.markAreaYAxis = append(ex.markAreaYAxis, &MarkAreaYAxis{
		SeriesIdx: seriesIdx,
		Items:     []MarkAreaYAxisItem{item, {YAxis: to}},
	})
}

func (ex *ChartBase) SetMarkLineXAxis(seriesIdx int, value any, args ...string) {
	var item MarkLineXAxis
	if len(args) > 0 {
		content := args[0]
		if !strings.HasPrefix(content, "{") {
			content = "{" + content + "}"
		}
		err := json.Unmarshal([]byte(content), &item)
		if err != nil {
			if ex.logger != nil {
				ex.logger.LogWarnf("SetMarkLineXAxis(...) %s", err.Error())
			}
			return
		}
	}
	if t, ok := value.(*time.Time); ok {
		value = t.UnixMilli()
	} else if t, ok := value.(time.Time); ok {
		value = t.UnixMilli()
	}
	item.SeriesIdx = seriesIdx
	item.XAxis = value
	ex.markLineXAxis = append(ex.markLineXAxis, &item)
}

func (ex *ChartBase) SetMarkLineYAxis(seriesIdx int, value any, args ...string) {
	var item MarkLineYAxis
	if len(args) > 0 {
		content := args[0]
		if !strings.HasPrefix(content, "{") {
			content = "{" + content + "}"
		}
		err := json.Unmarshal([]byte(content), &item)
		if err != nil {
			if ex.logger != nil {
				ex.logger.LogWarnf("SetMarkLineYAxis(...) %s", err.Error())
			}
			return
		}
	}
	if t, ok := value.(*time.Time); ok {
		value = t.UnixMilli()
	} else if t, ok := value.(time.Time); ok {
		value = t.UnixMilli()
	}
	item.SeriesIdx = seriesIdx
	item.YAxis = value
	ex.markLineYAxis = append(ex.markLineYAxis, &item)
}

func (ex *ChartBase) SetMarkLineStyle(seriesIdx int, content string) {
	style := opts.MarkLineStyle{Symbol: []string{"none", "none"}}
	if !strings.HasPrefix(content, "{") {
		content = "{" + content + "}"
	}
	err := json.Unmarshal([]byte(content), &style)
	if err != nil {
		if ex.logger != nil {
			ex.logger.LogWarnf("markLineXAxisCoord(...) %s", err.Error())
		}
		return
	}
	ser := ex.getSeries(seriesIdx, true)
	if ser.MarkLines == nil {
		ser.MarkLines = &opts.MarkLines{}
	}
	ser.MarkLines.MarkLineStyle = style
}

type MarkAreaXAxis struct {
	SeriesIdx int `json:"-"`
	Items     []MarkAreaXAxisItem
}

type MarkAreaXAxisItem struct {
	Name      string          `json:"name,omitempty"`
	ItemStyle *opts.ItemStyle `json:"itemStyle,omitempty"`
	Label     *opts.Label     `json:"label,omitempty"`
	XAxis     any             `json:"xAxis"`
}

func (m *MarkAreaXAxis) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Items)
}

type MarkAreaYAxis struct {
	SeriesIdx int `json:"-"`
	Items     []MarkAreaYAxisItem
}

type MarkAreaYAxisItem struct {
	Name      string          `json:"name,omitempty"`
	ItemStyle *opts.ItemStyle `json:"itemStyle,omitempty"`
	Label     *opts.Label     `json:"label,omitempty"`
	YAxis     any             `json:"yAxis"`
}

func (m *MarkAreaYAxis) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Items)
}

type MarkLineXAxis struct {
	SeriesIdx int             `json:"-"`
	Name      string          `json:"name,omitempty"`
	ItemStyle *opts.ItemStyle `json:"itemStyle,omitempty"`
	Label     *opts.Label     `json:"label,omitempty"`
	XAxis     any             `json:"xAxis"`
}

type MarkLineYAxis struct {
	SeriesIdx int             `json:"-"`
	Name      string          `json:"name,omitempty"`
	ItemStyle *opts.ItemStyle `json:"itemStyle,omitempty"`
	Label     *opts.Label     `json:"label,omitempty"`
	YAxis     any             `json:"yAxis"`
}

type ChartData struct {
	// Name of data item.
	Name string `json:"name,omitempty"`

	// Value of a single data item.
	Value interface{} `json:"value,omitempty"`

	// Symbol of single data.
	// Icon types provided by ECharts includes 'circle', 'rect', 'roundRect', 'triangle', 'diamond', 'pin', 'arrow', 'none'
	// It can be set to an image with 'image://url' , in which URL is the link to an image, or dataURI of an image.
	Symbol string `json:"symbol,omitempty"`

	// single data symbol size. It can be set to single numbers like 10, or
	// use an array to represent width and height. For example, [20, 10] means symbol width is 20, and height is10
	SymbolSize int `json:"symbolSize,omitempty"`

	// SymbolRotate (Scatter only)
	SymbolRotate int `json:"symbolRotate,omitempty"`

	// Index of x axis to combine with, which is useful for multiple x axes in one chart.
	XAxisIndex int `json:"xAxisIndex,omitempty"`

	// Index of y axis to combine with, which is useful for multiple y axes in one chart.
	YAxisIndex int `json:"yAxisIndex,omitempty"`
}
