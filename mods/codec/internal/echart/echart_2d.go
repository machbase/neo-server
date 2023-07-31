package echart

import (
	"fmt"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/render"
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

	markAreaNameCoord  []*MarkAreaNameCoord
	markLineXAxisCoord []*MarkLineXAxisCoord
	markLineYAxisCoord []*MarkLineYAxisCoord
}

func (ex *Base2D) ContentType() string {
	if ex.toJsonOutput {
		return "application/json"
	}
	return "text/html"
}

func (ex *Base2D) Open() error {
	return nil
}

func (ex *Base2D) Flush(heading bool) {
}

func (ex *Base2D) SetXAxis(idx int, label string, typ ...string) {
	ex.xAxisIdx = idx
	ex.xAxisLabel = label
}

func (ex *Base2D) SetYAxis(idx int, label string, typ ...string) {
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

func (ex *Base2D) SetMarkAreaNameCoord(from any, to any, label string, color string, opacity float64) {
	ex.markAreaNameCoord = append(ex.markAreaNameCoord, &MarkAreaNameCoord{
		Label:       label,
		Coordinate0: []any{from},
		Coordinate1: []any{to},
		Color:       color,
		Opacity:     float32(opacity),
	})
}

func (ex *Base2D) SetMarkLineXAxisCoord(xaxis any, name string) {
	ex.markLineXAxisCoord = append(ex.markLineXAxisCoord, &MarkLineXAxisCoord{
		Name:  name,
		XAxis: xaxis,
	})
}

func (ex *Base2D) SetMarkLineYAxisCoord(yaxis any, name string) {
	ex.markLineYAxisCoord = append(ex.markLineYAxisCoord, &MarkLineYAxisCoord{
		Name:  name,
		YAxis: yaxis,
	})
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

	assetHost := "https://go-echarts.github.io/go-echarts-assets/assets/"
	if len(ex.assetHost) > 0 {
		assetHost = ex.assetHost
	}
	globalOptions := []charts.GlobalOpts{
		charts.WithInitializationOpts(opts.Initialization{
			AssetsHost: assetHost,
			Theme:      ex.Theme(),
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

func xLabelCompare(x, y any) bool {
	toInt64 := func(o any) int64 {
		switch v := o.(type) {
		case int64:
			return v
		case *int64:
			return *v
		case float64:
			return int64(v)
		case time.Time:
			return v.UnixNano()
		default:
			fmt.Printf("ERR unhandled compare int64====> %T\n", v)
		}
		return -1
	}

	toFloat64 := func(o any) float64 {
		switch v := o.(type) {
		case float64:
			return v
		case *float64:
			return *v
		case int:
			return float64(v)
		case time.Time:
			return float64(v.UnixNano())
		default:
			fmt.Printf("ERR unhandled compare float64====> %T\n", v)
		}
		return -1.0
	}

	switch xv := x.(type) {
	case time.Time:
		return xv.UnixNano()-toInt64(y) >= 0
	case int64:
		return xv-toInt64(y) >= 0
	case float64:
		return xv-toFloat64(y) >= 0
	default:
		fmt.Printf("ERR unhandled compare x====> %T\n", xv)
		return false
	}
}
func (ex *Base2D) getSeriesOptions() []charts.SeriesOpts {
	var ret []charts.SeriesOpts
	for _, mark := range ex.markAreaNameCoord {
		if len(mark.Coordinate0) > 0 && len(mark.Coordinate1) > 0 {
			var idx0, idx1 int = -1, -1
			for i, v := range ex.xLabels {
				if idx0 == -1 && xLabelCompare(v, mark.Coordinate0[0]) {
					idx0 = i
				}
				if idx1 == -1 && xLabelCompare(v, mark.Coordinate1[0]) {
					idx1 = i
				}
				if idx0 != -1 && idx1 != -1 {
					break
				}
			}
			if idx0 == -1 && idx1 != -1 {
				idx0 = 0
			} else if idx0 != -1 && idx1 == -1 {
				idx1 = len(ex.xLabels) - 1
			}
			if idx0 >= 0 && idx1 >= 0 {
				ret = append(ret,
					charts.WithMarkAreaNameCoordItemOpts(opts.MarkAreaNameCoordItem{
						Name:        mark.Label,
						Coordinate0: []any{ex.xLabels[idx0]},
						Coordinate1: []any{ex.xLabels[idx1]},
						ItemStyle: &opts.ItemStyle{
							Color:   mark.Color,
							Opacity: mark.Opacity,
						},
					}),
				)
			}
		}
	}

	for _, mark := range ex.markLineXAxisCoord {
		var idx int = -1
		for i, v := range ex.xLabels {
			if idx == -1 && xLabelCompare(v, mark.XAxis) {
				idx = i
			}
			if idx != -1 {
				break
			}
		}
		if idx >= 0 {
			ret = append(ret,
				charts.WithMarkLineNameXAxisItemOpts(opts.MarkLineNameXAxisItem{
					Name:  mark.Name,
					XAxis: ex.xLabels[idx],
				}),
			)
		}
	}

	for _, mark := range ex.markLineYAxisCoord {
		ret = append(ret,
			charts.WithMarkLineNameYAxisItemOpts(opts.MarkLineNameYAxisItem{
				Name:  mark.Name,
				YAxis: mark.YAxis,
			}),
		)
	}

	return ret
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

func (ex *Line) Close() {
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
	seriesOpts = append(seriesOpts, ex.getSeriesOptions()...)
	for i, series := range ex.lineSeries {
		label := ex.getRenderSeriesLabel(i)
		line.AddSeries(label, series, seriesOpts...)
	}
	var rndr render.Renderer
	if ex.toJsonOutput {
		rndr = newJsonRender(line, line.Validate)
	} else {
		rndr = newChartRender(line, line.Validate)
	}
	err := rndr.Render(ex.output)
	if err != nil {
		fmt.Println("ERR", err.Error())
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

func (ex *Scatter) Close() {
	scatter := charts.NewScatter()
	scatter.SetGlobalOptions(ex.getGlobalOptions()...)
	scatter.SetXAxis(ex.xLabels)
	seriesOpts := []charts.SeriesOpts{
		charts.WithLabelOpts(opts.Label{
			Show: false,
		}),
	}
	seriesOpts = append(seriesOpts, ex.getSeriesOptions()...)
	for i, series := range ex.scatterSeries {
		label := ex.getRenderSeriesLabel(i)
		scatter.AddSeries(label, series, seriesOpts...)
	}
	var rndr render.Renderer
	if ex.toJsonOutput {
		rndr = newJsonRender(scatter, scatter.Validate)
	} else {
		rndr = newChartRender(scatter, scatter.Validate)
	}
	err := rndr.Render(ex.output)
	if err != nil {
		fmt.Println("ERR", err.Error())
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

func (ex *Bar) Close() {
	bar := charts.NewBar()
	bar.SetGlobalOptions(ex.getGlobalOptions()...)
	bar.SetXAxis(ex.xLabels)
	seriesOpts := []charts.SeriesOpts{
		charts.WithLabelOpts(opts.Label{
			Show: false,
		}),
	}
	seriesOpts = append(seriesOpts, ex.getSeriesOptions()...)
	for i, series := range ex.barSeries {
		label := ex.getRenderSeriesLabel(i)
		bar.AddSeries(label, series, seriesOpts...)
	}
	var rndr render.Renderer
	if ex.toJsonOutput {
		rndr = newJsonRender(bar, bar.Validate)
	} else {
		rndr = newChartRender(bar, bar.Validate)
	}
	err := rndr.Render(ex.output)
	if err != nil {
		fmt.Println("ERR", err.Error())
	}
}
