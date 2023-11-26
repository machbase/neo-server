package chart

import (
	"fmt"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/render"
)

type Base2D struct {
	ChartBase
}

func NewRectChart(defaultType string) *Base2D {
	ret := &Base2D{}
	ret.defaultChartType = defaultType
	ret.initialize()
	return ret
}

func (ex *Base2D) Open() error {
	return nil
}

func (ex *Base2D) Flush(heading bool) {
}

func (ex *Base2D) getSeriesOptions(seriesIdx int) []charts.SeriesOpts {
	var ret = []charts.SeriesOpts{}
	ret = append(ret, func(s *charts.SingleSeries) {
		if s.MarkAreas == nil {
			s.MarkAreas = &opts.MarkAreas{}
		}
		for _, mark := range ex.markAreaXAxis {
			if mark.SeriesIdx != seriesIdx {
				continue
			}
			s.MarkAreas.Data = append(s.MarkAreas.Data, mark)
		}
		for _, mark := range ex.markAreaYAxis {
			if mark.SeriesIdx != seriesIdx {
				continue
			}
			s.MarkAreas.Data = append(s.MarkAreas.Data, mark)
		}
		if s.MarkLines == nil {
			s.MarkLines = &opts.MarkLines{}
		}
		for _, mark := range ex.markLineXAxis {
			if mark.SeriesIdx != seriesIdx {
				continue
			}
			s.MarkLines.Data = append(s.MarkLines.Data, mark)
		}
		for _, mark := range ex.markLineYAxis {
			if mark.SeriesIdx != seriesIdx {
				continue
			}
			s.MarkLines.Data = append(s.MarkLines.Data, mark)
		}
	})

	return ret
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
