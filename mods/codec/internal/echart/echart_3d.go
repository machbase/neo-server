package echart

import (
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
)

type Base3D struct {
	series       []opts.Chart3DData
	TimeLocation *time.Location
	Output       spec.OutputStream
	Rownum       bool
	Heading      bool
	TimeFormat   string
	Precision    int
	Title        string
	Subtitle     string
}

func (ex *Base3D) ContentType() string {
	return "text/html"
}

func (ex *Base3D) Open(cols spi.Columns) error {
	return nil
}

func (ex *Base3D) Flush(heading bool) {
}

func (ex *Base3D) getGlobalOptions() []charts.GlobalOpts {
	return []charts.GlobalOpts{
		charts.WithInitializationOpts(opts.Initialization{
			// Theme:     types.ThemeChalk,
			Height:    "900px",
			Width:     "900px",
			PageTitle: "Test-chart",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    ex.Title,
			Subtitle: ex.Subtitle,
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
		charts.WithXAxis3DOpts(opts.XAxis3D{Name: "time", Type: "time"}),
		charts.WithYAxis3DOpts(opts.YAxis3D{Name: "Hz", Type: "value"}),
		charts.WithZAxis3DOpts(opts.ZAxis3D{Name: "Amplitude", Type: "value"}),
	}
}

func (ex *Base3D) AddRow(values []any) error {
	t := values[0].(time.Time).UnixMilli()
	hz := values[1].(float64)
	amp := values[2].(float64)

	if hz > 500 {
		return nil
	}
	ex.series = append(ex.series, opts.Chart3DData{Value: []any{t, hz, amp}, ItemStyle: &opts.ItemStyle{Opacity: 0.4}})

	return nil
}

type Line3D struct {
	Base3D
}

func (ex *Line3D) Close() {
	line3d := charts.NewLine3D()
	line3d.SetGlobalOptions(ex.getGlobalOptions()...)
	line3d.AddSeries("Amplitude", ex.series)
	line3d.Render(ex.Output)

	// page := components.NewPage()
	// page.AddCharts(line3d)
	// page.Render(ex.Output)
}

type Surface3D struct {
	Base3D
}

func (ex *Surface3D) Close() {
	surface3d := charts.NewSurface3D()
	surface3d.SetGlobalOptions(ex.getGlobalOptions()...)
	surface3d.AddSeries("Amplitude", ex.series)
	surface3d.Render(ex.Output)
}

type Scatter3D struct {
	Base3D
}

func (ex *Scatter3D) Close() {
	scatter3d := charts.NewScatter3D()
	scatter3d.SetGlobalOptions(ex.getGlobalOptions()...)
	scatter3d.AddSeries("Amplitude", ex.series)
	scatter3d.Render(ex.Output)
}

type Bar3D struct {
	Base3D
}

func (ex *Bar3D) Close() {
	bar3d := charts.NewBar3D()
	bar3d.SetGlobalOptions(ex.getGlobalOptions()...)
	bar3d.AddSeries("Amplitude", ex.series)
	bar3d.Render(ex.Output)
}
