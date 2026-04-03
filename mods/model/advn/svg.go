package advn

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	svgDefaultWidth      = 960
	svgDefaultHeight     = 420
	svgDefaultPadding    = 48
	svgDefaultFontSize   = 12
	svgDefaultFontFamily = "sans-serif"
	svgDefaultBackground = "white"
)

type SVGOptions struct {
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Padding    int    `json:"padding,omitempty"`
	Background string `json:"background,omitempty"`
	FontFamily string `json:"fontFamily,omitempty"`
	FontSize   int    `json:"fontSize,omitempty"`
	ShowLegend *bool  `json:"showLegend,omitempty"`
	Title      string `json:"title,omitempty"`
}

type svgResolvedOptions struct {
	Width      int
	Height     int
	Padding    int
	Background string
	FontFamily string
	FontSize   int
	ShowLegend bool
	Title      string
}

type svgRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type svgXScale struct {
	Kind       string
	Domain     Domain
	Min        float64
	Max        float64
	Categories []string
	CategoryIx map[string]int
	Rect       svgRect
}

type svgYScale struct {
	AxisID string
	Min    float64
	Max    float64
	Rect   svgRect
}

type svgLayout struct {
	Options        svgResolvedOptions
	Canvas         svgRect
	Plot           svgRect
	XAxis          svgXScale
	YAxes          map[string]svgYScale
	AxisOrder      []string
	AxisPlacements map[string]svgAxisPlacement
	PrimaryYAxisID string
	LegendItems    []svgLegendItem
	LegendEntries  []svgLegendEntry
	TitleY         float64
	LegendRect     svgRect
}

type svgLegendItem struct {
	Label string
	Color string
}

type svgAxisPlacement struct {
	AxisID      string
	Side        string
	LineX       float64
	TickLabelX  float64
	LabelX      float64
	TickAnchor  string
	TickDir     float64
	LabelAnchor string
}

type svgLegendEntry struct {
	Row   int
	Item  svgLegendItem
	BoxX  float64
	TextX float64
	Y     float64
}

func ToSVG(spec *Spec, options *SVGOptions) ([]byte, error) {
	if spec == nil {
		return nil, fmt.Errorf("advn: spec is nil")
	}
	resolved, err := normalizeSVGOptions(options)
	if err != nil {
		return nil, err
	}
	spec = spec.Normalize()
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	layout, err := buildSVGLayout(spec, resolved)
	if err != nil {
		return nil, err
	}
	ret, err := renderSVGDocument(spec, layout)
	if err != nil {
		return nil, err
	}
	return []byte(ret), nil
}

func normalizeSVGOptions(options *SVGOptions) (svgResolvedOptions, error) {
	resolved := svgResolvedOptions{
		Width:      svgDefaultWidth,
		Height:     svgDefaultHeight,
		Padding:    svgDefaultPadding,
		Background: svgDefaultBackground,
		FontFamily: svgDefaultFontFamily,
		FontSize:   svgDefaultFontSize,
		ShowLegend: true,
	}
	if options == nil {
		return resolved, nil
	}
	if options.Width != 0 {
		resolved.Width = options.Width
	}
	if options.Height != 0 {
		resolved.Height = options.Height
	}
	if options.Padding != 0 {
		resolved.Padding = options.Padding
	}
	if options.Background != "" {
		resolved.Background = options.Background
	}
	if options.FontFamily != "" {
		resolved.FontFamily = options.FontFamily
	}
	if options.FontSize != 0 {
		resolved.FontSize = options.FontSize
	}
	if options.ShowLegend != nil {
		resolved.ShowLegend = *options.ShowLegend
	}
	resolved.Title = options.Title
	if resolved.Width <= 0 {
		return svgResolvedOptions{}, fmt.Errorf("advn: svg width must be greater than 0")
	}
	if resolved.Height <= 0 {
		return svgResolvedOptions{}, fmt.Errorf("advn: svg height must be greater than 0")
	}
	if resolved.Padding < 0 {
		return svgResolvedOptions{}, fmt.Errorf("advn: svg padding must be 0 or greater")
	}
	if resolved.FontSize <= 0 {
		return svgResolvedOptions{}, fmt.Errorf("advn: svg fontSize must be greater than 0")
	}
	return resolved, nil
}

func buildSVGLayout(spec *Spec, options svgResolvedOptions) (svgLayout, error) {
	canvas := svgRect{Width: float64(options.Width), Height: float64(options.Height)}
	axisOrder := orderedSVGYAxisIDs(spec)
	leftAxisCount, rightAxisCount := svgAxisSideCounts(axisOrder)
	titleHeight := 0.0
	if options.Title != "" {
		titleHeight = float64(options.FontSize) + 18
	}
	legendItems := buildSVGLegendItems(spec, options.ShowLegend)
	leftMargin := float64(options.Padding) + 12 + float64(maxInt(leftAxisCount-1, 0))*44
	rightMargin := float64(options.Padding) + 12 + float64(rightAxisCount)*44
	plotWidth := float64(options.Width) - leftMargin - rightMargin
	if plotWidth <= 0 {
		return svgLayout{}, fmt.Errorf("advn: svg plot area is too small")
	}
	legendEntries, legendHeight := layoutSVGLegendEntries(legendItems, leftMargin, plotWidth, float64(options.FontSize))
	xLabelHeight := float64(options.FontSize)*2 + 16
	plotTopInset := math.Max(6, float64(options.FontSize)*0.75)
	plot := svgRect{
		X:      leftMargin,
		Y:      float64(options.Padding) + titleHeight + plotTopInset,
		Width:  plotWidth,
		Height: float64(options.Height) - (float64(options.Padding) + titleHeight) - float64(options.Padding) - xLabelHeight - legendHeight - plotTopInset,
	}
	if plot.Width <= 0 || plot.Height <= 0 {
		return svgLayout{}, fmt.Errorf("advn: svg plot area is too small")
	}
	xScale, err := buildSVGXScale(spec, plot)
	if err != nil {
		return svgLayout{}, err
	}
	yAxes := buildSVGYScales(spec, plot)
	primaryYAxisID := "y"
	if len(spec.Axes.Y) > 0 && spec.Axes.Y[0].ID != "" {
		primaryYAxisID = spec.Axes.Y[0].ID
	}
	if _, ok := yAxes[primaryYAxisID]; !ok {
		for axisID := range yAxes {
			primaryYAxisID = axisID
			break
		}
	}
	if _, ok := yAxes[primaryYAxisID]; !ok {
		yAxes[primaryYAxisID] = svgYScale{AxisID: primaryYAxisID, Min: 0, Max: 1, Rect: plot}
	}
	axisPlacements := svgBuildAxisPlacements(axisOrder, plot)
	legendRect := svgRect{}
	if legendHeight > 0 {
		legendRect = svgRect{
			X:      plot.X,
			Y:      plot.Y + plot.Height + xLabelHeight,
			Width:  plot.Width,
			Height: legendHeight,
		}
	}
	return svgLayout{
		Options:        options,
		Canvas:         canvas,
		Plot:           plot,
		XAxis:          xScale,
		YAxes:          yAxes,
		AxisOrder:      axisOrder,
		AxisPlacements: axisPlacements,
		PrimaryYAxisID: primaryYAxisID,
		LegendItems:    legendItems,
		LegendEntries:  legendEntries,
		TitleY:         float64(options.Padding) + float64(options.FontSize),
		LegendRect:     legendRect,
	}, nil
}

