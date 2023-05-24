package echart

import (
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	spi "github.com/machbase/neo-spi"
)

type Exporter struct {
	xLabels      []any
	seriesLabels []string
	series       [][]opts.LineData

	TimeLocation *time.Location
	Output       spi.OutputStream
	Rownum       bool
	Heading      bool
	TimeFormat   string
	Precision    int
	Title        string
	Subtitle     string
}

func NewEncoder() *Exporter {
	return &Exporter{}
}

func (ex *Exporter) ContentType() string {
	return "text/html"
}

func (ex *Exporter) Open(cols spi.Columns) error {
	names := cols.Names()
	ex.seriesLabels = names[1:]
	ex.series = make([][]opts.LineData, len(ex.seriesLabels))
	return nil
}

func (ex *Exporter) Close() {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeWesteros}),
		charts.WithTitleOpts(opts.Title{
			Title:    ex.Title,
			Subtitle: ex.Subtitle,
		}))
	// Put data into instance
	line.SetXAxis(ex.xLabels)
	for i, label := range ex.seriesLabels {
		line.AddSeries(label, ex.series[i]).
			SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	}
	line.Render(ex.Output)
}

func (ex *Exporter) Flush(heading bool) {
}

func (ex *Exporter) AddRow(values []any) error {
	ex.xLabels = append(ex.xLabels, values[0])
	for n := 1; n < len(values); n++ {
		ov := opts.LineData{
			Value: values[n],
		}
		ex.series[n-1] = append(ex.series[n-1], ov)
	}
	return nil
}
