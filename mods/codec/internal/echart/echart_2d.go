package echart

import (
	"fmt"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/render"
	"github.com/machbase/neo-server/mods/util"
)

type RectChartType string

const (
	LINE    RectChartType = "line"
	BAR     RectChartType = "bar"
	SCATTER RectChartType = "scatter"
)

type Base2D struct {
	ChartBase

	chartType RectChartType

	xAxisIdx int
	yAxisIdx int

	xLabels       []any
	lineSeries    [][]opts.LineData
	scatterSeries [][]opts.ScatterData
	barSeries     [][]opts.BarData

	useTimeformatter bool
	timeformatter    *util.TimeFormatter

	markAreaNameCoord  []*MarkAreaNameCoord
	markLineXAxisCoord []*MarkLineXAxisCoord
	markLineYAxisCoord []*MarkLineYAxisCoord
}

func NewRectChart(chartType RectChartType) *Base2D {
	ret := &Base2D{
		chartType: chartType,
		xAxisIdx:  0,
		yAxisIdx:  1,
	}
	ret.initialize()
	return ret
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

// xAxis(idx int, label string, typ string)
func (ex *Base2D) SetXAxis(args ...any) {
	if len(args) != 2 && len(args) != 3 {
		if ex.logger != nil {
			ex.logger.LogError("xAxis(idx, label [, type]), got %d args", len(args))
		}
		return
	}
	if n, err := util.ToFloat64(args[0]); err != nil {
		if ex.logger != nil {
			ex.logger.LogError("xAxis() idx", err.Error())
		}
	} else {
		ex.xAxisIdx = int(n)
	}
	if label, ok := args[1].(string); !ok {
		if ex.logger != nil {
			ex.logger.LogError("xAxis() label should be string, got %T", args[1])
		}
	} else {
		ex.globalOptions.XYAxis.XAxisList[0].Name = label
	}
	if len(args) == 3 {
		if typ, ok := args[2].(string); !ok {
			if ex.logger != nil {
				ex.logger.LogError("xAxis() type should be string, got %T", args[1])
			}
		} else {
			ex.globalOptions.XYAxis.XAxisList[0].Type = typ
		}
	}
	if ex.globalOptions.XYAxis.XAxisList[0].Type == "time" {
		ex.useTimeformatter = true
	}
}

// yAxis(idx int, label string, typ string)
func (ex *Base2D) SetYAxis(args ...any) {
	if len(args) != 2 && len(args) != 3 {
		if ex.logger != nil {
			ex.logger.LogError("yAxis(idx, label [, type]), got %d args", len(args))
		}
		return
	}
	if n, err := util.ToFloat64(args[0]); err != nil {
		if ex.logger != nil {
			ex.logger.LogError("yAxis() idx", err.Error())
		}
	} else {
		ex.yAxisIdx = int(n)
	}
	if label, ok := args[1].(string); !ok {
		if ex.logger != nil {
			ex.logger.LogError("yAxis() label should be string, got %T", args[1])
		}
	} else {
		ex.globalOptions.XYAxis.YAxisList[0].Name = label
	}
	if len(args) == 3 {
		if typ, ok := args[2].(string); !ok {
			if ex.logger != nil {
				ex.logger.LogError("yAxis() type should be string, got %T", args[1])
			}
		} else {
			ex.globalOptions.XYAxis.YAxisList[0].Type = typ
		}
	}
}

func (ex *Base2D) finalizeXAxis() []any {
	ret := make([]any, len(ex.xLabels))
	if ex.useTimeformatter {
		for i := range ex.xLabels {
			ret[i] = ex.renderXAxisLabelIndex(i)
		}
	} else {
		ret = ex.xLabels
	}
	return ret
}

func (ex *Base2D) renderXAxisLabelIndex(idx int) any {
	if idx < 0 || idx >= len(ex.xLabels) {
		return "n/a"
	}
	element := ex.xLabels[idx]

	if ex.useTimeformatter {
		var tv *time.Time
		switch v := element.(type) {
		case *time.Time:
			tv = v
		case time.Time:
			tv = &v
		}
		if ex.timeformatter != nil && tv != nil {
			return ex.timeformatter.Format(*tv)
		} else {
			return element
		}
	}
	return element
}

func (ex *Base2D) SetTimeformat(format string) {
	if ex.timeformatter == nil {
		ex.timeformatter = util.NewTimeFormatter()
	}
	ex.timeformatter.Set(util.Timeformat(format))
}

func (ex *Base2D) SetTimeLocation(tz *time.Location) {
	if ex.timeformatter == nil {
		ex.timeformatter = util.NewTimeFormatter()
	}
	ex.timeformatter.Set(util.TimeLocation(tz))
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

func (ex *Base2D) xLabelCompare(x, y any) bool {
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
			if ex.logger != nil {
				ex.logger.LogErrorf("unhandled comparison int64: %T\n", v)
			}
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
			if ex.logger != nil {
				ex.logger.LogErrorf("unhandled comparison float64: %T\n", v)
			}
		}
		return -1.0
	}

	switch xv := x.(type) {
	case time.Time:
		return xv.UnixNano() >= toInt64(y)
	case int64:
		return xv >= toInt64(y)
	case float64:
		return xv >= toFloat64(y)
	default:
		if ex.logger != nil {
			ex.logger.LogErrorf("unhandled comparison x: %T(%v)\n", xv, xv)
		}
		return false
	}
}