func renderSVGDocument(spec *Spec, layout svgLayout) (string, error) {
	var body strings.Builder
	writeSVGBackground(&body, layout)
	writeSVGTitle(&body, layout)
	writeSVGAxes(&body, spec, layout)
	body.WriteString("<g data-advn-role=\"series\" clip-path=\"url(#plot-clip)\">")
	for index, series := range spec.Series {
		if err := writeSVGSeries(&body, spec, layout, series, index); err != nil {
			return "", err
		}
	}
	body.WriteString("</g>")
	writeSVGAnnotations(&body, spec, layout)
	writeSVGLegend(&body, layout)

	var svg strings.Builder
	svg.WriteString("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"")
	svg.WriteString(strconv.Itoa(layout.Options.Width))
	svg.WriteString("\" height=\"")
	svg.WriteString(strconv.Itoa(layout.Options.Height))
	svg.WriteString("\" viewBox=\"0 0 ")
	svg.WriteString(strconv.Itoa(layout.Options.Width))
	svg.WriteString(" ")
	svg.WriteString(strconv.Itoa(layout.Options.Height))
	svg.WriteString("\" font-family=\"")
	svg.WriteString(escapeSVG(layout.Options.FontFamily))
	svg.WriteString("\" font-size=\"")
	svg.WriteString(strconv.Itoa(layout.Options.FontSize))
	svg.WriteString("\">")
	svg.WriteString("<defs><clipPath id=\"plot-clip\"><rect x=\"")
	svg.WriteString(svgNumber(layout.Plot.X))
	svg.WriteString("\" y=\"")
	svg.WriteString(svgNumber(layout.Plot.Y))
	svg.WriteString("\" width=\"")
	svg.WriteString(svgNumber(layout.Plot.Width))
	svg.WriteString("\" height=\"")
	svg.WriteString(svgNumber(layout.Plot.Height))
	svg.WriteString("\" /></clipPath></defs>")
	svg.WriteString(body.String())
	svg.WriteString("</svg>")
	return svg.String(), nil
}

func writeSVGBackground(builder *strings.Builder, layout svgLayout) {
	builder.WriteString("<g data-advn-role=\"background\"><rect x=\"0\" y=\"0\" width=\"")
	builder.WriteString(strconv.Itoa(layout.Options.Width))
	builder.WriteString("\" height=\"")
	builder.WriteString(strconv.Itoa(layout.Options.Height))
	builder.WriteString("\" fill=\"")
	builder.WriteString(escapeSVG(layout.Options.Background))
	builder.WriteString("\" /></g>")
}

func writeSVGTitle(builder *strings.Builder, layout svgLayout) {
	if layout.Options.Title == "" {
		return
	}
	builder.WriteString("<g data-advn-role=\"title\"><text x=\"")
	builder.WriteString(svgNumber(layout.Plot.X))
	builder.WriteString("\" y=\"")
	builder.WriteString(svgNumber(layout.TitleY))
	builder.WriteString("\" font-size=\"")
	builder.WriteString(strconv.Itoa(layout.Options.FontSize + 2))
	builder.WriteString("\" font-weight=\"600\">")
	builder.WriteString(escapeSVG(layout.Options.Title))
	builder.WriteString("</text></g>")
}

func writeSVGAxes(builder *strings.Builder, spec *Spec, layout svgLayout) {
	builder.WriteString("<g data-advn-role=\"axes\">")
	builder.WriteString("<rect x=\"")
	builder.WriteString(svgNumber(layout.Plot.X))
	builder.WriteString("\" y=\"")
	builder.WriteString(svgNumber(layout.Plot.Y))
	builder.WriteString("\" width=\"")
	builder.WriteString(svgNumber(layout.Plot.Width))
	builder.WriteString("\" height=\"")
	builder.WriteString(svgNumber(layout.Plot.Height))
	builder.WriteString("\" fill=\"none\" stroke=\"#d0d7de\" />")
	writeSVGXAxis(builder, spec, layout)
	writeSVGYAxes(builder, spec, layout)
	builder.WriteString("</g>")
}

func writeSVGXAxis(builder *strings.Builder, spec *Spec, layout svgLayout) {
	y := layout.Plot.Y + layout.Plot.Height
	builder.WriteString("<line x1=\"")
	builder.WriteString(svgNumber(layout.Plot.X))
	builder.WriteString("\" y1=\"")
	builder.WriteString(svgNumber(y))
	builder.WriteString("\" x2=\"")
	builder.WriteString(svgNumber(layout.Plot.X + layout.Plot.Width))
	builder.WriteString("\" y2=\"")
	builder.WriteString(svgNumber(y))
	builder.WriteString("\" stroke=\"#6e7781\" />")
	if layout.XAxis.Kind == AxisTypeCategory {
		for _, tick := range svgCategoryTicks(layout.XAxis) {
			builder.WriteString("<line x1=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y1=\"")
			builder.WriteString(svgNumber(y))
			builder.WriteString("\" x2=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y2=\"")
			builder.WriteString(svgNumber(y + 5))
			builder.WriteString("\" stroke=\"#6e7781\" />")
			builder.WriteString("<text x=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(y + float64(layout.Options.FontSize) + 8))
			builder.WriteString("\" text-anchor=\"middle\" fill=\"#24292f\">")
			builder.WriteString(escapeSVG(tick.Label))
			builder.WriteString("</text>")
		}
	} else {
		for _, tick := range svgContinuousTicks(layout.XAxis) {
			builder.WriteString("<line x1=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y1=\"")
			builder.WriteString(svgNumber(layout.Plot.Y))
			builder.WriteString("\" x2=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y2=\"")
			builder.WriteString(svgNumber(y))
			builder.WriteString("\" stroke=\"#eaeef2\" />")
			builder.WriteString("<text x=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(y + float64(layout.Options.FontSize) + 8))
			builder.WriteString("\" text-anchor=\"middle\" fill=\"#57606a\">")
			builder.WriteString(escapeSVG(tick.Label))
			builder.WriteString("</text>")
		}
	}
	label := spec.Axes.X.Label
	if label == "" {
		label = spec.Axes.X.ID
	}
	if label != "" {
		builder.WriteString("<text x=\"")
		builder.WriteString(svgNumber(layout.Plot.X + layout.Plot.Width/2))
		builder.WriteString("\" y=\"")
		builder.WriteString(svgNumber(y + float64(layout.Options.FontSize)*2 + 12))
		builder.WriteString("\" text-anchor=\"middle\" fill=\"#24292f\">")
		builder.WriteString(escapeSVG(label))
		builder.WriteString("</text>")
	}
}

func writeSVGYAxes(builder *strings.Builder, spec *Spec, layout svgLayout) {
	if len(layout.AxisOrder) == 0 {
		layout.AxisOrder = []string{layout.PrimaryYAxisID}
	}
	for _, axisID := range layout.AxisOrder {
		scale, ok := layout.YAxes[axisID]
		if !ok {
			continue
		}
		placement := layout.AxisPlacements[axisID]
		isPrimary := axisID == layout.PrimaryYAxisID
		builder.WriteString("<g data-advn-axis=\"")
		builder.WriteString(escapeSVG(axisID))
		builder.WriteString("\" data-advn-side=\"")
		builder.WriteString(placement.Side)
		builder.WriteString("\">")
		builder.WriteString("<line x1=\"")
		builder.WriteString(svgNumber(placement.LineX))
		builder.WriteString("\" y1=\"")
		builder.WriteString(svgNumber(layout.Plot.Y))
		builder.WriteString("\" x2=\"")
		builder.WriteString(svgNumber(placement.LineX))
		builder.WriteString("\" y2=\"")
		builder.WriteString(svgNumber(layout.Plot.Y + layout.Plot.Height))
		builder.WriteString("\" stroke=\"#6e7781\" />")
		for _, tick := range svgYTicks(scale) {
			if isPrimary {
				builder.WriteString("<line x1=\"")
				builder.WriteString(svgNumber(layout.Plot.X))
				builder.WriteString("\" y1=\"")
				builder.WriteString(svgNumber(tick.Y))
				builder.WriteString("\" x2=\"")
				builder.WriteString(svgNumber(layout.Plot.X + layout.Plot.Width))
				builder.WriteString("\" y2=\"")
				builder.WriteString(svgNumber(tick.Y))
				builder.WriteString("\" stroke=\"#eaeef2\" />")
			}
			builder.WriteString("<line x1=\"")
			builder.WriteString(svgNumber(placement.LineX))
			builder.WriteString("\" y1=\"")
			builder.WriteString(svgNumber(tick.Y))
			builder.WriteString("\" x2=\"")
			builder.WriteString(svgNumber(placement.LineX + placement.TickDir*5))
			builder.WriteString("\" y2=\"")
			builder.WriteString(svgNumber(tick.Y))
			builder.WriteString("\" stroke=\"#6e7781\" />")
			builder.WriteString("<text x=\"")
			builder.WriteString(svgNumber(placement.TickLabelX))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(tick.Y + float64(layout.Options.FontSize)/3))
			builder.WriteString("\" text-anchor=\"")
			builder.WriteString(placement.TickAnchor)
			builder.WriteString("\" fill=\"#57606a\">")
			builder.WriteString(escapeSVG(tick.Label))
			builder.WriteString("</text>")
		}
		label := svgAxisLabel(spec, axisID)
		if label != "" {
			builder.WriteString("<text x=\"")
			builder.WriteString(svgNumber(placement.LabelX))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(layout.Plot.Y + layout.Plot.Height/2))
			builder.WriteString("\" transform=\"rotate(-90 ")
			builder.WriteString(svgNumber(placement.LabelX))
			builder.WriteString(" ")
			builder.WriteString(svgNumber(layout.Plot.Y + layout.Plot.Height/2))
			builder.WriteString(")\" text-anchor=\"")
			builder.WriteString(placement.LabelAnchor)
			builder.WriteString("\" fill=\"#24292f\">")
			builder.WriteString(escapeSVG(label))
			builder.WriteString("</text>")
		}
		builder.WriteString("</g>")
	}
}

func writeSVGSeries(builder *strings.Builder, spec *Spec, layout svgLayout, series Series, index int) error {
	color := svgSeriesColor(series, index)
	lineColor := styleString(series.Style, "lineColor", color)
	bandColor := styleString(series.Style, "bandColor", color)
	opacity := styleFloat(series.Style, "opacity", 0.2)
	lineWidth := styleFloat(series.Style, "lineWidth", 2)
	builder.WriteString("<g data-advn-series=\"")
	builder.WriteString(escapeSVG(series.ID))
	builder.WriteString("\">")
	switch series.Representation.Kind {
	case RepresentationRawPoint, RepresentationTimeBucketValue:
		points := svgLinePoints(series, spec.Domain, layout.XAxis, layout.YAxes[svgSeriesAxisID(series, layout.PrimaryYAxisID)])
		if len(points) > 1 {
			builder.WriteString("<path d=\"")
			builder.WriteString(svgPath(points))
			builder.WriteString("\" fill=\"none\" stroke=\"")
			builder.WriteString(escapeSVG(lineColor))
			builder.WriteString("\" stroke-width=\"")
			builder.WriteString(svgNumber(lineWidth))
			builder.WriteString("\" />")
		}
	case RepresentationTimeBucketBand:
		minPoints, maxPoints, avgPoints := svgBandPoints(series, spec.Domain, layout.XAxis, layout.YAxes[svgSeriesAxisID(series, layout.PrimaryYAxisID)])
		if len(minPoints) > 1 && len(maxPoints) > 1 {
			builder.WriteString("<path d=\"")
			builder.WriteString(svgClosedBandPath(minPoints, maxPoints))
			builder.WriteString("\" fill=\"")
			builder.WriteString(escapeSVG(bandColor))
			builder.WriteString("\" fill-opacity=\"")
			builder.WriteString(svgNumber(opacity))
			builder.WriteString("\" stroke=\"none\" />")
		}
		if len(avgPoints) > 1 {
			builder.WriteString("<path d=\"")
			builder.WriteString(svgPath(avgPoints))
			builder.WriteString("\" fill=\"none\" stroke=\"")
			builder.WriteString(escapeSVG(lineColor))
			builder.WriteString("\" stroke-width=\"")
			builder.WriteString(svgNumber(lineWidth))
			builder.WriteString("\" />")
		}
	case RepresentationDistributionHistogram:
		for _, bar := range svgHistogramBars(series, layout.XAxis, layout.YAxes[svgSeriesAxisID(series, layout.PrimaryYAxisID)]) {
			builder.WriteString("<rect x=\"")
			builder.WriteString(svgNumber(bar.X))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(bar.Y))
			builder.WriteString("\" width=\"")
			builder.WriteString(svgNumber(bar.Width))
			builder.WriteString("\" height=\"")
			builder.WriteString(svgNumber(bar.Height))
			builder.WriteString("\" fill=\"")
			builder.WriteString(escapeSVG(color))
			builder.WriteString("\" fill-opacity=\"")
			builder.WriteString(svgNumber(maxFloat(opacity, 0.45)))
			builder.WriteString("\" />")
		}
	case RepresentationDistributionBoxplot:
		for _, shape := range svgBoxplotShapes(series, layout.XAxis, layout.YAxes[svgSeriesAxisID(series, layout.PrimaryYAxisID)], color, lineColor) {
			builder.WriteString(shape)
		}
	case RepresentationEventPoint:
		for _, point := range svgEventPoints(series, spec.Domain, layout.XAxis, layout.YAxes[svgSeriesAxisID(series, layout.PrimaryYAxisID)]) {
			builder.WriteString("<circle cx=\"")
			builder.WriteString(svgNumber(point.X))
			builder.WriteString("\" cy=\"")
			builder.WriteString(svgNumber(point.Y))
			builder.WriteString("\" r=\"3.5\" fill=\"")
			builder.WriteString(escapeSVG(color))
			builder.WriteString("\" />")
		}
	case RepresentationEventRange:
		for _, rect := range svgEventRangeRects(series, spec.Domain, layout.XAxis, layout.Plot) {
			builder.WriteString("<rect x=\"")
			builder.WriteString(svgNumber(rect.X))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(rect.Y))
			builder.WriteString("\" width=\"")
			builder.WriteString(svgNumber(rect.Width))
			builder.WriteString("\" height=\"")
			builder.WriteString(svgNumber(rect.Height))
			builder.WriteString("\" fill=\"")
			builder.WriteString(escapeSVG(color))
			builder.WriteString("\" fill-opacity=\"")
			builder.WriteString(svgNumber(opacity))
			builder.WriteString("\" />")
		}
	default:
		return fmt.Errorf("advn: unsupported svg representation %q", series.Representation.Kind)
	}
	builder.WriteString("</g>")
	return nil
}

