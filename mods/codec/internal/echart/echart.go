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
	seriesLabels map[int]string

	TimeLocation *time.Location
	TimeFormat   string
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

	line := charts.NewLine()
	line.SetGlobalOptions(
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
		// charts.WithXAxisOpts(opts.XAxis{
		// 	Name: "time",
		// 	Show: true,
		// 	Min:  ex.xLabels[0],
		// 	Max:  ex.xLabels[len(ex.xLabels)-1],
		// }, 0),
	)
	// Put data into instance
	line.SetXAxis(ex.xLabels)

	for i, series := range ex.series {
		var label string
		if i < len(ex.seriesLabels) {
			label = ex.seriesLabels[i]
		} else {
			label = fmt.Sprintf("value[%d]", i)
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

func (ex *Line) SetSeries(idx int, label string) {
	if ex.seriesLabels == nil {
		ex.seriesLabels = map[int]string{}
	}
	ex.seriesLabels[idx] = label
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
	ex.xLabels = append(ex.xLabels, values[0])
	for n := 1; n < len(values); n++ {
		ov := opts.LineData{
			Value: values[n],
		}
		ex.series[n-1] = append(ex.series[n-1], ov)
	}
	return nil
}
