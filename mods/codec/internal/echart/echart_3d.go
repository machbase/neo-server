package echart

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/render"
)

type Base3D struct {
	ChartBase
	series             [][]opts.Chart3DData
	sereisBreakByXAxis bool

	xAxisIdx   int
	yAxisIdx   int
	zAxisIdx   int
	xAxisLabel string
	yAxisLabel string
	zAxisLabel string
	xAxisType  string
	yAxisType  string
	zAxisType  string

	opacity float64

	lineWidth float32
}

func (ex *Base3D) ContentType() string {
	if ex.toJsonOutput {
		return "application/json"
	}
	return "text/html"
}

func (ex *Base3D) Open() error {
	return nil
}

func (ex *Base3D) Flush(heading bool) {
}

func (ex *Base3D) SetXAxis(idx int, label string, typ ...string) {
	ex.xAxisIdx = idx
	ex.xAxisLabel = label
	if len(typ) > 0 {
		ex.xAxisType = typ[0]
	}
}

func (ex *Base3D) SetYAxis(idx int, label string, typ ...string) {
	ex.yAxisIdx = idx
	ex.yAxisLabel = label
	if len(typ) > 0 {
		ex.yAxisType = typ[0]
	}
}

func (ex *Base3D) SetZAxis(idx int, label string, typ ...string) {
	ex.zAxisIdx = idx
	ex.zAxisLabel = label
	if len(typ) > 0 {
		ex.zAxisType = typ[0]
	}
}

// speed angle/sec
func (ex *Base3D) SetAutoRotate(speed float64) {
	if speed < 0 {
		speed = 0
	}
	if speed > 180 {
		speed = 180
	}

	ex.globalOptions.Grid3D.ViewControl = &opts.ViewControl{
		AutoRotate:      true,
		AutoRotateSpeed: float32(speed),
	}
}

func (ex *Base3D) SetShowGrid(flag bool) {
	ex.globalOptions.Grid3D.Show = flag
}

func (ex *Base3D) SetGridSize(args ...float64) {
	widthHeightDepth := [3]float32{100, 100, 100}
	for i := 0; i < 3 && i < len(args); i++ {
		widthHeightDepth[i] = float32(args[i])
	}
	ex.globalOptions.Grid3D.BoxWidth = widthHeightDepth[0]
	ex.globalOptions.Grid3D.BoxHeight = widthHeightDepth[1]
	ex.globalOptions.Grid3D.BoxDepth = widthHeightDepth[2]
}

func (ex *Base3D) SetOpacity(opacity float64) {
	ex.opacity = opacity
}

func (ex *Base3D) SetLineWidth(width float64) {
	ex.lineWidth = float32(width)
}

func (ex *Base3D) getGlobalOptions() []charts.GlobalOpts {
	options := ex.ChartBase.getGlobalOptions()
	options = append(options,
		charts.WithLegendOpts(opts.Legend{
			Show: false,
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: true, Trigger: "axis"}),
		charts.WithXAxis3DOpts(opts.XAxis3D{Name: ex.xAxisLabel, Type: ex.xAxisType}),
		charts.WithYAxis3DOpts(opts.YAxis3D{Name: ex.yAxisLabel, Type: ex.yAxisType}),
		charts.WithZAxis3DOpts(opts.ZAxis3D{Name: ex.zAxisLabel, Type: ex.zAxisType}),
	)
	return options
}