func (ex *Base2D) getSeriesOptions(seriesIdx int) []charts.SeriesOpts {
	var ret = ex.ChartBase.getSeriesOptions(seriesIdx)
	for _, mark := range ex.markAreaNameCoord {
		if len(mark.Coordinate0) > 0 && len(mark.Coordinate1) > 0 {
			var idx0, idx1 int = -1, -1
			for i, v := range ex.xLabels {
				if idx0 == -1 && ex.xLabelCompare(v, mark.Coordinate0[0]) {
					idx0 = i
				}
				if idx1 == -1 && ex.xLabelCompare(v, mark.Coordinate1[0]) {
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
						Coordinate0: []any{ex.renderXAxisLabelIndex(idx0)},
						Coordinate1: []any{ex.renderXAxisLabelIndex(idx1)},
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
			if idx == -1 && ex.xLabelCompare(v, mark.XAxis) {
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

func (ex *Base2D) Close() {
	var chart any
	var before []func()

	switch ex.chartType {
	case LINE:
		line := charts.NewLine()
		line.SetGlobalOptions(ex.getGlobalOptions()...)
		line.SetXAxis(ex.finalizeXAxis())
		for i, series := range ex.lineSeries {
			label := ex.getSeriesName(i)
			opts := ex.getSeriesOptions(i)
			line.AddSeries(label, series, opts...)
		}
		chart = line
		before = append(before, line.Validate)
	case SCATTER:
		scatter := charts.NewScatter()
		scatter.SetGlobalOptions(ex.getGlobalOptions()...)
		scatter.SetXAxis(ex.finalizeXAxis())
		for i, series := range ex.scatterSeries {
			label := ex.getSeriesName(i)
			opts := ex.getSeriesOptions(i)
			scatter.AddSeries(label, series, opts...)
		}
		chart = scatter
		before = append(before, scatter.Validate)
	case BAR:
		bar := charts.NewBar()
		bar.SetGlobalOptions(ex.getGlobalOptions()...)
		bar.SetXAxis(ex.finalizeXAxis())
		for i, series := range ex.barSeries {
			label := ex.getSeriesName(i)
			opts := ex.getSeriesOptions(i)
			bar.AddSeries(label, series, opts...)
		}
		chart = bar
		before = append(before, bar.Validate)
	}

	var rndr render.Renderer
	if ex.toJsonOutput {
		rndr = newJsonRender(chart, before...)
	} else {
		rndr = newChartRender(chart, before...)
	}
	err := rndr.Render(ex.output)
	if err != nil {
		fmt.Println("ERR", err.Error())
	}
}

func (ex *Base2D) AddRow(values []any) error {
	switch ex.chartType {
	case LINE:
		if ex.lineSeries == nil {
			ex.lineSeries = make([][]opts.LineData, len(values)-1)
		}
		if len(ex.lineSeries) < len(values)-1 {
			for i := 0; i < len(values)-1-len(ex.lineSeries); i++ {
				ex.lineSeries = append(ex.lineSeries, []opts.LineData{})
			}
		}
	case SCATTER:
		if ex.scatterSeries == nil {
			ex.scatterSeries = make([][]opts.ScatterData, len(values)-1)
		}
		if len(ex.scatterSeries) < len(values)-1 {
			for i := 0; i < len(values)-1-len(ex.scatterSeries); i++ {
				ex.scatterSeries = append(ex.scatterSeries, []opts.ScatterData{})
			}
		}
	case BAR:
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
		if vv, ok := v.(time.Time); ok {
			v = vv.UnixMilli()
		} else if vv, ok := v.(*time.Time); ok {
			v = vv.UnixMilli()
		}
		switch ex.chartType {
		case LINE:
			ov := opts.LineData{
				Value: v,
			}
			ex.lineSeries[seriesIdx] = append(ex.lineSeries[seriesIdx], ov)
		case SCATTER:
			ov := opts.ScatterData{
				Value:      v,
				SymbolSize: 5,
			}
			ex.scatterSeries[seriesIdx] = append(ex.scatterSeries[seriesIdx], ov)
		case BAR:
			ov := opts.BarData{
				Value: v,
			}
			ex.barSeries[seriesIdx] = append(ex.barSeries[seriesIdx], ov)
		}
	}
	return nil
}
