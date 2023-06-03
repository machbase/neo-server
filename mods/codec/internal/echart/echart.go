package echart

import (
	"fmt"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	"github.com/machbase/neo-server/mods/stream/spec"
)

type ChartBase struct {
	output spec.OutputStream

	title    string
	subtitle string
	theme    string
	width    string
	height   string
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

type Line struct {
	ChartBase
	xLabels      []any
	series       [][]opts.LineData
	seriesLabels []string

	xAxisIdx   int
	yAxisIdx   int
	xAxisLabel string
	yAxisLabel string

	dataZoomType  string  // inside, slider
	dataZoomStart float32 // 0 ~ 100 %
	dataZoomEnd   float32 // 0 ~ 100 %

	TimeLocation *time.Location
	TimeFormat   string
}

func NewLine() *Line {
	return &Line{
		xAxisIdx:   0,
		xAxisLabel: "x",
		yAxisIdx:   1,
		yAxisLabel: "y",
	}
}

func (ex *Line) ContentType() string {
	return "text/html"
}

func (ex *Line) Open() error {
	return nil
}

func (ex *Line) Close() {
	width := "600px"
	if ex.width != "" {
		width = ex.width
	}
	height := "400px"
	if ex.height != "" {
		height = ex.height
	}

	theme := ex.theme
	if theme == "" {
		theme = types.ThemeWesteros
	}

	globalOptions := []charts.GlobalOpts{
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  theme,
			Width:  width,
			Height: height,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    ex.title,
			Subtitle: ex.subtitle,
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    true,
			Trigger: "axis",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: ex.xAxisLabel,
			Show: true,
		}, 0),
		charts.WithYAxisOpts(opts.YAxis{
			Name: ex.yAxisLabel,
			Show: true,
		}, 0),
	}
	if ex.dataZoomStart < ex.dataZoomEnd {
		globalOptions = append(globalOptions,
			charts.WithDataZoomOpts(opts.DataZoom{
				Type:  ex.dataZoomType,
				Start: ex.dataZoomStart,
				End:   ex.dataZoomEnd,
			}),
		)
	}
	line := charts.NewLine()
	line.SetGlobalOptions(globalOptions...)
	// Put data into instance
	line.SetXAxis(ex.xLabels)

	for i, series := range ex.series {
		var label string
		if i < len(ex.seriesLabels) {
			label = ex.seriesLabels[i]
		} else {
			label = fmt.Sprintf("column[%d]", i)
		}
		line.AddSeries(label, series,
			charts.WithLabelOpts(opts.Label{
				Show: true,
				// Color: "red",
			}),
			charts.WithLineChartOpts(
				opts.LineChart{
					Smooth:     true,
					XAxisIndex: 0,
				},
			),
		)
	}
	line.Render(ex.output)
}

func (ex *Line) Flush(heading bool) {
}

func (ex *Line) SetDataZoom(typ string, start float32, end float32) {
	ex.dataZoomType = typ
	ex.dataZoomStart = start
	ex.dataZoomEnd = end
}

func (ex *Line) SetXAxis(idx int, label string, typ string) {
	ex.xAxisIdx = idx
	ex.xAxisLabel = label
}

func (ex *Line) SetYAxis(idx int, label string, typ string) {
	ex.yAxisIdx = idx
	ex.yAxisLabel = label
}

func (ex *Line) SetSeriesLabels(labels ...string) {
	ex.seriesLabels = labels
}

func (ex *Line) AddRow(values []any) error {
	if ex.series == nil {
		ex.series = make([][]opts.LineData, len(values)-1)
	}

	if len(ex.series) < len(values)-1 {
		for i := 0; i < len(values)-1-len(ex.series); i++ {
			ex.series = append(ex.series, []opts.LineData{})
		}
	}
	ex.xLabels = append(ex.xLabels, values[ex.xAxisIdx])
	seriesIdx := -1
	for n, v := range values {
		if n == ex.xAxisIdx {
			continue
		} else {
			seriesIdx++
		}
		ov := opts.LineData{
			Value: v,
		}
		ex.series[seriesIdx] = append(ex.series[seriesIdx], ov)
	}
	return nil
}