func writeSVGAnnotations(builder *strings.Builder, spec *Spec, layout svgLayout) {
	if len(spec.Annotations) == 0 {
		return
	}
	builder.WriteString("<g data-advn-role=\"annotations\">")
	for _, annotation := range spec.Annotations {
		color := styleString(annotation.Style, "color", "#cf222e")
		opacity := styleFloat(annotation.Style, "opacity", 0.18)
		switch annotation.Kind {
		case AnnotationKindLine:
			if isXAxis(spec, annotation.Axis) {
				x, ok := svgMapXValue(annotation.Value, layout.XAxis)
				if !ok {
					continue
				}
				builder.WriteString("<line x1=\"")
				builder.WriteString(svgNumber(x))
				builder.WriteString("\" y1=\"")
				builder.WriteString(svgNumber(layout.Plot.Y))
				builder.WriteString("\" x2=\"")
				builder.WriteString(svgNumber(x))
				builder.WriteString("\" y2=\"")
				builder.WriteString(svgNumber(layout.Plot.Y + layout.Plot.Height))
				builder.WriteString("\" stroke=\"")
				builder.WriteString(escapeSVG(color))
				builder.WriteString("\" stroke-dasharray=\"4 4\" />")
			} else {
				scale := layout.YAxes[svgAnnotationAxisID(annotation, layout.PrimaryYAxisID)]
				y, ok := svgMapYValue(annotation.Value, scale)
				if !ok {
					continue
				}
				builder.WriteString("<line x1=\"")
				builder.WriteString(svgNumber(layout.Plot.X))
				builder.WriteString("\" y1=\"")
				builder.WriteString(svgNumber(y))
				builder.WriteString("\" x2=\"")
				builder.WriteString(svgNumber(layout.Plot.X + layout.Plot.Width))
				builder.WriteString("\" y2=\"")
				builder.WriteString(svgNumber(y))
				builder.WriteString("\" stroke=\"")
				builder.WriteString(escapeSVG(color))
				builder.WriteString("\" stroke-dasharray=\"4 4\" />")
			}
		case AnnotationKindRange:
			if isXAxis(spec, annotation.Axis) {
				from, fromOK := svgMapXValue(annotation.From, layout.XAxis)
				to, toOK := svgMapXValue(annotation.To, layout.XAxis)
				if !fromOK || !toOK {
					continue
				}
				x := math.Min(from, to)
				w := math.Abs(to - from)
				builder.WriteString("<rect x=\"")
				builder.WriteString(svgNumber(x))
				builder.WriteString("\" y=\"")
				builder.WriteString(svgNumber(layout.Plot.Y))
				builder.WriteString("\" width=\"")
				builder.WriteString(svgNumber(w))
				builder.WriteString("\" height=\"")
				builder.WriteString(svgNumber(layout.Plot.Height))
				builder.WriteString("\" fill=\"")
				builder.WriteString(escapeSVG(color))
				builder.WriteString("\" fill-opacity=\"")
				builder.WriteString(svgNumber(opacity))
				builder.WriteString("\" />")
			} else {
				scale := layout.YAxes[svgAnnotationAxisID(annotation, layout.PrimaryYAxisID)]
				from, fromOK := svgMapYValue(annotation.From, scale)
				to, toOK := svgMapYValue(annotation.To, scale)
				if !fromOK || !toOK {
					continue
				}
				y := math.Min(from, to)
				h := math.Abs(to - from)
				builder.WriteString("<rect x=\"")
				builder.WriteString(svgNumber(layout.Plot.X))
				builder.WriteString("\" y=\"")
				builder.WriteString(svgNumber(y))
				builder.WriteString("\" width=\"")
				builder.WriteString(svgNumber(layout.Plot.Width))
				builder.WriteString("\" height=\"")
				builder.WriteString(svgNumber(h))
				builder.WriteString("\" fill=\"")
				builder.WriteString(escapeSVG(color))
				builder.WriteString("\" fill-opacity=\"")
				builder.WriteString(svgNumber(opacity))
				builder.WriteString("\" />")
			}
		case AnnotationKindPoint:
			x, xOK := svgMapXValue(annotation.At, layout.XAxis)
			y, yOK := svgMapYValue(annotation.Value, layout.YAxes[svgAnnotationAxisID(annotation, layout.PrimaryYAxisID)])
			if !xOK || !yOK {
				continue
			}
			builder.WriteString("<circle cx=\"")
			builder.WriteString(svgNumber(x))
			builder.WriteString("\" cy=\"")
			builder.WriteString(svgNumber(y))
			builder.WriteString("\" r=\"4\" fill=\"")
			builder.WriteString(escapeSVG(color))
			builder.WriteString("\" />")
		}
	}
	builder.WriteString("</g>")
}