func (ex *Base3D) AddRow(values []any) error {
	if len(values) < 3 {
		return errors.New("3D chart require  at last 3 values")
	}
	var xv float64
	var yv float64
	var zv float64

	if ex.xAxisType == "time" {
		if v, ok := values[ex.xAxisIdx].(time.Time); ok {
			xv = float64(v.UnixMilli())
		} else {
			if pv, ok := values[ex.xAxisIdx].(*time.Time); ok {
				xv = float64((*pv).UnixMilli())
			} else {
				return errors.New("3D chart requires time for x-axis")
			}
		}
	} else {
		if v, ok := ex.value(values[ex.xAxisIdx]); ok {
			xv = v
		} else {
			return errors.New("3D chart requires value for x-axis")
		}
	}

	if ex.yAxisType == "time" {
		if v, ok := values[ex.yAxisIdx].(time.Time); ok {
			yv = float64(v.UnixMilli())
		} else {
			if pv, ok := values[ex.yAxisIdx].(*time.Time); ok {
				yv = float64((*pv).UnixMilli())
			} else {
				return errors.New("3D chart requires time for y-axis")
			}
		}
	} else {
		if v, ok := ex.value(values[ex.yAxisIdx]); ok {
			yv = v
		} else {
			return errors.New("3D chart requires value for y-axis")
		}
	}

	if ex.zAxisType == "time" {
		if v, ok := values[ex.zAxisIdx].(time.Time); ok {
			zv = float64(v.UnixMilli())
		} else {
			if pv, ok := values[ex.zAxisIdx].(*time.Time); ok {
				zv = float64((*pv).UnixMilli())
			} else {
				return errors.New("3D chart requires time for z-axis")
			}
		}
	} else {
		if v, ok := ex.value(values[ex.zAxisIdx]); ok {
			zv = v
		} else {
			return errors.New("3D chart requires value for z-axis")
		}
	}

	vv := opts.Chart3DData{Value: []any{xv, yv, zv}}
	if ex.opacity > 0.0 {
		vv.ItemStyle = &opts.ItemStyle{Opacity: float32(ex.opacity)}
	}

	if ex.sereisBreakByXAxis {
		nSer := len(ex.series)
		if nSer == 0 {
			ex.series = append(ex.series, []opts.Chart3DData{})
		} else {
			ser := ex.series[nSer-1]
			prev := ser[len(ser)-1]
			if len(prev.Value) > 0 && len(values) > 0 {
				if prev.Value[0] != xv {
					ex.series = append(ex.series, []opts.Chart3DData{})
				}
			}
		}
		nSer = len(ex.series)
		ex.series[nSer-1] = append(ex.series[nSer-1], vv)
	} else {
		if len(ex.series) == 0 {
			ex.series = append(ex.series, []opts.Chart3DData{})
		}
		ex.series[0] = append(ex.series[0], vv)
	}

	return nil
}

func (ex *Base3D) value(x any) (float64, bool) {
	switch v := x.(type) {
	case int:
		return float64(v), true
	case *int:
		return float64(*v), true
	case int16:
		return float64(v), true
	case *int16:
		return float64(*v), true
	case int32:
		return float64(v), true
	case *int32:
		return float64(*v), true
	case int64:
		return float64(v), true
	case *int64:
		return float64(*v), true
	case float32:
		return float64(v), true
	case *float32:
		return float64(*v), true
	case float64:
		return v, true
	case *float64:
		return *v, true
	case time.Time:
		return float64(v.UnixMilli()), true
	case *time.Time:
		return float64((*v).UnixMilli()), true
	default:
		return 0, false
	}
}

type Line3D struct {
	Base3D
}

func NewLine3D() *Line3D {
	ret := &Line3D{
		Base3D{
			sereisBreakByXAxis: true,

			xAxisIdx:   0,
			xAxisLabel: "x",
			xAxisType:  "value",
			yAxisIdx:   1,
			yAxisLabel: "y",
			yAxisType:  "value",
			zAxisIdx:   2,
			zAxisLabel: "z",
			zAxisType:  "value",
		},
	}
	ret.initialize()
	return ret
}

