package chart

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/render"
)

type Base2D struct {
	ChartBase

	xLabels []any

	markAreaXAxis []*MarkArea
	markAreaYAxis []*MarkArea
	markLineXAxis []*MarkLineXAxis
	markLineYAxis []*MarkLineYAxis
}

func NewRectChart(defaultType string) *Base2D {
	ret := &Base2D{}
	ret.defaultChartType = defaultType
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

func (ex *Base2D) SetMarkAreaXAxis(seriesIdx int, from any, to any, content string) {
	var item opts.MarkAreaNameCoordItem

	if !strings.HasPrefix(content, "{") {
		content = "{" + content + "}"
	}
	err := json.Unmarshal([]byte(content), &item)
	if err != nil {
		if ex.logger != nil {
			ex.logger.LogWarnf("SetMarkAreaXAxis(...) %s", err.Error())
		}
		return
	}

	item.Coordinate0 = []any{from}
	item.Coordinate1 = []any{to}
	ex.markAreaXAxis = append(ex.markAreaXAxis,
		&MarkArea{MarkAreaNameCoordItem: item, SeriesIdx: seriesIdx},
	)
}

func (ex *Base2D) SetMarkAreaYAxis(seriesIdx int, from any, to any, content string) {
	var item opts.MarkAreaNameCoordItem

	if !strings.HasPrefix(content, "{") {
		content = "{" + content + "}"
	}
	err := json.Unmarshal([]byte(content), &item)
	if err != nil {
		if ex.logger != nil {
			ex.logger.LogWarnf("SetMarkAreaYAxis(...) %s", err.Error())
		}
		return
	}

	item.Coordinate0 = []any{from}
	item.Coordinate1 = []any{to}
	ex.markAreaYAxis = append(ex.markAreaYAxis,
		&MarkArea{MarkAreaNameCoordItem: item, SeriesIdx: seriesIdx},
	)
}

func (ex *Base2D) SetMarkLineXAxis(seriesIdx int, value any) {
	ex.markLineXAxis = append(ex.markLineXAxis, &MarkLineXAxis{
		SeriesIdx: seriesIdx,
		XAxis:     value,
	})
}

func (ex *Base2D) SetMarkLineYAxis(seriesIdx int, value any) {
	ex.markLineYAxis = append(ex.markLineYAxis, &MarkLineYAxis{
		SeriesIdx: seriesIdx,
		YAxis:     value,
	})
}

func (ex *Base2D) SetMarkLineStyle(seriesIdx int, content string) {
	style := opts.MarkLineStyle{Symbol: []string{"none", "none"}}
	if !strings.HasPrefix(content, "{") {
		content = "{" + content + "}"
	}
	err := json.Unmarshal([]byte(content), &style)
	if err != nil {
		if ex.logger != nil {
			ex.logger.LogWarnf("markLineXAxisCoord(...) %s", err.Error())
		}
		return
	}
	ser := ex.getSeries(seriesIdx, true)
	if ser.MarkLines == nil {
		ser.MarkLines = &opts.MarkLines{}
	}
	ser.MarkLines.MarkLineStyle = style
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
	var ret = []charts.SeriesOpts{}
	for _, mark := range ex.markAreaXAxis {
		if mark.SeriesIdx != seriesIdx {
			continue
		}
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
				mark.MarkAreaNameCoordItem.Coordinate0 = []any{ex.renderXAxisLabelIndex(idx0)}
				mark.MarkAreaNameCoordItem.Coordinate1 = []any{ex.renderXAxisLabelIndex(idx1)}
				ret = append(ret, charts.WithMarkAreaNameCoordItemOpts(mark.MarkAreaNameCoordItem))
			}
		}
	}

	for _, mark := range ex.markAreaYAxis {
		if mark.SeriesIdx != seriesIdx {
			continue
		}
		ret = append(ret,
			charts.WithMarkAreaNameYAxisItemOpts(opts.MarkAreaNameYAxisItem{
				Name:  mark.Name,
				YAxis: mark.Coordinate0,
			}),
		)
	}

	for _, mark := range ex.markLineXAxis {
		if mark.SeriesIdx != seriesIdx {
			continue
		}
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
				charts.WithMarkLineNameXAxisItemOpts(
					opts.MarkLineNameXAxisItem{
						Name:  mark.Name,
						XAxis: ex.xLabels[idx],
					}),
			)
		}
	}

	for _, mark := range ex.markLineYAxis {
		if mark.SeriesIdx != seriesIdx {
			continue
		}
		ret = append(ret,
			charts.WithMarkLineNameYAxisItemOpts(opts.MarkLineNameYAxisItem{
				Name:  mark.Name,
				YAxis: mark.YAxis,
			}),
		)
	}

	return ret
}