func writeSVGLegend(builder *strings.Builder, layout svgLayout) {
	if len(layout.LegendEntries) == 0 {
		return
	}
	builder.WriteString("<g data-advn-role=\"legend\">")
	currentRow := -1
	for _, entry := range layout.LegendEntries {
		if entry.Row != currentRow {
			if currentRow >= 0 {
				builder.WriteString("</g>")
			}
			currentRow = entry.Row
			builder.WriteString("<g data-advn-legend-row=\"")
			builder.WriteString(strconv.Itoa(currentRow))
			builder.WriteString("\">")
		}
		builder.WriteString("<rect x=\"")
		builder.WriteString(svgNumber(entry.BoxX))
		builder.WriteString("\" y=\"")
		builder.WriteString(svgNumber(entry.Y))
		builder.WriteString("\" width=\"10\" height=\"10\" fill=\"")
		builder.WriteString(escapeSVG(entry.Item.Color))
		builder.WriteString("\" />")
		builder.WriteString("<text x=\"")
		builder.WriteString(svgNumber(entry.TextX))
		builder.WriteString("\" y=\"")
		builder.WriteString(svgNumber(entry.Y + 9))
		builder.WriteString("\" fill=\"#24292f\">")
		builder.WriteString(escapeSVG(entry.Item.Label))
		builder.WriteString("</text>")
	}
	if currentRow >= 0 {
		builder.WriteString("</g>")
	}
	builder.WriteString("</g>")
}

func buildSVGLegendItems(spec *Spec, enabled bool) []svgLegendItem {
	if !enabled {
		return nil
	}
	items := make([]svgLegendItem, 0, len(spec.Series))
	for index, series := range spec.Series {
		label := series.Name
		if label == "" {
			label = series.ID
		}
		if label == "" {
			continue
		}
		items = append(items, svgLegendItem{Label: label, Color: svgSeriesColor(series, index)})
	}
	return items
}

