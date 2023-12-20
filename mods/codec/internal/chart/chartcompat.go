package chart

import (
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/machbase/neo-server/mods/util"
)

type ChartW struct {
	*Chart
	Type      string
	VisualMap opts.VisualMap
}

func NewRectChart(typ string) *ChartW {
	switch typ {
	default: // case "line":
		return &ChartW{Chart: NewChart(), Type: "line"}
	case "scatter", "bar":
		return &ChartW{Chart: NewChart(), Type: typ}
	}
}

func (w *ChartW) SetGlobalOptions(opt string) {
	w.Chart.SetChartOption(opt)
}

func (w *ChartW) SetVisualMap(min float64, max float64) {
	colors := []string{
		"#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
		"#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026",
	}
	w.SetVisualMapColor(min, max, colors...)
}

func (w *ChartW) SetVisualMapColor(min float64, max float64, colors ...string) {
	opt := opts.VisualMap{}
	util.SetDefaultValue(&opt)
	opt.Min = float32(min)
	opt.Max = float32(max)
	opt.Calculable = true
	opt.InRange = &opts.VisualMapInRange{
		Color: colors,
	}
	w.VisualMap = opt
}

func (w *ChartW) Close() {
	w.option = `{
		"xAxis": { "type": "time", "data": column(0 ) },
		"yAxis": { "type": "value"},
		"series": [
			{ "type": "line", "data": column( 1) }
		]
	}`

	w.Chart.Close()
}