func (ex *Base2D) renderXAxisLabelIndex(idx int) any {
	if idx < 0 || idx >= len(ex.xLabels) {
		return "n/a"
	}
	element := ex.xLabels[idx]
	if tv, ok := element.(time.Time); ok {
		return tv
	} else {
		return element
	}
}

func (ex *Base2D) Close() {
	var before []func()

	chart := charts.NewLine()
	chart.SetGlobalOptions(ex.getGlobalOptions()...)
	for i := 0; i < len(ex.multiSeries); i++ {
		ser := ex.getSeries(i, false)
		if ser == nil {
			continue
		}
		opts := ex.getSeriesOptions(i)
		for _, o := range opts {
			o(ser)
		}
		if ser.Type == "" {
			ser.Type = ex.defaultChartType
		}
		if ser.Name == "" {
			ser.Name = ex.getSeriesName(i)
		}
		chart.MultiSeries = append(chart.MultiSeries, *ser)
	}
	before = append(before, chart.Validate, func() {
		chart.XAxisList[0].Type = "time"
		chart.XAxisList[0].Show = true
		//chart.XAxisList[0].Data = .. only for categroy ..
		chart.XAxisList[0].AxisLabel = &opts.AxisLabel{
			Show:   true,
			Rotate: 0,
		}
		chart.XAxisList[0].SplitLine = &opts.SplitLine{
			Show: true,
			LineStyle: &opts.LineStyle{
				Width:   0.8,
				Opacity: 0.3,
			},
		}
	})

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
	xAxisValue := values[ex.timeColumnIdx]
	switch xv := xAxisValue.(type) {
	case time.Time:
		ex.xLabels = append(ex.xLabels, xv)
		xAxisValue = float64(xv.UnixNano()) / float64(time.Millisecond)
	case *time.Time:
		ex.xLabels = append(ex.xLabels, *xv)
		xAxisValue = float64(xv.UnixNano()) / float64(time.Millisecond)
	default:
		ex.xLabels = append(ex.xLabels, xv)
	}

	seriesIdx := -1
	for n, v := range values {
		if n == ex.timeColumnIdx {
			continue
		} else {
			seriesIdx++
		}
		ser := ex.getSeries(seriesIdx, true)
		if vv, ok := v.(time.Time); ok {
			v = vv.UnixMilli()
		} else if vv, ok := v.(*time.Time); ok {
			v = vv.UnixMilli()
		}
		var data []any
		if ser.Data == nil {
			data = []any{}
		} else {
			data = ser.Data.([]any)
		}
		// hint: use ChartData instead of v for customizing a specefic item
		data = append(data, []any{xAxisValue, v})
		ser.Data = data
	}
	return nil
}

type ChartData struct {
	// Name of data item.
	Name string `json:"name,omitempty"`

	// Value of a single data item.
	Value interface{} `json:"value,omitempty"`

	// Symbol of single data.
	// Icon types provided by ECharts includes 'circle', 'rect', 'roundRect', 'triangle', 'diamond', 'pin', 'arrow', 'none'
	// It can be set to an image with 'image://url' , in which URL is the link to an image, or dataURI of an image.
	Symbol string `json:"symbol,omitempty"`

	// single data symbol size. It can be set to single numbers like 10, or
	// use an array to represent width and height. For example, [20, 10] means symbol width is 20, and height is10
	SymbolSize int `json:"symbolSize,omitempty"`

	// SymbolRotate (Scatter only)
	SymbolRotate int `json:"symbolRotate,omitempty"`

	// Index of x axis to combine with, which is useful for multiple x axes in one chart.
	XAxisIndex int `json:"xAxisIndex,omitempty"`

	// Index of y axis to combine with, which is useful for multiple y axes in one chart.
	YAxisIndex int `json:"yAxisIndex,omitempty"`
}