func buildSVGXScale(spec *Spec, plot svgRect) (svgXScale, error) {
	kind := svgXAxisKind(spec)
	scale := svgXScale{Kind: kind, Domain: spec.Domain, Rect: plot}
	switch kind {
	case AxisTypeCategory:
		categories := svgCollectCategories(spec)
		if len(categories) == 0 {
			categories = []string{"value"}
		}
		scale.Categories = categories
		scale.CategoryIx = map[string]int{}
		for index, category := range categories {
			scale.CategoryIx[category] = index
		}
		return scale, nil
	case AxisTypeTime:
		minValue, maxValue, ok := svgCollectXContinuousDomain(spec, true)
		if !ok {
			return svgXScale{}, fmt.Errorf("advn: unable to resolve svg time domain")
		}
		scale.Min = minValue
		scale.Max = maxValue
		return scale, nil
	default:
		minValue, maxValue, ok := svgCollectXContinuousDomain(spec, false)
		if !ok {
			minValue, maxValue = 0, 1
		}
		scale.Min = minValue
		scale.Max = maxValue
		return scale, nil
	}
}

func buildSVGYScales(spec *Spec, plot svgRect) map[string]svgYScale {
	axisIDs := map[string]struct{}{}
	for _, axis := range spec.Axes.Y {
		if axis.ID != "" {
			axisIDs[axis.ID] = struct{}{}
		}
	}
	for _, series := range spec.Series {
		axisIDs[svgSeriesAxisID(series, "y")] = struct{}{}
	}
	if len(axisIDs) == 0 {
		axisIDs["y"] = struct{}{}
	}
	ret := map[string]svgYScale{}
	for axisID := range axisIDs {
		minValue, maxValue, ok := svgCollectYDomain(spec, axisID)
		if !ok {
			minValue, maxValue = 0, 1
		}
		ret[axisID] = svgYScale{AxisID: axisID, Min: minValue, Max: maxValue, Rect: plot}
	}
	return ret
}

func orderedSVGYAxisIDs(spec *Spec) []string {
	ret := []string{}
	seen := map[string]struct{}{}
	add := func(axisID string) {
		if axisID == "" {
			axisID = "y"
		}
		if _, ok := seen[axisID]; ok {
			return
		}
		seen[axisID] = struct{}{}
		ret = append(ret, axisID)
	}
	for _, axis := range spec.Axes.Y {
		add(axis.ID)
	}
	for _, series := range spec.Series {
		add(series.Axis)
	}
	for _, annotation := range spec.Annotations {
		if !isXAxis(spec, annotation.Axis) {
			add(annotation.Axis)
		}
	}
	if len(ret) == 0 {
		ret = append(ret, "y")
	}
	return ret
}

func svgAxisSideCounts(axisOrder []string) (int, int) {
	left := 0
	right := 0
	for index := range axisOrder {
		if index%2 == 0 {
			left++
		} else {
			right++
		}
	}
	if left == 0 {
		left = 1
	}
	return left, right
}

func svgBuildAxisPlacements(axisOrder []string, plot svgRect) map[string]svgAxisPlacement {
	ret := map[string]svgAxisPlacement{}
	leftSlot := 0
	rightSlot := 0
	for index, axisID := range axisOrder {
		if index%2 == 0 {
			lineX := plot.X - float64(leftSlot)*44
			ret[axisID] = svgAxisPlacement{
				AxisID:      axisID,
				Side:        "left",
				LineX:       lineX,
				TickLabelX:  lineX - 8,
				LabelX:      lineX - 24,
				TickAnchor:  "end",
				TickDir:     -1,
				LabelAnchor: "middle",
			}
			leftSlot++
		} else {
			lineX := plot.X + plot.Width + float64(rightSlot)*44
			ret[axisID] = svgAxisPlacement{
				AxisID:      axisID,
				Side:        "right",
				LineX:       lineX,
				TickLabelX:  lineX + 8,
				LabelX:      lineX + 24,
				TickAnchor:  "start",
				TickDir:     1,
				LabelAnchor: "middle",
			}
			rightSlot++
		}
	}
	return ret
}

func svgAxisLabel(spec *Spec, axisID string) string {
	for _, axis := range spec.Axes.Y {
		if axis.ID == axisID {
			if axis.Label != "" {
				return axis.Label
			}
			return axis.ID
		}
	}
	return axisID
}

func layoutSVGLegendEntries(items []svgLegendItem, startX float64, width float64, fontSize float64) ([]svgLegendEntry, float64) {
	if len(items) == 0 {
		return nil, 0
	}
	entries := make([]svgLegendEntry, 0, len(items))
	row := 0
	x := startX
	y := 4.0
	maxX := startX + width
	rowHeight := fontSize + 8
	for _, item := range items {
		itemWidth := 16 + estimateSVGTextWidth(item.Label, int(fontSize)) + 18
		if x > startX && x+itemWidth > maxX {
			row++
			x = startX
			y += rowHeight
		}
		entries = append(entries, svgLegendEntry{Row: row, Item: item, BoxX: x, TextX: x + 16, Y: y})
		x += itemWidth
	}
	return entries, y + rowHeight
}

func svgXAxisKind(spec *Spec) string {
	if spec.Axes.X.Type == AxisTypeCategory {
		return AxisTypeCategory
	}
	if spec.Axes.X.Type == AxisTypeTime {
		return AxisTypeTime
	}
	if spec.Domain.Kind == DomainKindTime {
		return AxisTypeTime
	}
	for _, series := range spec.Series {
		switch series.Representation.Kind {
		case RepresentationDistributionBoxplot:
			return AxisTypeCategory
		case RepresentationTimeBucketValue, RepresentationTimeBucketBand, RepresentationEventPoint, RepresentationEventRange:
			return AxisTypeTime
		}
	}
	return AxisTypeLinear
}

