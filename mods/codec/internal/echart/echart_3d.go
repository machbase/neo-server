package echart

import (
	"errors"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

type Base3D struct {
	ChartBase
	series []opts.Chart3DData

	xAxisIdx   int
	yAxisIdx   int
	zAxisIdx   int
	xAxisLabel string
	yAxisLabel string
	zAxisLabel string
	xAxisType  string
	yAxisType  string
	zAxisType  string
}

func (ex *Base3D) ContentType() string {
	return "text/html"
}

func (ex *Base3D) Open() error {
	return nil
}

func (ex *Base3D) Flush(heading bool) {
}

func (ex *Base3D) SetXAxis(idx int, label string, typ string) {
	ex.xAxisIdx = idx
	ex.xAxisLabel = label
	ex.xAxisType = typ
}

func (ex *Base3D) SetYAxis(idx int, label string, typ string) {
	ex.yAxisIdx = idx
	ex.yAxisLabel = label
	ex.yAxisType = typ
}

func (ex *Base3D) SetZAxis(idx int, label string, typ string) {
	ex.zAxisIdx = idx
	ex.zAxisLabel = label
	ex.zAxisType = typ
}

func (ex *Base3D) getGlobalOptions() []charts.GlobalOpts {
	width := "600px"
	if ex.width != "" {
		width = ex.width
	}
	height := "400px"
	if ex.height != "" {
		height = ex.height
	}
	title := ""
	if ex.title != "" {
		title = ex.title
	}
	subtitle := ""
	if ex.subtitle != "" {
		subtitle = ex.subtitle
	}
	theme := ex.theme
	if theme == "" {
		theme = types.ThemeWesteros
	}
	return []charts.GlobalOpts{
		charts.WithInitializationOpts(opts.Initialization{
			Theme:     theme,
			Width:     width,
			Height:    height,
			PageTitle: title,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    title,
			Subtitle: subtitle,
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: false,
		}),
		charts.WithVisualMapOpts(opts.VisualMap{
			InRange: &opts.VisualMapInRange{
				Color: []string{
					"#313695",
					"#4575b4",
					"#74add1",
					"#abd9e9",
					"#e0f3f8",
					"#ffffbf",
					"#fee090",
					"#fdae61",
					"#f46d43",
					"#d73027",
					"#a50026",
				},
			},
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: true, Trigger: "axis"}),
		charts.WithGrid3DOpts(opts.Grid3D{Show: true}),
		charts.WithXAxis3DOpts(opts.XAxis3D{Name: ex.xAxisLabel, Type: ex.xAxisType}),
		charts.WithYAxis3DOpts(opts.YAxis3D{Name: ex.yAxisLabel, Type: ex.yAxisType}),
		charts.WithZAxis3DOpts(opts.ZAxis3D{Name: ex.zAxisLabel, Type: ex.zAxisType}),
	}
}

func (ex *Base3D) AddRow(values []any) error {
	if len(values) < 3 {
		return errors.New("3D chart require  at last 3 vlaues")
	}
	var xv time.Time
	var yv float64
	var zv float64

	if v, ok := values[ex.xAxisIdx].(time.Time); ok {
		xv = v
	} else {
		if pv, ok := values[ex.xAxisIdx].(*time.Time); ok {
			xv = *pv
		} else {
			return errors.New("3D chart requires time.Time value for x-axis")
		}
	}
	if v, ok := ex.value(values[ex.yAxisIdx]); ok {
		yv = v
	} else {
		return errors.New("3D chart requires float64 value for y-axis")
	}

	if v, ok := ex.value(values[ex.zAxisIdx]); ok {
		zv = v
	} else {
		return errors.New("3D chart requires float64 value for z-axis")
	}

	ex.series = append(ex.series, opts.Chart3DData{Value: []any{xv.UnixMilli(), yv, zv}, ItemStyle: &opts.ItemStyle{Opacity: 0.4}})

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
	default:
		return 0, false
	}
}

type Line3D struct {
	Base3D
}

func NewLine3D() *Line3D {
	return &Line3D{
		Base3D{
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
}

func (ex *Line3D) Close() {
	line3d := charts.NewLine3D()
	line3d.SetGlobalOptions(ex.getGlobalOptions()...)
	line3d.AddSeries(ex.zAxisLabel, ex.series)
	line3d.Render(ex.output)

	// page := components.NewPage()
	// page.AddCharts(line3d)
	// page.Render(ex.Output)
}

type Surface3D struct {
	Base3D
}

func NewSurface3D() *Surface3D {
	return &Surface3D{
		Base3D{
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
}

func (ex *Surface3D) Close() {
	surface3d := charts.NewSurface3D()
	surface3d.SetGlobalOptions(ex.getGlobalOptions()...)
	surface3d.AddSeries(ex.zAxisLabel, ex.series)
	surface3d.Render(ex.output)
}

type Scatter3D struct {
	Base3D
}

func NewScatter3D() *Scatter3D {
	return &Scatter3D{
		Base3D{
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
}

func (ex *Scatter3D) Close() {
	scatter3d := charts.NewScatter3D()
	scatter3d.SetGlobalOptions(ex.getGlobalOptions()...)
	scatter3d.AddSeries(ex.zAxisLabel, ex.series)
	scatter3d.Render(ex.output)
}

type Bar3D struct {
	Base3D
}

func NewBar3D() *Bar3D {
	return &Bar3D{
		Base3D{
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
}

func (ex *Bar3D) Close() {
	bar3d := charts.NewBar3D()
	bar3d.SetGlobalOptions(ex.getGlobalOptions()...)
	bar3d.AddSeries(ex.zAxisLabel, ex.series)
	bar3d.Render(ex.output)
}
