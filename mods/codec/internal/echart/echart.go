package echart

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
)

var globalLog logging.Log

type ChartBase struct {
	output spec.OutputStream

	toJsonOutput bool

	globalOptions ChartGlobalOptions
	seriesOpts    []*ChartSeriesOptions
	seriesLabels  []string

	onceInit sync.Once
}

type ChartGlobalOptions struct {
	PageTitle       string   `json:"pageTitle" default:"chart"` // HTML title
	Width           string   `json:"width" default:"600px"`     // Width of canvas
	Height          string   `json:"height" default:"600px"`    // Height of canvas
	BackgroundColor string   `json:"backgroundColor"`           // BackgroundColor of canvas
	ChartID         string   `json:"chartId"`                   // Chart unique ID
	AssetsHost      string   `json:"assetsHost" default:"https://go-echarts.github.io/go-echarts-assets/assets/"`
	Theme           string   `json:"theme" default:"white"`
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

type ChartSeriesOptions struct {
	Name                string `json:"name,omitempty"`
	*opts.Encode        `json:"encode,omitempty"`
	*opts.ItemStyle     `json:"itemStyle,omitempty"`
	*opts.Label         `json:"label,omitempty"`
	*opts.LabelLine     `json:"labelLine,omitempty"`
	*opts.Emphasis      `json:"emphasis,omitempty"`
	*opts.MarkLines     `json:"markLine,omitempty"`
	*opts.MarkAreas     `json:"markArea,omitempty"`
	*opts.MarkPoints    `json:"markPoint,omitempty"`
	*opts.RippleEffect  `json:"rippleEffect,omitempty"`
	*opts.LineStyle     `json:"lineStyle,omitempty"`
	*opts.AreaStyle     `json:"areaStyle,omitempty"`
	*opts.TextStyle     `json:"textStyle,omitempty"`
	*opts.CircularStyle `json:"circular,omitempty"`
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
	opt := opts.VisualMap{}
	util.SetDefaultValue(&opt)
	opt.Min = float32(min)
	opt.Max = float32(max)
	opt.Calculable = true
	opt.InRange = &opts.VisualMapInRange{
		Color: []string{
			"#313695",
			"#4575b4",
			"#74add1",
			"#abd9e9",
			"#e0f3f8",
			"#ffffbf",
			"#fee090",
			"#fdae61",
			"#f46d43",
			"#d73027",
			"#a50026",
		},
	}
	if opt.Min > opt.Max {
		// reverse colors
		for i, j := 0, len(opt.InRange.Color)-1; i < j; i, j = i+1, j-1 {
			opt.InRange.Color[i], opt.InRange.Color[j] = opt.InRange.Color[j], opt.InRange.Color[i]
		}
	}
	ex.globalOptions.VisualMapList = append(ex.globalOptions.VisualMapList, opt)
}

func (ex *ChartBase) SetChartJson(flag bool) {
	ex.toJsonOutput = flag
	if flag {
		ex.globalOptions.Theme = "-" // client choose 'white' or 'dark'
	} else {
		ex.globalOptions.Theme = "white" // echarts default
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

func (ex *ChartBase) SetGlobalOptions(content string) {
	if err := json.Unmarshal([]byte(content), &ex.globalOptions); err != nil {
		if globalLog == nil {
			globalLog = logging.GetLog("chart")
		}
		globalLog.Warn("invalid syntax of globalOptions", err.Error())
		return
	}
}

func (ex *ChartBase) getGlobalOptions() []charts.GlobalOpts {
	ret := []charts.GlobalOpts{
		charts.WithInitializationOpts(opts.Initialization{
			PageTitle:       ex.globalOptions.PageTitle,
			Width:           ex.globalOptions.Width,
			Height:          ex.globalOptions.Height,
			BackgroundColor: ex.globalOptions.BackgroundColor,
			ChartID:         ex.globalOptions.ChartID,
			AssetsHost:      ex.globalOptions.AssetsHost,
			Theme:           ex.globalOptions.Theme,
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

func (ex *ChartBase) SetSeriesOptions(data ...string) {
	ret := []*ChartSeriesOptions{}
	for _, content := range data {
		opt := &ChartSeriesOptions{}
		err := json.Unmarshal([]byte(content), opt)
		if err != nil {
			if globalLog == nil {
				globalLog = logging.GetLog("chart")
			}
			globalLog.Warn("invalid syntax of seriesOptions", err.Error())
			return
		}
		ret = append(ret, opt)
	}
	ex.seriesOpts = ret
}

func (ex *ChartBase) SetSeriesLabels(labels ...string) {
	ex.seriesLabels = labels
}

func (ex *ChartBase) getSeriesName(idx int) string {
	var label string
	if idx < len(ex.seriesOpts) {
		label = ex.seriesOpts[idx].Name
	}

	if label == "" && idx < len(ex.seriesLabels) {
		label = ex.seriesLabels[idx]
	}

	if label == "" {
		label = fmt.Sprintf("column[%d]", idx)
	}
	return label
}

func (ex *ChartBase) getSeriesOptions(idx int) []charts.SeriesOpts {
	var ret = []charts.SeriesOpts{
		charts.WithLabelOpts(opts.Label{
			Show: false,
		}),
		charts.WithLineChartOpts(
			opts.LineChart{
				Smooth:     true,
				XAxisIndex: 0,
			},
		),
	}

	if len(ex.seriesOpts) <= idx {
		return ret
	}

	ret = append(ret, func(s *charts.SingleSeries) {
		opt := ex.seriesOpts[idx]
		if opt.Encode != nil {
			s.Encode = opt.Encode
		}
		if opt.ItemStyle != nil {
			s.ItemStyle = opt.ItemStyle
		}
		if opt.Label != nil {
			s.Label = opt.Label
		}
		if opt.LabelLine != nil {
			s.LabelLine = opt.LabelLine
		}
		if opt.Emphasis != nil {
			s.Emphasis = opt.Emphasis
		}
		if opt.MarkLines != nil {
			s.MarkLines = opt.MarkLines
		}
		if opt.MarkAreas != nil {
			s.MarkAreas = opt.MarkAreas
		}
		if opt.MarkPoints != nil {
			s.MarkPoints = opt.MarkPoints
		}
		if opt.RippleEffect != nil {
			s.RippleEffect = opt.RippleEffect
		}
		if opt.LineStyle != nil {
			s.LineStyle = opt.LineStyle
		}
		if opt.AreaStyle != nil {
			s.AreaStyle = opt.AreaStyle
		}
		if opt.TextStyle != nil {
			s.TextStyle = opt.TextStyle
		}
		if opt.CircularStyle != nil {
			s.CircularStyle = opt.CircularStyle
		}
	})

	return ret
}

type MarkAreaNameCoord struct {
	Label       string
	Coordinate0 []any
	Coordinate1 []any
	Color       string
	Opacity     float32
}

type MarkLineXAxisCoord struct {
	Name  string
	XAxis any
}

type MarkLineYAxisCoord struct {
	Name  string
	YAxis any
}