func svgCollectCategories(spec *Spec) []string {
	values := []string{}
	seen := map[string]struct{}{}
	add := func(value string) {
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	for _, category := range spec.Domain.Categories {
		add(category)
	}
	for _, series := range spec.Series {
		if series.Representation.Kind != RepresentationDistributionBoxplot {
			continue
		}
		index := fieldIndex(series.Representation.Fields, "category")
		for _, row := range series.Data {
			valuesRow, ok := row.([]any)
			if !ok || index < 0 || index >= len(valuesRow) {
				continue
			}
			add(formatAny(valuesRow[index]))
		}
	}
	return values
}

func svgCollectXContinuousDomain(spec *Spec, isTime bool) (float64, float64, bool) {
	values := []float64{}
	add := func(value any) {
		if converted, ok := svgContinuousValue(value, spec.Domain, isTime); ok {
			values = append(values, converted)
		}
	}
	if spec.Axes.X.Extent != nil {
		add(spec.Axes.X.Extent.Min)
		add(spec.Axes.X.Extent.Max)
	}
	if spec.Domain.From != nil {
		add(spec.Domain.From)
	}
	if spec.Domain.To != nil {
		add(spec.Domain.To)
	}
	for _, series := range spec.Series {
		switch series.Representation.Kind {
		case RepresentationRawPoint, RepresentationTimeBucketValue:
			xIndex := rawPointXIndex(series, spec.Domain)
			if series.Representation.Kind == RepresentationTimeBucketValue {
				xIndex = timeDomainXIndex(series, spec.Domain)
			}
			for _, row := range series.Data {
				valuesRow, ok := row.([]any)
				if !ok || xIndex < 0 || xIndex >= len(valuesRow) {
					continue
				}
				add(valuesRow[xIndex])
			}
		case RepresentationTimeBucketBand, RepresentationEventPoint:
			xIndex := fieldIndex(series.Representation.Fields, "time")
			if xIndex < 0 {
				xIndex = 0
			}
			for _, row := range series.Data {
				valuesRow, ok := row.([]any)
				if !ok || xIndex >= len(valuesRow) {
					continue
				}
				add(valuesRow[xIndex])
			}
		case RepresentationEventRange:
			fromIndex := fieldIndex(series.Representation.Fields, "from")
			toIndex := fieldIndex(series.Representation.Fields, "to")
			if fromIndex < 0 {
				fromIndex = 0
			}
			if toIndex < 0 {
				toIndex = 1
			}
			for _, row := range series.Data {
				valuesRow, ok := row.([]any)
				if !ok {
					continue
				}
				if fromIndex < len(valuesRow) {
					add(valuesRow[fromIndex])
				}
				if toIndex < len(valuesRow) {
					add(valuesRow[toIndex])
				}
			}
		case RepresentationDistributionHistogram:
			startIndex := fieldIndex(series.Representation.Fields, "binStart")
			endIndex := fieldIndex(series.Representation.Fields, "binEnd")
			for _, row := range series.Data {
				valuesRow, ok := row.([]any)
				if !ok {
					continue
				}
				if startIndex >= 0 && startIndex < len(valuesRow) {
					add(valuesRow[startIndex])
				}
				if endIndex >= 0 && endIndex < len(valuesRow) {
					add(valuesRow[endIndex])
				}
			}
		}
	}
	if len(values) == 0 {
		return 0, 0, false
	}
	minValue, maxValue := values[0], values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	if minValue == maxValue {
		if minValue == 0 {
			maxValue = 1
		} else {
			minValue = minValue * 0.95
			maxValue = maxValue * 1.05
		}
	}
	return minValue, maxValue, true
}

func svgCollectYDomain(spec *Spec, axisID string) (float64, float64, bool) {
	values := []float64{}
	add := func(value any) {
		if converted, ok := toFloat64(value); ok {
			values = append(values, converted)
		}
	}
	for _, axis := range spec.Axes.Y {
		if axis.ID != axisID || axis.Extent == nil {
			continue
		}
		add(axis.Extent.Min)
		add(axis.Extent.Max)
	}
	for _, series := range spec.Series {
		if svgSeriesAxisID(series, "y") != axisID {
			continue
		}
		switch series.Representation.Kind {
		case RepresentationRawPoint, RepresentationTimeBucketValue, RepresentationEventPoint:
			index := rawPointYIndex(series)
			for _, row := range series.Data {
				valuesRow, ok := row.([]any)
				if !ok || index < 0 || index >= len(valuesRow) {
					continue
				}
				add(valuesRow[index])
			}
		case RepresentationTimeBucketBand:
			for _, field := range []string{"min", "max", "avg"} {
				index := fieldIndex(series.Representation.Fields, field)
				for _, row := range series.Data {
					valuesRow, ok := row.([]any)
					if !ok || index < 0 || index >= len(valuesRow) {
						continue
					}
					add(valuesRow[index])
				}
			}
		case RepresentationDistributionHistogram:
			index := fieldIndex(series.Representation.Fields, "count")
			for _, row := range series.Data {
				valuesRow, ok := row.([]any)
				if !ok || index < 0 || index >= len(valuesRow) {
					continue
				}
				add(valuesRow[index])
			}
		case RepresentationDistributionBoxplot:
			for _, field := range []string{"low", "q1", "median", "q3", "high"} {
				index := fieldIndex(series.Representation.Fields, field)
				for _, row := range series.Data {
					valuesRow, ok := row.([]any)
					if !ok || index < 0 || index >= len(valuesRow) {
						continue
					}
					add(valuesRow[index])
				}
			}
			if outliers, ok := series.Extra["outliers"].([]any); ok {
				valueIndex := fieldIndex(series.Representation.OutlierFields, "value")
				if valueIndex < 0 {
					valueIndex = 1
				}
				for _, row := range outliers {
					valuesRow, ok := row.([]any)
					if !ok || valueIndex >= len(valuesRow) {
						continue
					}
					add(valuesRow[valueIndex])
				}
			}
		}
	}
	for _, annotation := range spec.Annotations {
		if isXAxis(spec, annotation.Axis) || svgAnnotationAxisID(annotation, axisID) != axisID {
			continue
		}
		switch annotation.Kind {
		case AnnotationKindLine:
			add(annotation.Value)
		case AnnotationKindRange:
			add(annotation.From)
			add(annotation.To)
		case AnnotationKindPoint:
			add(annotation.Value)
		}
	}
	if len(values) == 0 {
		return 0, 0, false
	}
	minValue, maxValue := values[0], values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	if minValue > 0 {
		minValue = 0
	}
	if minValue == maxValue {
		if minValue == 0 {
			maxValue = 1
		} else {
			minValue = minValue * 0.95
			maxValue = maxValue * 1.05
		}
	}
	return minValue, maxValue, true
}

func svgContinuousValue(value any, domain Domain, isTime bool) (float64, bool) {
	if isTime {
		ret, ok := parseTimeValueWithDomain(value, domain)
		if !ok {
			return 0, false
		}
		return float64(ret.UnixNano()), true
	}
	return toFloat64(value)
}

func svgSeriesAxisID(series Series, fallback string) string {
	if series.Axis != "" {
		return series.Axis
	}
	return fallback
}

func svgAnnotationAxisID(annotation Annotation, fallback string) string {
	if annotation.Axis == "" || annotation.Axis == "x" {
		return fallback
	}
	return annotation.Axis
}

type svgPoint struct {
	X float64
	Y float64
}

type svgTick struct {
	X     float64
	Y     float64
	Label string
}

func svgLinePoints(series Series, domain Domain, xScale svgXScale, yScale svgYScale) []svgPoint {
	points := []svgPoint{}
	xIndex := rawPointXIndex(series, domain)
	if series.Representation.Kind == RepresentationTimeBucketValue {
		xIndex = timeDomainXIndex(series, domain)
	}
	yIndex := rawPointYIndex(series)
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || xIndex < 0 || yIndex < 0 || xIndex >= len(values) || yIndex >= len(values) {
			continue
		}
		x, xOK := svgMapXValue(values[xIndex], xScale)
		y, yOK := svgMapYValue(values[yIndex], yScale)
		if xOK && yOK {
			points = append(points, svgPoint{X: x, Y: y})
		}
	}
	return points
}

func svgBandPoints(series Series, domain Domain, xScale svgXScale, yScale svgYScale) ([]svgPoint, []svgPoint, []svgPoint) {
	minPoints := []svgPoint{}
	maxPoints := []svgPoint{}
	avgPoints := []svgPoint{}
	timeIndex := fieldIndex(series.Representation.Fields, "time")
	if timeIndex < 0 {
		timeIndex = 0
	}
	minIndex := fieldIndex(series.Representation.Fields, "min")
	maxIndex := fieldIndex(series.Representation.Fields, "max")
	avgIndex := fieldIndex(series.Representation.Fields, "avg")
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || timeIndex >= len(values) {
			continue
		}
		x, xOK := svgMapXValue(values[timeIndex], xScale)
		if !xOK {
			continue
		}
		if minIndex >= 0 && minIndex < len(values) {
			if y, ok := svgMapYValue(values[minIndex], yScale); ok {
				minPoints = append(minPoints, svgPoint{X: x, Y: y})
			}
		}
		if maxIndex >= 0 && maxIndex < len(values) {
			if y, ok := svgMapYValue(values[maxIndex], yScale); ok {
				maxPoints = append(maxPoints, svgPoint{X: x, Y: y})
			}
		}
		if avgIndex >= 0 && avgIndex < len(values) {
			if y, ok := svgMapYValue(values[avgIndex], yScale); ok {
				avgPoints = append(avgPoints, svgPoint{X: x, Y: y})
			}
		}
	}
	return minPoints, maxPoints, avgPoints
}

