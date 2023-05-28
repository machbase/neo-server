package echart

import (
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
)

type Line struct {
	xLabels      []any
	seriesLabels []string
	series       [][]opts.LineData

	TimeLocation *time.Location
	Output       spec.OutputStream
	Rownum       bool
	Heading      bool
	TimeFormat   string
	Precision    int
	Title        string
	Subtitle     string
	Width        string
	Height       string
}

func (ex *Line) ContentType() string {
	return "text/html"
}

func (ex *Line) Open(cols spi.Columns) error {
	names := cols.Names()
	ex.seriesLabels = names[1:]
	ex.series = make([][]opts.LineData, len(ex.seriesLabels))
	return nil
}

func (ex *Line) Close() {
	width := "600px"
	if ex.Width != "" {
		width = ex.Width
	}
	height := "400px"
	if ex.Height != "" {
		height = ex.Height
	}

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  types.ThemeWesteros,
			Width:  width,
			Height: height,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    ex.Title,
			Subtitle: ex.Subtitle,
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: true, Trigger: "axis"}),
	)
	// Put data into instance
	line.SetXAxis(ex.xLabels)
	for i, label := range ex.seriesLabels {
		line.AddSeries(label, ex.series[i]).
			SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	}
	line.Render(ex.Output)
}

func (ex *Line) Flush(heading bool) {
}

func (ex *Line) AddRow(values []any) error {
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
