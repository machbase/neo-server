package echart

import (
	"fmt"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

type Base2D struct {
	ChartBase

	chartType string

	xAxisIdx   int
	yAxisIdx   int
	xAxisLabel string
	yAxisLabel string

	xLabels       []any
	lineSeries    [][]opts.LineData
	scatterSeries [][]opts.ScatterData
	barSeries     [][]opts.BarData
	seriesLabels  []string

	dataZoomType  string  // inside, slider
	dataZoomStart float32 // 0 ~ 100 %
	dataZoomEnd   float32 // 0 ~ 100 %

	TimeLocation *time.Location
	TimeFormat   string
}

func (ex *Base2D) ContentType() string {
	return "text/html"
}

func (ex *Base2D) Open() error {
	return nil
}

func (ex *Base2D) Flush(heading bool) {
}

func (ex *Base2D) SetXAxis(idx int, label string, typ string) {
	ex.xAxisIdx = idx
	ex.xAxisLabel = label
}

func (ex *Base2D) SetYAxis(idx int, label string, typ string) {
	ex.yAxisIdx = idx
	ex.yAxisLabel = label
}

func (ex *Base2D) SetDataZoom(typ string, start float32, end float32) {
	ex.dataZoomType = typ
	ex.dataZoomStart = start
	ex.dataZoomEnd = end
}

func (ex *Base2D) SetSeriesLabels(labels ...string) {
	ex.seriesLabels = labels
}

func (ex *Base2D) getGlobalOptions() []charts.GlobalOpts {
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
			AssetsHost: "/echarts/",
			Theme:      theme,
			Width:      width,
			Height:     height,
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
	return globalOptions
}

func (ex *Base2D) AddRow(values []any) error {
	switch ex.chartType {
	case "line":
		if ex.lineSeries == nil {
			ex.lineSeries = make([][]opts.LineData, len(values)-1)
		}
		if len(ex.lineSeries) < len(values)-1 {
			for i := 0; i < len(values)-1-len(ex.lineSeries); i++ {
				ex.lineSeries = append(ex.lineSeries, []opts.LineData{})
			}
		}
	case "scatter":
		if ex.scatterSeries == nil {
			ex.scatterSeries = make([][]opts.ScatterData, len(values)-1)
		}
		if len(ex.scatterSeries) < len(values)-1 {
			for i := 0; i < len(values)-1-len(ex.scatterSeries); i++ {
				ex.scatterSeries = append(ex.scatterSeries, []opts.ScatterData{})
			}
		}
	case "bar":
		if ex.barSeries == nil {
			ex.barSeries = make([][]opts.BarData, len(values)-1)
		}
		if len(ex.barSeries) < len(values)-1 {
			for i := 0; i < len(values)-1-len(ex.barSeries); i++ {
				ex.barSeries = append(ex.barSeries, []opts.BarData{})
			}
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
		switch ex.chartType {
		case "line":
			ov := opts.LineData{
				Value: v,
			}
			ex.lineSeries[seriesIdx] = append(ex.lineSeries[seriesIdx], ov)
		case "scatter":
			ov := opts.ScatterData{
				Value:      v,
				SymbolSize: 5,
			}
			ex.scatterSeries[seriesIdx] = append(ex.scatterSeries[seriesIdx], ov)
		case "bar":
			ov := opts.BarData{
				Value: v,
			}
			ex.barSeries[seriesIdx] = append(ex.barSeries[seriesIdx], ov)
		}
	}
	return nil
}

func (ex *Base2D) getRenderSeriesLabel(idx int) string {
	var label string
	if idx < len(ex.seriesLabels) {
		label = ex.seriesLabels[idx]
	} else {
		label = fmt.Sprintf("column[%d]", idx)
	}
	return label
}

func (ex *Base2D) Close() {
	switch ex.chartType {
	case "line":
		line := charts.NewLine()
		line.SetGlobalOptions(ex.getGlobalOptions()...)
		line.SetXAxis(ex.xLabels)
		seriesOpts := []charts.SeriesOpts{charts.WithLabelOpts(opts.Label{
			Show: true,
		}),
			charts.WithLineChartOpts(
				opts.LineChart{
					Smooth:     true,
					XAxisIndex: 0,
				},
			),
		}
		for i, series := range ex.lineSeries {
			label := ex.getRenderSeriesLabel(i)
			line.AddSeries(label, series, seriesOpts...)
		}
		line.Render(ex.output)
	case "scatter":
		scatter := charts.NewScatter()
		scatter.SetGlobalOptions(ex.getGlobalOptions()...)
		scatter.SetXAxis(ex.xLabels)
		seriesOpts := []charts.SeriesOpts{
			charts.WithLabelOpts(opts.Label{
				Show: false,
			}),
		}
		for i, series := range ex.scatterSeries {
			label := ex.getRenderSeriesLabel(i)
			scatter.AddSeries(label, series, seriesOpts...)
		}
		scatter.Render(ex.output)
	case "bar":
		bar := charts.NewBar()
		bar.SetGlobalOptions(ex.getGlobalOptions()...)
		bar.SetXAxis(ex.xLabels)
		seriesOpts := []charts.SeriesOpts{
			charts.WithLabelOpts(opts.Label{
				Show: false,
			}),
		}
		for i, series := range ex.barSeries {
			label := ex.getRenderSeriesLabel(i)
			bar.AddSeries(label, series, seriesOpts...)
		}
		bar.Render(ex.output)
	}
}

type Line struct {
	Base2D
}

func NewLine() *Line {
	return &Line{
		Base2D{
			chartType: "line",
			xAxisIdx:  0, xAxisLabel: "x",
			yAxisIdx: 1, yAxisLabel: "y",
		},
	}
}

type Scatter struct {
	Base2D
}

func NewScatter() *Scatter {
	return &Scatter{
		Base2D{
			chartType: "scatter",
			xAxisIdx:  0, xAxisLabel: "x",
			yAxisIdx: 1, yAxisLabel: "y",
		},
	}
}

type Bar struct {
	Base2D
}

func NewBar() *Bar {
	return &Bar{
		Base2D{
			chartType: "bar",
			xAxisIdx:  0, xAxisLabel: "x",
			yAxisIdx: 1, yAxisLabel: "y",
		},
	}
}