func svgHistogramBars(series Series, xScale svgXScale, yScale svgYScale) []svgRect {
	ret := []svgRect{}
	startIndex := fieldIndex(series.Representation.Fields, "binStart")
	endIndex := fieldIndex(series.Representation.Fields, "binEnd")
	countIndex := fieldIndex(series.Representation.Fields, "count")
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || startIndex < 0 || endIndex < 0 || countIndex < 0 || startIndex >= len(values) || endIndex >= len(values) || countIndex >= len(values) {
			continue
		}
		x1, ok1 := svgMapXValue(values[startIndex], xScale)
		x2, ok2 := svgMapXValue(values[endIndex], xScale)
		y, okY := svgMapYValue(values[countIndex], yScale)
		if !ok1 || !ok2 || !okY {
			continue
		}
		ret = append(ret, svgRect{X: math.Min(x1, x2), Y: y, Width: math.Abs(x2 - x1), Height: yScale.Rect.Y + yScale.Rect.Height - y})
	}
	return ret
}

func svgBoxplotShapes(series Series, xScale svgXScale, yScale svgYScale, fillColor string, strokeColor string) []string {
	shapes := []string{}
	categoryIndex := fieldIndex(series.Representation.Fields, "category")
	lowIndex := fieldIndex(series.Representation.Fields, "low")
	q1Index := fieldIndex(series.Representation.Fields, "q1")
	medianIndex := fieldIndex(series.Representation.Fields, "median")
	q3Index := fieldIndex(series.Representation.Fields, "q3")
	highIndex := fieldIndex(series.Representation.Fields, "high")
	boxWidth := svgCategoryStep(xScale) * 0.46
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || categoryIndex < 0 || highIndex >= len(values) {
			continue
		}
		cx, okX := svgMapXValue(values[categoryIndex], xScale)
		low, okLow := svgMapYValue(values[lowIndex], yScale)
		q1, okQ1 := svgMapYValue(values[q1Index], yScale)
		median, okMedian := svgMapYValue(values[medianIndex], yScale)
		q3, okQ3 := svgMapYValue(values[q3Index], yScale)
		high, okHigh := svgMapYValue(values[highIndex], yScale)
		if !okX || !okLow || !okQ1 || !okMedian || !okQ3 || !okHigh {
			continue
		}
		left := cx - boxWidth/2
		shapes = append(shapes,
			"<line x1=\""+svgNumber(cx)+"\" y1=\""+svgNumber(low)+"\" x2=\""+svgNumber(cx)+"\" y2=\""+svgNumber(high)+"\" stroke=\""+escapeSVG(strokeColor)+"\" />",
			"<line x1=\""+svgNumber(cx-boxWidth/4)+"\" y1=\""+svgNumber(low)+"\" x2=\""+svgNumber(cx+boxWidth/4)+"\" y2=\""+svgNumber(low)+"\" stroke=\""+escapeSVG(strokeColor)+"\" />",
			"<line x1=\""+svgNumber(cx-boxWidth/4)+"\" y1=\""+svgNumber(high)+"\" x2=\""+svgNumber(cx+boxWidth/4)+"\" y2=\""+svgNumber(high)+"\" stroke=\""+escapeSVG(strokeColor)+"\" />",
			"<rect x=\""+svgNumber(left)+"\" y=\""+svgNumber(q3)+"\" width=\""+svgNumber(boxWidth)+"\" height=\""+svgNumber(math.Abs(q1-q3))+"\" fill=\""+escapeSVG(fillColor)+"\" fill-opacity=\"0.24\" stroke=\""+escapeSVG(strokeColor)+"\" />",
			"<line x1=\""+svgNumber(left)+"\" y1=\""+svgNumber(median)+"\" x2=\""+svgNumber(left+boxWidth)+"\" y2=\""+svgNumber(median)+"\" stroke=\""+escapeSVG(strokeColor)+"\" stroke-width=\"1.5\" />",
		)
	}
	if outliers, ok := series.Extra["outliers"].([]any); ok {
		categoryField := fieldIndex(series.Representation.OutlierFields, "category")
		valueField := fieldIndex(series.Representation.OutlierFields, "value")
		if categoryField < 0 {
			categoryField = 0
		}
		if valueField < 0 {
			valueField = 1
		}
		for _, row := range outliers {
			values, ok := row.([]any)
			if !ok || categoryField >= len(values) || valueField >= len(values) {
				continue
			}
			cx, okX := svgMapXValue(values[categoryField], xScale)
			cy, okY := svgMapYValue(values[valueField], yScale)
			if !okX || !okY {
				continue
			}
			shapes = append(shapes, "<circle cx=\""+svgNumber(cx)+"\" cy=\""+svgNumber(cy)+"\" r=\"2.5\" fill=\""+escapeSVG(strokeColor)+"\" />")
		}
	}
	return shapes
}

func svgEventPoints(series Series, domain Domain, xScale svgXScale, yScale svgYScale) []svgPoint {
	ret := []svgPoint{}
	timeIndex := fieldIndex(series.Representation.Fields, "time")
	valueIndex := fieldIndex(series.Representation.Fields, "value")
	if timeIndex < 0 {
		timeIndex = 0
	}
	if valueIndex < 0 {
		valueIndex = 1
	}
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || timeIndex >= len(values) || valueIndex >= len(values) {
			continue
		}
		x, okX := svgMapXValue(values[timeIndex], xScale)
		y, okY := svgMapYValue(values[valueIndex], yScale)
		if okX && okY {
			ret = append(ret, svgPoint{X: x, Y: y})
		}
	}
	return ret
}

func svgEventRangeRects(series Series, domain Domain, xScale svgXScale, plot svgRect) []svgRect {
	ret := []svgRect{}
	fromIndex := fieldIndex(series.Representation.Fields, "from")
	toIndex := fieldIndex(series.Representation.Fields, "to")
	if fromIndex < 0 {
		fromIndex = 0
	}
	if toIndex < 0 {
		toIndex = 1
	}
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || fromIndex >= len(values) || toIndex >= len(values) {
			continue
		}
		x1, ok1 := svgMapXValue(values[fromIndex], xScale)
		x2, ok2 := svgMapXValue(values[toIndex], xScale)
		if !ok1 || !ok2 {
			continue
		}
		ret = append(ret, svgRect{X: math.Min(x1, x2), Y: plot.Y, Width: math.Abs(x2 - x1), Height: plot.Height})
	}
	return ret
}

func svgMapXValue(value any, scale svgXScale) (float64, bool) {
	switch scale.Kind {
	case AxisTypeCategory:
		label := formatAny(value)
		index, ok := scale.CategoryIx[label]
		if !ok {
			return 0, false
		}
		step := svgCategoryStep(scale)
		return scale.Rect.X + step*float64(index) + step/2, true
	case AxisTypeTime:
		converted, ok := svgContinuousValue(value, scale.Domain, true)
		if !ok {
			return 0, false
		}
		return svgProject(converted, scale.Min, scale.Max, scale.Rect.X, scale.Rect.X+scale.Rect.Width), true
	default:
		converted, ok := toFloat64(value)
		if !ok {
			return 0, false
		}
		return svgProject(converted, scale.Min, scale.Max, scale.Rect.X, scale.Rect.X+scale.Rect.Width), true
	}
}

