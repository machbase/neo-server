package chart

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

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

	charts.XYAxis
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
		ex.globalOptions.XYAxis.ExtendXAxis(opts.XAxis{Name: "x", Show: true, SplitLine: &opts.SplitLine{Show: true, LineStyle: &opts.LineStyle{Width: 0.8, Opacity: 0.3}}})
		ex.globalOptions.XYAxis.ExtendYAxis(opts.YAxis{Name: "y", Show: true, SplitLine: &opts.SplitLine{Show: true, LineStyle: &opts.LineStyle{Width: 0.8, Opacity: 0.3}}})
	})
}

func (ex *ChartBase) SetGlobal(content string) {
	if !strings.HasPrefix(content, "{") {
		content = "{" + content + "}"
	}
	if err := json.Unmarshal([]byte(content), &ex.globalOptions); err != nil {
		if ex.logger != nil {
			ex.logger.LogWarn("invalid syntax of global", err.Error())
		}
		return
	}
}

func (ex *ChartBase) getGlobalOptions() []charts.GlobalOpts {
	theme := "white"
	if ex.globalOptions.Theme != "" {
		theme = ex.globalOptions.Theme
	}
	ret := []charts.GlobalOpts{
		charts.WithInitializationOpts(opts.Initialization{
			PageTitle:       ex.globalOptions.PageTitle,
			Width:           ex.globalOptions.Width,
			Height:          ex.globalOptions.Height,
			BackgroundColor: ex.globalOptions.BackgroundColor,
			ChartID:         ex.globalOptions.ChartID,
			AssetsHost:      ex.globalOptions.AssetsHost,
			Theme:           theme,
		}),
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
			bc.XYAxis = ex.globalOptions.XYAxis
			bc.DataZoomList = ex.globalOptions.DataZoomList
			bc.VisualMapList = ex.globalOptions.VisualMapList
			bc.GridList = ex.globalOptions.GridList
			bc.Grid3D = ex.globalOptions.Grid3D
		},
	}

	for i, xaxis := range ex.globalOptions.XAxisList {
		xaxis.Type = "" // REMOVE ME: debug
		ret = append(ret, charts.WithXAxisOpts(xaxis, i))
	}
	for i, yaxis := range ex.globalOptions.YAxisList {
		yaxis.Type = "" // REMOVE ME: debug
		ret = append(ret, charts.WithYAxisOpts(yaxis, i))
	}
	return ret
}

func (ex *ChartBase) SetSeries(idx int, content string) {
	if ex.multiSeries == nil {
		ex.multiSeries = map[int]*charts.SingleSeries{}
	}
	ser := ex.getSeries(idx, true)
	if !strings.HasPrefix(content, "{") {
		content = "{" + content + "}"
	}
	err := json.Unmarshal([]byte(content), ser)
	if err != nil {
		if ex.logger != nil {
			ex.logger.LogWarnf("series(%d,...) %s", idx, err.Error())
		}
		return
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

				MarkLines: &opts.MarkLines{
					MarkLineStyle: opts.MarkLineStyle{
						Symbol: []string{"none", "none"},
					},
				},
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

type MarkArea struct {
	opts.MarkAreaNameCoordItem
	SeriesIdx int
}

type MarkLineXAxis struct {
	SeriesIdx int
	Name      string
	XAxis     any
}

type MarkLineYAxis struct {
	SeriesIdx int
	Name      string
	YAxis     any
}
