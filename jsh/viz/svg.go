package viz

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
	svgThemeOuterFrame   = "#8b8b8b"
	svgThemeOuterHi      = "#d4d4d4"
	svgThemePlotBorder   = "#000000"
	svgThemeGrid         = "#000000"
	svgThemeText         = "#000000"
)

var svgSeriesPalette = []string{"#00a000", "#0000ff", "#ff8000", "#ff0000", "#008080", "#800080"}

type SVGOptions struct {
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Padding    int    `json:"padding,omitempty"`
	Background string `json:"background,omitempty"`
	FontFamily string `json:"fontFamily,omitempty"`
	FontSize   int    `json:"fontSize,omitempty"`
	ShowLegend *bool  `json:"showLegend,omitempty"`
	Title      string `json:"title,omitempty"`
	Timeformat string `json:"timeformat,omitempty"`
	TZ         string `json:"tz,omitempty"`
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
	Time       resolvedOutputTimeOptions
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
	outputTime := OutputTimeOptions{}
	if options != nil {
		outputTime = OutputTimeOptions{Timeformat: options.Timeformat, TZ: options.TZ}
	}
	spec = spec.Normalize()
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	timeOptions, err := resolveOutputTimeOptions(spec.Domain, outputTime)
	if err != nil {
		return nil, err
	}
	resolved.Time = timeOptions
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
		Time:       resolvedOutputTimeOptions{Timeformat: TimeformatRFC3339, Location: time.UTC},
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
	if isMRTGTimeSeriesSpec(spec) {
		return buildMRTGSVGLayout(spec, options)
	}
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

func buildMRTGSVGLayout(spec *Spec, options svgResolvedOptions) (svgLayout, error) {
	canvas := svgRect{Width: float64(options.Width), Height: float64(options.Height)}
	axisOrder := orderedSVGYAxisIDs(spec)
	leftAxisCount, rightAxisCount := svgAxisSideCounts(axisOrder)
	titleHeight := 0.0
	if options.Title != "" {
		titleHeight = float64(options.FontSize) + 8
	}
	leftMargin := math.Max(88, float64(maxInt(options.Padding-10, 28))+float64(maxInt(leftAxisCount-1, 0))*34)
	rightMargin := math.Max(14, float64(maxInt(options.Padding/5, 6))+float64(rightAxisCount)*30)
	plotWidth := float64(options.Width) - leftMargin - rightMargin
	if plotWidth <= 0 {
		return svgLayout{}, fmt.Errorf("advn: svg plot area is too small")
	}
	legendItems := buildSVGLegendItems(spec, options.ShowLegend)
	legendEntries, legendHeight := layoutSVGLegendEntries(legendItems, leftMargin, plotWidth, float64(options.FontSize))
	legendGap := 0.0
	if legendHeight > 0 {
		legendGap = 2
	}
	xLabelHeight := float64(options.FontSize) + 20
	plotTop := math.Max(8, float64(options.Padding)/4) + titleHeight + legendHeight + legendGap
	plot := svgRect{
		X:      leftMargin,
		Y:      plotTop,
		Width:  plotWidth,
		Height: float64(options.Height) - plotTop - math.Max(12, float64(options.Padding)/4) - xLabelHeight,
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
	applyMRTGSVGAxisPlacements(axisPlacements, plot)
	legendRect := svgRect{}
	if legendHeight > 0 {
		legendRect = svgRect{
			X:      plot.X,
			Y:      math.Max(6, float64(options.Padding)/4) + titleHeight,
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

func applyMRTGSVGAxisPlacements(placements map[string]svgAxisPlacement, plot svgRect) {
	for axisID, placement := range placements {
		if placement.Side == "left" {
			placement.LineX = plot.X
			placement.TickLabelX = placement.LineX - 10
			placement.LabelX = placement.LineX - 46
			placement.TickDir = -1
			placement.TickAnchor = "end"
			placement.LabelAnchor = "middle"
		} else {
			placement.LineX = plot.X + plot.Width
			placement.TickLabelX = placement.LineX + 12
			placement.LabelX = placement.LineX + 44
			placement.TickDir = 1
			placement.TickAnchor = "start"
			placement.LabelAnchor = "middle"
		}
		placements[axisID] = placement
	}
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
	builder.WriteString("\" />")
	builder.WriteString("<rect x=\"4\" y=\"6\" width=\"")
	builder.WriteString(strconv.Itoa(layout.Options.Width - 8))
	builder.WriteString("\" height=\"")
	builder.WriteString(strconv.Itoa(layout.Options.Height - 12))
	builder.WriteString("\" fill=\"none\" stroke=\"")
	builder.WriteString(svgThemeOuterFrame)
	builder.WriteString("\" />")
	builder.WriteString("<rect x=\"5\" y=\"7\" width=\"")
	builder.WriteString(strconv.Itoa(layout.Options.Width - 10))
	builder.WriteString("\" height=\"")
	builder.WriteString(strconv.Itoa(layout.Options.Height - 14))
	builder.WriteString("\" fill=\"none\" stroke=\"")
	builder.WriteString(svgThemeOuterHi)
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
	builder.WriteString("<tspan fill=\"")
	builder.WriteString(svgThemeText)
	builder.WriteString("\">")
	builder.WriteString(escapeSVG(layout.Options.Title))
	builder.WriteString("</tspan></text></g>")
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
	builder.WriteString("\" fill=\"#ffffff\" stroke=\"")
	builder.WriteString(svgThemePlotBorder)
	builder.WriteString("\" />")
	builder.WriteString("<rect x=\"")
	builder.WriteString(svgNumber(layout.Plot.X + 1))
	builder.WriteString("\" y=\"")
	builder.WriteString(svgNumber(layout.Plot.Y + 1))
	builder.WriteString("\" width=\"")
	builder.WriteString(svgNumber(layout.Plot.Width - 2))
	builder.WriteString("\" height=\"")
	builder.WriteString(svgNumber(layout.Plot.Height - 2))
	builder.WriteString("\" fill=\"none\" stroke=\"")
	builder.WriteString(svgThemeOuterHi)
	builder.WriteString("\" />")
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
	builder.WriteString("\" stroke=\"")
	builder.WriteString(svgThemePlotBorder)
	builder.WriteString("\" />")
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
			builder.WriteString("\" stroke=\"")
			builder.WriteString(svgThemePlotBorder)
			builder.WriteString("\" />")
			builder.WriteString("<text x=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(y + float64(layout.Options.FontSize) + 8))
			builder.WriteString("\" text-anchor=\"middle\" fill=\"")
			builder.WriteString(svgThemeText)
			builder.WriteString("\">")
			builder.WriteString(escapeSVG(tick.Label))
			builder.WriteString("</text>")
		}
	} else {
		for _, tick := range svgContinuousTicks(layout.XAxis, layout.Options.Time) {
			builder.WriteString("<line x1=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y1=\"")
			builder.WriteString(svgNumber(layout.Plot.Y))
			builder.WriteString("\" x2=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y2=\"")
			builder.WriteString(svgNumber(y))
			builder.WriteString("\" stroke=\"")
			builder.WriteString(svgThemeGrid)
			builder.WriteString("\" stroke-dasharray=\"1 3\" />")
			builder.WriteString("<text x=\"")
			builder.WriteString(svgNumber(tick.X))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(y + float64(layout.Options.FontSize) + 8))
			builder.WriteString("\" text-anchor=\"middle\" fill=\"")
			builder.WriteString(svgThemeText)
			builder.WriteString("\">")
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
		builder.WriteString("\" text-anchor=\"middle\" fill=\"")
		builder.WriteString(svgThemeText)
		builder.WriteString("\">")
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
		tickFontSize := layout.Options.FontSize
		tickYOffset := float64(layout.Options.FontSize) / 3
		labelX := placement.LabelX
		if isMRTGTimeSeriesSpec(spec) {
			tickFontSize = maxInt(10, layout.Options.FontSize-1)
			tickYOffset = float64(tickFontSize) / 3
		}
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
		builder.WriteString("\" stroke=\"")
		builder.WriteString(svgThemePlotBorder)
		builder.WriteString("\" />")
		for _, tick := range svgYTicksWithSpec(spec, scale) {
			if isPrimary {
				builder.WriteString("<line x1=\"")
				builder.WriteString(svgNumber(layout.Plot.X))
				builder.WriteString("\" y1=\"")
				builder.WriteString(svgNumber(tick.Y))
				builder.WriteString("\" x2=\"")
				builder.WriteString(svgNumber(layout.Plot.X + layout.Plot.Width))
				builder.WriteString("\" y2=\"")
				builder.WriteString(svgNumber(tick.Y))
				builder.WriteString("\" stroke=\"")
				builder.WriteString(svgThemeGrid)
				builder.WriteString("\" stroke-dasharray=\"1 3\" />")
			}
			builder.WriteString("<line x1=\"")
			builder.WriteString(svgNumber(placement.LineX))
			builder.WriteString("\" y1=\"")
			builder.WriteString(svgNumber(tick.Y))
			builder.WriteString("\" x2=\"")
			builder.WriteString(svgNumber(placement.LineX + placement.TickDir*5))
			builder.WriteString("\" y2=\"")
			builder.WriteString(svgNumber(tick.Y))
			builder.WriteString("\" stroke=\"")
			builder.WriteString(svgThemePlotBorder)
			builder.WriteString("\" />")
			builder.WriteString("<text x=\"")
			builder.WriteString(svgNumber(placement.TickLabelX))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(tick.Y + tickYOffset))
			builder.WriteString("\" text-anchor=\"")
			builder.WriteString(placement.TickAnchor)
			builder.WriteString("\" font-size=\"")
			builder.WriteString(strconv.Itoa(tickFontSize))
			builder.WriteString("\" fill=\"")
			builder.WriteString(svgThemeText)
			builder.WriteString("\">")
			builder.WriteString(escapeSVG(tick.Label))
			builder.WriteString("</text>")
		}
		label := svgAxisLabel(spec, axisID)
		if label != "" {
			builder.WriteString("<text x=\"")
			builder.WriteString(svgNumber(labelX))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(layout.Plot.Y + layout.Plot.Height/2))
			builder.WriteString("\" transform=\"rotate(-90 ")
			builder.WriteString(svgNumber(labelX))
			builder.WriteString(" ")
			builder.WriteString(svgNumber(layout.Plot.Y + layout.Plot.Height/2))
			builder.WriteString(")\" text-anchor=\"")
			builder.WriteString(placement.LabelAnchor)
			builder.WriteString("\" fill=\"")
			builder.WriteString(svgThemeText)
			builder.WriteString("\">")
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
	lineWidth := styleFloat(series.Style, "lineWidth", 1.2)
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
	rowEntries := []svgLegendEntry{}
	flushRow := func() {
		if len(rowEntries) == 0 {
			return
		}
		rowWidth := svgLegendRowWidth(rowEntries, float64(layout.Options.FontSize))
		offset := (layout.LegendRect.Width - rowWidth) / 2
		if offset < 0 {
			offset = 0
		}
		builder.WriteString("<g data-advn-legend-row=\"")
		builder.WriteString(strconv.Itoa(currentRow))
		builder.WriteString("\">")
		for _, entry := range rowEntries {
			boxX := entry.BoxX + offset
			textX := entry.TextX + offset
			y := entry.Y + layout.LegendRect.Y
			builder.WriteString("<rect x=\"")
			builder.WriteString(svgNumber(boxX))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(y))
			builder.WriteString("\" width=\"10\" height=\"10\" fill=\"")
			builder.WriteString(escapeSVG(entry.Item.Color))
			builder.WriteString("\" stroke=\"")
			builder.WriteString(svgThemePlotBorder)
			builder.WriteString("\" />")
			builder.WriteString("<text x=\"")
			builder.WriteString(svgNumber(textX))
			builder.WriteString("\" y=\"")
			builder.WriteString(svgNumber(y + 9))
			builder.WriteString("\" fill=\"")
			builder.WriteString(svgThemeText)
			builder.WriteString("\">")
			builder.WriteString(escapeSVG(entry.Item.Label))
			builder.WriteString("</text>")
		}
		builder.WriteString("</g>")
	}
	for _, entry := range layout.LegendEntries {
		if entry.Row != currentRow {
			flushRow()
			currentRow = entry.Row
			rowEntries = rowEntries[:0]
		}
		rowEntries = append(rowEntries, entry)
	}
	flushRow()
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
		scale.CategoryIx = make(map[string]int, len(categories))
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
	values := make([]string, 0, len(spec.Domain.Categories)+len(spec.Series))
	seen := make(map[string]struct{}, len(spec.Domain.Categories)+len(spec.Series))
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
	minValue := 0.0
	maxValue := 0.0
	hasValue := false
	add := func(value any) {
		if converted, ok := svgContinuousValue(value, spec.Domain, isTime); ok {
			if !hasValue {
				minValue = converted
				maxValue = converted
				hasValue = true
				return
			}
			if converted < minValue {
				minValue = converted
			}
			if converted > maxValue {
				maxValue = converted
			}
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
	if !hasValue {
		return 0, 0, false
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
	minValue := 0.0
	maxValue := 0.0
	hasValue := false
	add := func(value any) {
		if converted, ok := toFloat64(value); ok {
			if !hasValue {
				minValue = converted
				maxValue = converted
				hasValue = true
				return
			}
			if converted < minValue {
				minValue = converted
			}
			if converted > maxValue {
				maxValue = converted
			}
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
	if !hasValue {
		return 0, 0, false
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
	points := make([]svgPoint, 0, len(series.Data))
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
	minPoints := make([]svgPoint, 0, len(series.Data))
	maxPoints := make([]svgPoint, 0, len(series.Data))
	avgPoints := make([]svgPoint, 0, len(series.Data))
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
	ret := make([]svgRect, 0, len(series.Data))
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
	shapes := make([]string, 0, len(series.Data)*5)
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
	ret := make([]svgPoint, 0, len(series.Data))
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
	ret := make([]svgRect, 0, len(series.Data))
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
	if len(points) == 0 {
		return ""
	}
	var builder strings.Builder
	for index, point := range points {
		if index == 0 {
			builder.WriteString("M ")
		} else {
			builder.WriteString(" L ")
		}
		builder.WriteString(svgNumber(point.X))
		builder.WriteByte(' ')
		builder.WriteString(svgNumber(point.Y))
	}
	return builder.String()
}

func svgClosedBandPath(minPoints []svgPoint, maxPoints []svgPoint) string {
	if len(maxPoints) == 0 {
		return ""
	}
	var builder strings.Builder
	for index, point := range maxPoints {
		if index == 0 {
			builder.WriteString("M ")
		} else {
			builder.WriteString(" L ")
		}
		builder.WriteString(svgNumber(point.X))
		builder.WriteByte(' ')
		builder.WriteString(svgNumber(point.Y))
	}
	for index := len(minPoints) - 1; index >= 0; index-- {
		point := minPoints[index]
		builder.WriteString(" L ")
		builder.WriteString(svgNumber(point.X))
		builder.WriteByte(' ')
		builder.WriteString(svgNumber(point.Y))
	}
	builder.WriteString(" Z")
	return builder.String()
}

func svgContinuousTicks(scale svgXScale, timeOptions resolvedOutputTimeOptions) []svgTick {
	if scale.Kind == AxisTypeTime {
		return svgTimeTicks(scale, timeOptions)
	}
	return svgNumericTicks(scale)
}

func svgNumericTicks(scale svgXScale) []svgTick {
	count := 5
	ticks := make([]svgTick, 0, count)
	for index := 0; index < count; index++ {
		ratio := float64(index) / float64(count-1)
		value := scale.Min + (scale.Max-scale.Min)*ratio
		label := formatFloat(value)
		ticks = append(ticks, svgTick{X: scale.Rect.X + scale.Rect.Width*ratio, Label: label})
	}
	return ticks
}

func svgTimeTicks(scale svgXScale, timeOptions resolvedOutputTimeOptions) []svgTick {
	minValue := int64(math.Round(scale.Min))
	maxValue := int64(math.Round(scale.Max))
	if maxValue <= minValue {
		return []svgTick{{X: scale.Rect.X, Label: formatUnixNanoWithOptions(minValue, 0, timeOptions)}}
	}
	span := time.Duration(maxValue - minValue)
	spec := chooseSVGTimeTickSpec(span, int(math.Round(scale.Rect.Width)))
	step := int64(spec.Step)
	start := (minValue / step) * step
	if start < minValue {
		start += step
	}
	ticks := make([]svgTick, 0, 8)
	for value := start; value <= maxValue; value += step {
		x := svgProject(float64(value), scale.Min, scale.Max, scale.Rect.X, scale.Rect.X+scale.Rect.Width)
		label := formatMRTGTimeTick(time.Unix(0, value).In(timeOptions.Location), spec.Format)
		ticks = append(ticks, svgTick{X: x, Label: label})
		if len(ticks) > 8 {
			break
		}
	}
	if len(ticks) < 2 {
		count := 5
		for index := 0; index < count; index++ {
			ratio := float64(index) / float64(count-1)
			value := minValue + int64(math.Round(float64(maxValue-minValue)*ratio))
			label := formatMRTGTimeTick(time.Unix(0, value).In(timeOptions.Location), spec.Format)
			ticks = append(ticks, svgTick{X: scale.Rect.X + scale.Rect.Width*ratio, Label: label})
		}
	}
	return ticks
}

func chooseSVGTimeTickSpec(span time.Duration, plotWidth int) pngTimeTickSpec {
	candidates := mrtgTickCandidates(span)
	if len(candidates) == 0 {
		return pngTimeTickSpec{Step: time.Hour, Format: "15:04"}
	}
	normalized := make([]pngTimeTickSpec, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		normalizedCandidate := normalizeSVGTimeTickSpec(span, candidate)
		key := fmt.Sprintf("%d|%s", normalizedCandidate.Step, normalizedCandidate.Format)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, normalizedCandidate)
	}
	if len(normalized) == 0 {
		return pngTimeTickSpec{Step: time.Hour, Format: "15:04"}
	}
	minSpacing := 18
	for _, candidate := range normalized {
		labelWidth := int(math.Ceil(estimateSVGTextWidth(sampleMRTGTimeTickLabel(candidate, span), svgDefaultFontSize))) + 8
		requiredSpacing := maxInt(minSpacing, labelWidth/2+6)
		tickCount := int(math.Ceil(span.Seconds()/candidate.Step.Seconds())) + 1
		if tickCount <= 1 {
			return candidate
		}
		spacing := plotWidth / maxInt(1, tickCount-1)
		if spacing >= requiredSpacing {
			return candidate
		}
	}
	return normalized[len(normalized)-1]
}

func normalizeSVGTimeTickSpec(span time.Duration, candidate pngTimeTickSpec) pngTimeTickSpec {
	switch {
	case span <= 15*time.Minute:
		if candidate.Step < time.Minute {
			candidate.Step = time.Minute
		}
		candidate.Format = "15:04"
	case span <= 24*time.Hour:
		candidate.Format = "15:04"
	case span <= 72*time.Hour:
		if candidate.Step < 12*time.Hour {
			candidate.Format = "15:04"
		} else if candidate.Step < 24*time.Hour {
			candidate.Format = "01-02 15h"
		} else {
			candidate.Format = "01-02"
		}
	}
	return candidate
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
	ret := make([]svgTick, 0, limit+1)
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

func svgYTicksWithSpec(spec *Spec, scale svgYScale) []svgTick {
	count := 5
	ticks := make([]svgTick, 0, count)
	for index := 0; index < count; index++ {
		ratio := float64(index) / float64(count-1)
		value := scale.Max - (scale.Max-scale.Min)*ratio
		y := scale.Rect.Y + scale.Rect.Height*ratio
		label := formatFloat(value)
		if isMRTGTimeSeriesSpec(spec) {
			label = formatMRTGNumber(value)
		}
		ticks = append(ticks, svgTick{Y: y, Label: label})
	}
	return ticks
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
	return svgSeriesPalette[index%len(svgSeriesPalette)]
}

func svgLegendRowWidth(entries []svgLegendEntry, fontSize float64) float64 {
	if len(entries) == 0 {
		return 0
	}
	first := entries[0]
	last := entries[len(entries)-1]
	return (last.TextX + estimateSVGTextWidth(last.Item.Label, int(fontSize))) - first.BoxX
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
