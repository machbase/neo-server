package echart

import (
	"encoding/json"
	"fmt"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/stream/spec"
)

var globalLog logging.Log

type ChartBase struct {
	output spec.OutputStream

	title    string
	subtitle string
	theme    string
	width    string
	height   string

	assetHost    string
	toJsonOutput bool

	seriesOpts   []*ChartSeriesOptions
	seriesLabels []string
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
	ex.width = width
	ex.height = height
}

func (ex *ChartBase) SetTheme(theme string) {
	ex.theme = theme
}

func (ex *ChartBase) SetTitle(title string) {
	ex.title = title
}

func (ex *ChartBase) SetSubtitle(subtitle string) {
	ex.subtitle = subtitle
}

func (ex *ChartBase) SetAssetHost(path string) {
	ex.assetHost = path
}

func (ex *ChartBase) SetChartJson(flag bool) {
	ex.toJsonOutput = flag
}

func (ex *ChartBase) Theme() string {
	if ex.theme == "" {
		if ex.toJsonOutput {
			return "-" // client choose 'white' or 'dark'
		} else {
			return "white" // echarts default
		}
	} else {
		return ex.theme
	}
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
		s.Encode = opt.Encode
		s.ItemStyle = opt.ItemStyle
		s.Label = opt.Label
		s.LabelLine = opt.LabelLine
		s.Emphasis = opt.Emphasis
		s.MarkLines = opt.MarkLines
		s.MarkAreas = opt.MarkAreas
		s.MarkPoints = opt.MarkPoints
		s.RippleEffect = opt.RippleEffect
		s.LineStyle = opt.LineStyle
		s.AreaStyle = opt.AreaStyle
		s.TextStyle = opt.TextStyle
		s.CircularStyle = opt.CircularStyle
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