func svgMapYValue(value any, scale svgYScale) (float64, bool) {
	converted, ok := toFloat64(value)
	if !ok {
		return 0, false
	}
	return svgProject(converted, scale.Min, scale.Max, scale.Rect.Y+scale.Rect.Height, scale.Rect.Y), true
}

func svgProject(value, minValue, maxValue, start, end float64) float64 {
	if maxValue == minValue {
		return start
	}
	ratio := (value - minValue) / (maxValue - minValue)
	return start + (end-start)*ratio
}

func svgPath(points []svgPoint) string {
	parts := make([]string, 0, len(points)*3)
	for index, point := range points {
		cmd := "L"
		if index == 0 {
			cmd = "M"
		}
		parts = append(parts, cmd, svgNumber(point.X), svgNumber(point.Y))
	}
	return strings.Join(parts, " ")
}

func svgClosedBandPath(minPoints []svgPoint, maxPoints []svgPoint) string {
	parts := make([]string, 0, (len(minPoints)+len(maxPoints))*3+1)
	for index, point := range maxPoints {
		cmd := "L"
		if index == 0 {
			cmd = "M"
		}
		parts = append(parts, cmd, svgNumber(point.X), svgNumber(point.Y))
	}
	for index := len(minPoints) - 1; index >= 0; index-- {
		point := minPoints[index]
		parts = append(parts, "L", svgNumber(point.X), svgNumber(point.Y))
	}
	parts = append(parts, "Z")
	return strings.Join(parts, " ")
}

func svgContinuousTicks(scale svgXScale) []svgTick {
	if scale.Kind == AxisTypeTime {
		return svgTimeTicks(scale)
	}
	return svgNumericTicks(scale)
}

func svgNumericTicks(scale svgXScale) []svgTick {
	count := 5
	ticks := make([]svgTick, 0, count)
	for index := 0; index < count; index++ {
		ratio := float64(index) / float64(count-1)
		value := scale.Min + (scale.Max-scale.Min)*ratio
		label := svgFormatContinuousLabel(value, scale)
		ticks = append(ticks, svgTick{X: scale.Rect.X + scale.Rect.Width*ratio, Label: label})
	}
	return ticks
}

func svgTimeTicks(scale svgXScale) []svgTick {
	minValue := int64(math.Round(scale.Min))
	maxValue := int64(math.Round(scale.Max))
	if maxValue <= minValue {
		return []svgTick{{X: scale.Rect.X, Label: svgFormatTimeTick(minValue, 0)}}
	}
	span := maxValue - minValue
	step := svgNiceTimeStep(span, 6)
	start := (minValue / step) * step
	if start < minValue {
		start += step
	}
	ticks := []svgTick{}
	for value := start; value <= maxValue; value += step {
		x := svgProject(float64(value), scale.Min, scale.Max, scale.Rect.X, scale.Rect.X+scale.Rect.Width)
		ticks = append(ticks, svgTick{X: x, Label: svgFormatTimeTick(value, span)})
		if len(ticks) > 8 {
			break
		}
	}
	if len(ticks) < 2 {
		count := 5
		for index := 0; index < count; index++ {
			ratio := float64(index) / float64(count-1)
			value := minValue + int64(math.Round(float64(span)*ratio))
			ticks = append(ticks, svgTick{X: scale.Rect.X + scale.Rect.Width*ratio, Label: svgFormatTimeTick(value, span)})
		}
	}
	return ticks
}

func svgCategoryTicks(scale svgXScale) []svgTick {
	count := len(scale.Categories)
	if count == 0 {
		return nil
	}
	limit := count
	if limit > 8 {
		limit = 8
	}
	stepIndex := int(math.Ceil(float64(count) / float64(limit)))
	if stepIndex < 1 {
		stepIndex = 1
	}
	ret := []svgTick{}
	for index, label := range scale.Categories {
		if index%stepIndex != 0 && index != count-1 {
			continue
		}
		x, ok := svgMapXValue(label, scale)
		if !ok {
			continue
		}
		ret = append(ret, svgTick{X: x, Label: label})
	}
	return ret
}

func svgYTicks(scale svgYScale) []svgTick {
	count := 5
	ticks := make([]svgTick, 0, count)
	for index := 0; index < count; index++ {
		ratio := float64(index) / float64(count-1)
		value := scale.Max - (scale.Max-scale.Min)*ratio
		y := scale.Rect.Y + scale.Rect.Height*ratio
		ticks = append(ticks, svgTick{Y: y, Label: formatFloat(value)})
	}
	return ticks
}

func svgFormatContinuousLabel(value float64, scale svgXScale) string {
	if scale.Kind == AxisTypeTime {
		return svgFormatTimeTick(int64(math.Round(value)), int64(math.Round(scale.Max-scale.Min)))
	}
	return formatFloat(value)
}

func svgNiceTimeStep(span int64, targetCount int) int64 {
	if targetCount <= 1 || span <= 0 {
		return int64(time.Second)
	}
	target := span / int64(targetCount-1)
	candidates := []int64{
		int64(time.Second),
		int64(5 * time.Second),
		int64(15 * time.Second),
		int64(30 * time.Second),
		int64(time.Minute),
		int64(5 * time.Minute),
		int64(15 * time.Minute),
		int64(30 * time.Minute),
		int64(time.Hour),
		int64(3 * time.Hour),
		int64(6 * time.Hour),
		int64(12 * time.Hour),
		int64(24 * time.Hour),
		int64(7 * 24 * time.Hour),
		int64(30 * 24 * time.Hour),
		int64(90 * 24 * time.Hour),
		int64(365 * 24 * time.Hour),
	}
	for _, candidate := range candidates {
		if candidate >= target {
			return candidate
		}
	}
	return candidates[len(candidates)-1]
}

func svgFormatTimeTick(unixNano int64, span int64) string {
	timeValue := time.Unix(0, unixNano).UTC()
	switch {
	case span <= int64(time.Minute):
		return timeValue.Format("15:04:05")
	case span <= int64(6*time.Hour):
		return timeValue.Format("15:04")
	case span <= int64(48*time.Hour):
		return timeValue.Format("01-02 15:04")
	case span <= int64(180*24*time.Hour):
		return timeValue.Format("2006-01-02")
	case span <= int64(2*365*24*time.Hour):
		return timeValue.Format("2006-01")
	default:
		return timeValue.Format("2006")
	}
}

func svgCategoryStep(scale svgXScale) float64 {
	count := len(scale.Categories)
	if count <= 1 {
		return scale.Rect.Width
	}
	return scale.Rect.Width / float64(count)
}

func svgSeriesColor(series Series, index int) string {
	if color := styleString(series.Style, "color", ""); color != "" {
		return color
	}
	palette := []string{"#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd", "#8c564b"}
	return palette[index%len(palette)]
}

func estimateSVGTextWidth(value string, fontSize int) float64 {
	return float64(len(value)) * float64(fontSize) * 0.58
}

func svgNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func escapeSVG(value string) string {
	return html.EscapeString(value)
}

func maxFloat(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ret := values[0]
	for _, value := range values[1:] {
		if value > ret {
			ret = value
		}
	}
	return ret
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