func (ex *Line3D) Close() {
	line3d := charts.NewLine3D()
	line3d.SetGlobalOptions(ex.getGlobalOptions()...)
	serOpts := []charts.SeriesOpts{}
	if ex.lineWidth > 0 {
		serOpts = append(serOpts, charts.WithLineStyleOpts(
			opts.LineStyle{Width: ex.lineWidth},
		))
	}
	for _, ser := range ex.series {
		line3d.AddSeries(ex.zAxisLabel, ser, serOpts...)
	}
	var rndr render.Renderer
	if ex.toJsonOutput {
		rndr = newJsonRender(line3d, line3d.Validate)
	} else {
		rndr = newChartRender(line3d, line3d.Validate)
	}
	err := rndr.Render(ex.output)
	if err != nil {
		fmt.Println("ERR", err.Error())
	}
}

type Surface3D struct {
	Base3D
}

func NewSurface3D() *Surface3D {
	ret := &Surface3D{
		Base3D{
			sereisBreakByXAxis: true,

			xAxisIdx:   0,
			xAxisLabel: "x",
			xAxisType:  "value",
			yAxisIdx:   1,
			yAxisLabel: "y",
			yAxisType:  "value",
			zAxisIdx:   2,
			zAxisLabel: "z",
			zAxisType:  "value",
		},
	}
	ret.initialize()
	return ret
}

func (ex *Surface3D) Close() {
	surface3d := charts.NewSurface3D()
	surface3d.SetGlobalOptions(ex.getGlobalOptions()...)
	for _, ser := range ex.series {
		surface3d.AddSeries(ex.zAxisLabel, ser)
	}
	var rndr render.Renderer
	if ex.toJsonOutput {
		rndr = newJsonRender(surface3d, surface3d.Validate)
	} else {
		rndr = newChartRender(surface3d, surface3d.Validate)
	}
	err := rndr.Render(ex.output)
	if err != nil {
		fmt.Println("ERR", err.Error())
	}
}

type Scatter3D struct {
	Base3D
}

func NewScatter3D() *Scatter3D {
	ret := &Scatter3D{
		Base3D{
			sereisBreakByXAxis: true,

			xAxisIdx:   0,
			xAxisLabel: "x",
			xAxisType:  "value",
			yAxisIdx:   1,
			yAxisLabel: "y",
			yAxisType:  "value",
			zAxisIdx:   2,
			zAxisLabel: "z",
			zAxisType:  "value",
		},
	}
	ret.initialize()
	return ret
}

func (ex *Scatter3D) Close() {
	scatter3d := charts.NewScatter3D()
	scatter3d.SetGlobalOptions(ex.getGlobalOptions()...)
	for _, ser := range ex.series {
		scatter3d.AddSeries(ex.zAxisLabel, ser)
	}
	var rndr render.Renderer
	if ex.toJsonOutput {
		rndr = newJsonRender(scatter3d, scatter3d.Validate)
	} else {
		rndr = newChartRender(scatter3d, scatter3d.Validate)
	}
	err := rndr.Render(ex.output)
	if err != nil {
		fmt.Println("ERR", err.Error())
	}
}

type Bar3D struct {
	Base3D
}

func NewBar3D() *Bar3D {
	ret := &Bar3D{
		Base3D{
			sereisBreakByXAxis: false,

			xAxisIdx:   0,
			xAxisLabel: "x",
			xAxisType:  "value",
			yAxisIdx:   1,
			yAxisLabel: "y",
			yAxisType:  "value",
			zAxisIdx:   2,
			zAxisLabel: "z",
			zAxisType:  "value",
		},
	}
	ret.initialize()
	return ret
}

func (ex *Bar3D) Close() {
	bar3d := charts.NewBar3D()
	bar3d.SetGlobalOptions(ex.getGlobalOptions()...)
	for _, ser := range ex.series {
		bar3d.AddSeries(ex.zAxisLabel, ser)
	}
	var rndr render.Renderer
	if ex.toJsonOutput {
		rndr = newJsonRender(bar3d, bar3d.Validate)
	} else {
		rndr = newChartRender(bar3d, bar3d.Validate)
	}
	err := rndr.Render(ex.output)
	if err != nil {
		fmt.Println("ERR", err.Error())
	}
}
