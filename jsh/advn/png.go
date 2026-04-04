package advn

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/png"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	pngDefaultScale = 1.0
	pngDefaultTheme = "mrtg"
)

type PNGOptions struct {
	Scale      float64 `json:"scale,omitempty"`
	DPI        int     `json:"dpi,omitempty"`
	Background string  `json:"background,omitempty"`
	Theme      string  `json:"theme,omitempty"`
}

type pngResolvedOptions struct {
	Scale      float64
	Background color.RGBA
	Theme      string
}

type pngTheme struct {
	Background      color.RGBA
	OuterFrame      color.RGBA
	OuterHighlight  color.RGBA
	PlotBackground  color.RGBA
	PlotBorder      color.RGBA
	Grid            color.RGBA
	Text            color.RGBA
	StatsText       color.RGBA
	SeriesFill      []color.RGBA
	SeriesLine      []color.RGBA
	Annotation      color.RGBA
	StatsBackground color.RGBA
}

type pngRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

type pngLayout struct {
	Canvas       pngRect
	Frame        pngRect
	Plot         pngRect
	Stats        pngRect
	Legend       pngLegendPlan
	TitleBaseY   int
	XTickBaseY   int
	YLabelX      int
	YLabelCenter int
}

type pngSeriesStats struct {
	Label   string
	Color   color.RGBA
	Max     float64
	Average float64
	Current float64
	HasData bool
}

type pngStatTextItem struct {
	Text  string
	Color color.RGBA
}

type pngLegendItem struct {
	Parts []pngStatTextItem
	Width int
}

type pngLegendPlan struct {
	Rows       [][]pngLegendItem
	LabelsOnly bool
}

type pngSeriesData struct {
	Series       Series
	Label        string
	FillColor    color.RGBA
	LineColor    color.RGBA
	Points       []pngPoint
	Stats        pngSeriesStats
	IsPrimary    bool
	IsAreaFilled bool
}

type pngPoint struct {
	Time  int64
	Value float64
	X     int
	Y     int
}

type pngTick struct {
	Value float64
	Pos   int
	Label string
}

type pngTimeTickSpec struct {
	Step   time.Duration
	Format string
}

type pngRenderer struct {
	img        *image.RGBA
	layout     pngLayout
	theme      pngTheme
	face       font.Face
	ascent     int
	lineHeight int
}

func ToPNG(spec *Spec, svgOptions *SVGOptions, options *PNGOptions) ([]byte, error) {
	if spec == nil {
		return nil, fmt.Errorf("advn: spec is nil")
	}
	resolvedSVG, err := normalizeSVGOptions(svgOptions)
	if err != nil {
		return nil, err
	}
	outputTime := OutputTimeOptions{}
	if svgOptions != nil {
		outputTime = OutputTimeOptions{Timeformat: svgOptions.Timeformat, TZ: svgOptions.TZ}
	}
	spec = spec.Normalize()
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	timeOptions, err := resolveOutputTimeOptions(spec.Domain, outputTime)
	if err != nil {
		return nil, err
	}
	resolvedSVG.Time = timeOptions

	resolvedPNG, err := normalizePNGOptions(options, resolvedSVG.Background)
	if err != nil {
		return nil, err
	}
	resolvedSVG = scaleSVGOptionsForPNG(resolvedSVG, resolvedPNG.Scale)

	if isMRTGTimeSeriesSpec(spec) {
		return renderMRTGPNG(spec, resolvedSVG, resolvedPNG)
	}
	return renderFallbackPNG(spec, resolvedSVG, resolvedPNG)
}

func isMRTGTimeSeriesSpec(spec *Spec) bool {
	if spec == nil || len(spec.Series) == 0 {
		return false
	}
	if spec.Domain.Kind != "" && spec.Domain.Kind != DomainKindTime {
		return false
	}
	for _, series := range spec.Series {
		switch series.Representation.Kind {
		case RepresentationRawPoint, RepresentationTimeBucketValue:
		case RepresentationEventRange:
		default:
			return false
		}
	}
	return true
}

func renderMRTGPNG(spec *Spec, options svgResolvedOptions, pngOptions pngResolvedOptions) ([]byte, error) {
	theme := mrtgPNGTheme()
	if pngOptions.Background.A != 0 {
		theme.Background = pngOptions.Background
	}

	start, end, err := resolveMRTGTimeRange(spec)
	if err != nil {
		return nil, err
	}
	primaryLabel := ""
	if len(spec.Axes.Y) > 0 {
		primaryLabel = spec.Axes.Y[0].Label
		if primaryLabel == "" {
			primaryLabel = spec.Axes.Y[0].ID
		}
	}
	if primaryLabel == "" {
		primaryLabel = "Value"
	}
	seriesData := buildMRTGSeriesData(spec, theme)
	legendPlan := buildMRTGLegendPlan(seriesData, options.Width-12, theme.StatsText)
	layout := buildMRTGPNGLayout(options, legendPlan)
	projectMRTGSeriesPoints(seriesData, start, end, layout.Plot)
	img := image.NewRGBA(image.Rect(0, 0, layout.Canvas.Width, layout.Canvas.Height))
	renderer := newPNGRenderer(img, layout, theme)
	yMin, yMax := resolveMRTGYRange(spec, seriesData)
	yTicks := buildMRTGYTicks(yMin, yMax, layout.Plot)
	xTicks := buildMRTGXTimeTicks(start, end, layout.Plot, options.Time, rendererMeasureTickLabelWidth)

	renderer.fillRect(rectToImage(layout.Canvas), theme.Background)
	renderer.strokeRect(rectToImage(layout.Frame), theme.OuterFrame)
	renderer.strokeRectInset(rectToImage(layout.Frame), 1, theme.OuterHighlight)
	renderer.fillRect(rectToImage(layout.Plot), theme.PlotBackground)
	if options.Title != "" {
		renderer.drawText(layout.Frame.X+layout.Frame.Width/2, layout.TitleBaseY, options.Title, theme.Text, "center")
	}
	renderer.drawMRTGTimeSeries(spec, seriesData, yMin, yMax, start, end)
	renderer.drawMRTGGrid(xTicks, yTicks)
	renderer.strokeRect(rectToImage(layout.Plot), theme.PlotBorder)
	renderer.strokeRectInset(rectToImage(layout.Plot), 1, theme.OuterHighlight)
	renderer.drawMRTGAxes(xTicks, yTicks)
	renderer.drawRotatedTextCCW(layout.YLabelX, layout.YLabelCenter, primaryLabel, theme.Text)
	renderer.drawMRTGStats(seriesData, layout.Stats)
	renderer.strokeRect(rectToImage(layout.Frame), theme.OuterFrame)
	renderer.strokeRectInset(rectToImage(layout.Frame), 1, theme.OuterHighlight)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderFallbackPNG(spec *Spec, options svgResolvedOptions, pngOptions pngResolvedOptions) ([]byte, error) {
	return renderMRTGPNG(spec, options, pngOptions)
}

func normalizePNGOptions(options *PNGOptions, fallbackBackground string) (pngResolvedOptions, error) {
	scale := pngDefaultScale
	backgroundText := fallbackBackground
	theme := pngDefaultTheme
	if options != nil {
		if options.Scale > 0 {
			scale = options.Scale
		} else if options.DPI > 0 {
			scale = float64(options.DPI) / 96.0
		}
		if options.Background != "" {
			backgroundText = options.Background
		}
		if options.Theme != "" {
			theme = strings.ToLower(strings.TrimSpace(options.Theme))
		}
	}
	if scale <= 0 {
		return pngResolvedOptions{}, fmt.Errorf("advn: png scale must be greater than 0")
	}
	if theme != pngDefaultTheme {
		return pngResolvedOptions{}, fmt.Errorf("advn: unsupported png theme %q", theme)
	}
	background, err := parseSimpleColor(backgroundText)
	if err != nil {
		return pngResolvedOptions{}, err
	}
	return pngResolvedOptions{Scale: scale, Background: background, Theme: theme}, nil
}

func scaleSVGOptionsForPNG(options svgResolvedOptions, scale float64) svgResolvedOptions {
	if scale == 1 {
		return options
	}
	options.Width = maxInt(1, int(math.Round(float64(options.Width)*scale)))
	options.Height = maxInt(1, int(math.Round(float64(options.Height)*scale)))
	options.Padding = maxInt(0, int(math.Round(float64(options.Padding)*scale)))
	options.FontSize = maxInt(1, int(math.Round(float64(options.FontSize)*scale)))
	return options
}

func mrtgPNGTheme() pngTheme {
	return pngTheme{
		Background:      mustParseColor("#ffffff"),
		OuterFrame:      mustParseColor("#8b8b8b"),
		OuterHighlight:  mustParseColor("#d4d4d4"),
		PlotBackground:  mustParseColor("#ffffff"),
		PlotBorder:      mustParseColor("#000000"),
		Grid:            mustParseColor("#000000"),
		Text:            mustParseColor("#000000"),
		StatsText:       mustParseColor("#4a4a4a"),
		SeriesFill:      []color.RGBA{mustParseColor("#00d000"), mustParseColor("#0000ff"), mustParseColor("#ff8000")},
		SeriesLine:      []color.RGBA{mustParseColor("#00a000"), mustParseColor("#0000ff"), mustParseColor("#ff0000")},
		Annotation:      mustParseColor("#ff0000"),
		StatsBackground: mustParseColor("#ffffff"),
	}
}

func mrtgStatsHeight(options svgResolvedOptions, legendPlan pngLegendPlan) int {
	lineCount := len(legendPlan.Rows)
	if lineCount == 0 {
		lineCount = 1
	}
	lineHeight := options.FontSize + 2
	padding := 6
	return lineCount*lineHeight + padding
}

func buildMRTGPNGLayout(options svgResolvedOptions, legendPlan pngLegendPlan) pngLayout {
	width := options.Width
	height := options.Height
	canvas := pngRect{Width: width, Height: height}
	frame := pngRect{X: 4, Y: 6, Width: width - 8, Height: height - 12}
	titleHeight := 0
	if options.Title != "" {
		titleHeight = options.FontSize + 8
	}
	statsHeight := mrtgStatsHeight(options, legendPlan)
	xTickHeight := options.FontSize + 8
	xTickOffset := options.FontSize + 4
	leftMargin := maxInt(66, options.FontSize*5+6)
	rightMargin := maxInt(18, options.FontSize+8)
	topMargin := frame.Y + 12 + titleHeight
	statsGap := 6
	statsBottomInset := 4
	if legendPlan.LabelsOnly && len(legendPlan.Rows) > 1 {
		statsGap = 8
		statsBottomInset = 6
	}
	frameBottom := frame.Y + frame.Height
	plotHeight := frameBottom - topMargin - statsHeight - xTickHeight - statsGap - statsBottomInset
	if plotHeight < 80 {
		plotHeight = 80
	}
	plot := pngRect{
		X:      leftMargin + 16,
		Y:      topMargin,
		Width:  width - leftMargin - rightMargin - 22,
		Height: plotHeight,
	}
	stats := pngRect{
		X:      frame.X + 2,
		Y:      frameBottom - statsBottomInset - statsHeight,
		Width:  frame.Width - 4,
		Height: statsHeight,
	}
	return pngLayout{
		Canvas:       canvas,
		Frame:        frame,
		Plot:         plot,
		Stats:        stats,
		Legend:       legendPlan,
		TitleBaseY:   frame.Y + options.FontSize + 3,
		XTickBaseY:   plot.Y + plot.Height + xTickOffset,
		YLabelX:      frame.X + 12,
		YLabelCenter: plot.Y + plot.Height/2,
	}
}

func newPNGRenderer(img *image.RGBA, layout pngLayout, theme pngTheme) *pngRenderer {
	face := basicfont.Face7x13
	metrics := face.Metrics()
	return &pngRenderer{
		img:        img,
		layout:     layout,
		theme:      theme,
		face:       face,
		ascent:     metrics.Ascent.Ceil(),
		lineHeight: (metrics.Ascent + metrics.Descent).Ceil(),
	}
}

func resolveMRTGTimeRange(spec *Spec) (int64, int64, error) {
	values := []int64{}
	add := func(value any) {
		ts, ok := parseTimeValueWithDomain(value, spec.Domain)
		if ok {
			values = append(values, ts.UnixNano())
		}
	}
	add(spec.Domain.From)
	add(spec.Domain.To)
	for _, series := range spec.Series {
		xIndex := rawPointXIndex(series, spec.Domain)
		if series.Representation.Kind == RepresentationTimeBucketValue {
			xIndex = timeDomainXIndex(series, spec.Domain)
		}
		if series.Representation.Kind == RepresentationEventRange {
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
			continue
		}
		for _, row := range series.Data {
			valuesRow, ok := row.([]any)
			if !ok || xIndex < 0 || xIndex >= len(valuesRow) {
				continue
			}
			add(valuesRow[xIndex])
		}
	}
	if len(values) == 0 {
		return 0, 0, fmt.Errorf("advn: unable to resolve png time domain")
	}
	start, end := values[0], values[0]
	for _, value := range values[1:] {
		if value < start {
			start = value
		}
		if value > end {
			end = value
		}
	}
	if start == end {
		end = start + int64(time.Minute)
	}
	return start, end, nil
}

func buildMRTGSeriesData(spec *Spec, theme pngTheme) []pngSeriesData {
	ret := make([]pngSeriesData, 0, len(spec.Series))
	seriesIndex := 0
	for _, series := range spec.Series {
		if series.Representation.Kind == RepresentationEventRange {
			continue
		}
		data := pngSeriesData{
			Series:       series,
			Label:        pngSeriesLabel(series),
			FillColor:    theme.SeriesFill[minInt(seriesIndex, len(theme.SeriesFill)-1)],
			LineColor:    theme.SeriesLine[minInt(seriesIndex, len(theme.SeriesLine)-1)],
			IsPrimary:    seriesIndex == 0,
			IsAreaFilled: seriesIndex == 0,
		}
		if colorValue, ok := styleColor(series.Style, "color"); ok {
			data.FillColor = colorValue
			data.LineColor = colorValue
		}
		if colorValue, ok := styleColor(series.Style, "lineColor"); ok {
			data.LineColor = colorValue
		}
		xIndex := rawPointXIndex(series, spec.Domain)
		if series.Representation.Kind == RepresentationTimeBucketValue {
			xIndex = timeDomainXIndex(series, spec.Domain)
		}
		yIndex := rawPointYIndex(series)
		sum := 0.0
		for _, row := range series.Data {
			values, ok := row.([]any)
			if !ok || xIndex < 0 || yIndex < 0 || xIndex >= len(values) || yIndex >= len(values) {
				continue
			}
			ts, ok := parseTimeValueWithDomain(values[xIndex], spec.Domain)
			if !ok {
				continue
			}
			v, ok := toFloat64(values[yIndex])
			if !ok {
				continue
			}
			data.Points = append(data.Points, pngPoint{Time: ts.UnixNano(), Value: v})
			if !data.Stats.HasData || v > data.Stats.Max {
				data.Stats.Max = v
			}
			data.Stats.Current = v
			data.Stats.HasData = true
			sum += v
		}
		slices.SortFunc(data.Points, func(a pngPoint, b pngPoint) int {
			switch {
			case a.Time < b.Time:
				return -1
			case a.Time > b.Time:
				return 1
			default:
				return 0
			}
		})
		if len(data.Points) > 0 {
			data.Stats.Average = sum / float64(len(data.Points))
		}
		data.Stats.Label = data.Label
		data.Stats.Color = data.LineColor
		ret = append(ret, data)
		seriesIndex++
	}
	return ret
}

func projectMRTGSeriesPoints(seriesData []pngSeriesData, start int64, end int64, plot pngRect) {
	for idx := range seriesData {
		for i := range seriesData[idx].Points {
			seriesData[idx].Points[i].X = projectTimeToX(seriesData[idx].Points[i].Time, start, end, plot)
		}
	}
}

func buildMRTGLegendPlan(seriesData []pngSeriesData, availableWidth int, statsTextColor color.RGBA) pngLegendPlan {
	if availableWidth <= 0 {
		availableWidth = svgDefaultWidth
	}
	fullItems := buildMRTGLegendItems(seriesData, false, statsTextColor)
	if plan, ok := fitMRTGLegendPlan(fullItems, availableWidth, false); ok {
		return plan
	}
	labelItems := buildMRTGLegendItems(seriesData, true, statsTextColor)
	if plan, ok := fitMRTGLegendPlan(labelItems, availableWidth, true); ok {
		return plan
	}
	return forceMRTGLegendPlan(labelItems, true)
}

func buildMRTGLegendItems(seriesData []pngSeriesData, labelsOnly bool, statsTextColor color.RGBA) []pngLegendItem {
	items := make([]pngLegendItem, 0, len(seriesData))
	for index, series := range seriesData {
		if !series.Stats.HasData {
			continue
		}
		label := series.Stats.Label
		if label == "" {
			label = fmt.Sprintf("Series%d", index+1)
		}
		parts := []pngStatTextItem{{Text: label, Color: series.Stats.Color}}
		if !labelsOnly {
			parts = append(parts, pngStatTextItem{
				Text:  fmt.Sprintf(" Max: %s, Avg: %s, Cur: %s", formatMRTGStat(series.Stats.Max), formatMRTGStat(series.Stats.Average), formatMRTGStat(series.Stats.Current)),
				Color: statsTextColor,
			})
		}
		width := 0
		for _, part := range parts {
			width += measurePNGTextWidth(part.Text)
		}
		items = append(items, pngLegendItem{Parts: parts, Width: width})
	}
	return items
}

func fitMRTGLegendPlan(items []pngLegendItem, availableWidth int, labelsOnly bool) (pngLegendPlan, bool) {
	if len(items) == 0 {
		return pngLegendPlan{LabelsOnly: labelsOnly}, true
	}
	for rows := 1; rows <= minInt(2, len(items)); rows++ {
		if plan, ok := arrangeMRTGLegendRows(items, availableWidth, rows, labelsOnly); ok {
			return plan, true
		}
	}
	return pngLegendPlan{}, false
}

func arrangeMRTGLegendRows(items []pngLegendItem, availableWidth int, rows int, labelsOnly bool) (pngLegendPlan, bool) {
	if rows <= 0 || len(items) == 0 {
		return pngLegendPlan{LabelsOnly: labelsOnly}, true
	}
	if rows == 1 {
		width := mrtgLegendRowWidth(items)
		if width > availableWidth {
			return pngLegendPlan{}, false
		}
		return pngLegendPlan{Rows: [][]pngLegendItem{items}, LabelsOnly: labelsOnly}, true
	}
	bestSplit := -1
	bestWidth := math.MaxInt
	for split := 1; split < len(items); split++ {
		row1 := items[:split]
		row2 := items[split:]
		width1 := mrtgLegendRowWidth(row1)
		width2 := mrtgLegendRowWidth(row2)
		if width1 > availableWidth || width2 > availableWidth {
			continue
		}
		maxWidth := maxInt(width1, width2)
		if maxWidth < bestWidth {
			bestWidth = maxWidth
			bestSplit = split
		}
	}
	if bestSplit < 0 {
		return pngLegendPlan{}, false
	}
	return pngLegendPlan{
		Rows:       [][]pngLegendItem{items[:bestSplit], items[bestSplit:]},
		LabelsOnly: labelsOnly,
	}, true
}

func forceMRTGLegendPlan(items []pngLegendItem, labelsOnly bool) pngLegendPlan {
	if len(items) <= 1 {
		return pngLegendPlan{Rows: [][]pngLegendItem{items}, LabelsOnly: labelsOnly}
	}
	split := (len(items) + 1) / 2
	return pngLegendPlan{
		Rows:       [][]pngLegendItem{items[:split], items[split:]},
		LabelsOnly: labelsOnly,
	}
}

func mrtgLegendRowWidth(items []pngLegendItem) int {
	if len(items) == 0 {
		return 0
	}
	width := 0
	for index, item := range items {
		width += item.Width
		if index > 0 {
			width += measurePNGTextWidth("   ")
		}
	}
	return width
}

func resolveMRTGYRange(spec *Spec, seriesData []pngSeriesData) (float64, float64) {
	minValue := 0.0
	maxValue := 0.0
	hasValue := false
	for _, data := range seriesData {
		for _, point := range data.Points {
			if !hasValue {
				minValue = point.Value
				maxValue = point.Value
				hasValue = true
			} else {
				minValue = math.Min(minValue, point.Value)
				maxValue = math.Max(maxValue, point.Value)
			}
		}
	}
	for _, axis := range spec.Axes.Y {
		if axis.Extent == nil {
			continue
		}
		if v, ok := toFloat64(axis.Extent.Min); ok {
			if !hasValue {
				minValue = v
				maxValue = v
				hasValue = true
			} else {
				minValue = math.Min(minValue, v)
				maxValue = math.Max(maxValue, v)
			}
		}
		if v, ok := toFloat64(axis.Extent.Max); ok {
			if !hasValue {
				minValue = v
				maxValue = v
				hasValue = true
			} else {
				minValue = math.Min(minValue, v)
				maxValue = math.Max(maxValue, v)
			}
		}
	}
	if !hasValue {
		return 0, 1
	}
	if minValue > 0 {
		minValue = 0
	}
	if minValue == maxValue {
		if minValue == 0 {
			maxValue = 1
		} else {
			maxValue = minValue * 1.1
		}
	}
	span := maxValue - minValue
	niceMax := niceCeil(maxValue + span*0.03)
	if niceMax <= minValue {
		niceMax = minValue + 1
	}
	return minValue, niceMax
}

func buildMRTGYTicks(minValue float64, maxValue float64, plot pngRect) []pngTick {
	count := 5
	step := (maxValue - minValue) / float64(count-1)
	ret := make([]pngTick, 0, count)
	for i := 0; i < count; i++ {
		value := minValue + step*float64(i)
		ratio := (value - minValue) / (maxValue - minValue)
		y := plot.Y + plot.Height - int(math.Round(float64(plot.Height)*ratio))
		ret = append(ret, pngTick{
			Value: value,
			Pos:   y,
			Label: formatMRTGNumber(value),
		})
	}
	slices.Reverse(ret)
	return ret
}

func buildMRTGXTimeTicks(start int64, end int64, plot pngRect, timeOptions resolvedOutputTimeOptions, measureLabelWidth func(string) int) []pngTick {
	startTime := time.Unix(0, start).In(timeOptions.Location)
	endTime := time.Unix(0, end).In(timeOptions.Location)
	span := endTime.Sub(startTime)
	spec := chooseMRTGTickSpec(span, plot.Width, measureLabelWidth)
	step := spec.Step
	first := startTime.Truncate(step)
	if first.Before(startTime) {
		first = first.Add(step)
	}
	ret := []pngTick{}
	for current := first; !current.After(endTime); current = current.Add(step) {
		ret = append(ret, pngTick{
			Pos:   projectTimeToX(current.UnixNano(), start, end, plot),
			Label: formatMRTGTimeTick(current, spec.Format),
		})
		if len(ret) > 16 {
			break
		}
	}
	if len(ret) < 2 {
		for i := 1; i <= 3; i++ {
			ts := start + int64(float64(end-start)*(float64(i)/4.0))
			current := time.Unix(0, ts).In(timeOptions.Location)
			ret = append(ret, pngTick{
				Pos:   projectTimeToX(ts, start, end, plot),
				Label: formatMRTGTimeTick(current, spec.Format),
			})
		}
	}
	return ret
}

func chooseMRTGTickSpec(span time.Duration, plotWidth int, measureLabelWidth func(string) int) pngTimeTickSpec {
	candidates := mrtgTickCandidates(span)
	if len(candidates) == 0 {
		return pngTimeTickSpec{Step: time.Hour, Format: "15:04"}
	}
	if measureLabelWidth == nil {
		measureLabelWidth = rendererMeasureTickLabelWidth
	}
	minSpacing := 18
	for _, candidate := range candidates {
		labelWidth := measureLabelWidth(sampleMRTGTimeTickLabel(candidate, span))
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
	return candidates[len(candidates)-1]
}

func formatMRTGTimeTick(ts time.Time, format string) string {
	return ts.Format(format)
}

func mrtgTickCandidates(span time.Duration) []pngTimeTickSpec {
	switch {
	case span <= 2*time.Minute:
		return []pngTimeTickSpec{
			{Step: 10 * time.Second, Format: "15:04:05"},
			{Step: 15 * time.Second, Format: "15:04:05"},
			{Step: 30 * time.Second, Format: "15:04:05"},
			{Step: time.Minute, Format: "15:04"},
		}
	case span <= 15*time.Minute:
		return []pngTimeTickSpec{
			{Step: time.Minute, Format: "15:04"},
			{Step: 2 * time.Minute, Format: "15:04"},
			{Step: 5 * time.Minute, Format: "15:04"},
			{Step: 10 * time.Minute, Format: "15:04"},
			{Step: 15 * time.Minute, Format: "15:04"},
		}
	case span <= 3*time.Hour:
		return []pngTimeTickSpec{
			{Step: 5 * time.Minute, Format: "15:04"},
			{Step: 10 * time.Minute, Format: "15:04"},
			{Step: 15 * time.Minute, Format: "15:04"},
			{Step: 30 * time.Minute, Format: "15:04"},
			{Step: time.Hour, Format: "15:04"},
		}
	case span <= 24*time.Hour:
		return []pngTimeTickSpec{
			{Step: 30 * time.Minute, Format: "15:04"},
			{Step: time.Hour, Format: "15:04"},
			{Step: 2 * time.Hour, Format: "15:04"},
			{Step: 3 * time.Hour, Format: "15:04"},
			{Step: 4 * time.Hour, Format: "15:04"},
			{Step: 6 * time.Hour, Format: "15:04"},
		}
	case span <= 72*time.Hour:
		return []pngTimeTickSpec{
			{Step: 2 * time.Hour, Format: "15h"},
			{Step: 4 * time.Hour, Format: "15h"},
			{Step: 6 * time.Hour, Format: "15h"},
			{Step: 12 * time.Hour, Format: "01-02 15h"},
			{Step: 24 * time.Hour, Format: "01-02"},
		}
	case span <= 14*24*time.Hour:
		return []pngTimeTickSpec{
			{Step: 12 * time.Hour, Format: "01-02 15h"},
			{Step: 24 * time.Hour, Format: "01-02"},
			{Step: 2 * 24 * time.Hour, Format: "01-02"},
			{Step: 7 * 24 * time.Hour, Format: "01-02"},
		}
	case span <= 45*24*time.Hour:
		return []pngTimeTickSpec{
			{Step: 24 * time.Hour, Format: "01-02"},
			{Step: 2 * 24 * time.Hour, Format: "01-02"},
			{Step: 7 * 24 * time.Hour, Format: "2006-01-02"},
			{Step: 14 * 24 * time.Hour, Format: "2006-01-02"},
		}
	default:
		return []pngTimeTickSpec{
			{Step: 7 * 24 * time.Hour, Format: "2006-01-02"},
			{Step: 14 * 24 * time.Hour, Format: "2006-01-02"},
			{Step: 30 * 24 * time.Hour, Format: "2006-01"},
			{Step: 90 * 24 * time.Hour, Format: "2006-01"},
		}
	}
}

func sampleMRTGTimeTickLabel(spec pngTimeTickSpec, span time.Duration) string {
	base := time.Date(2026, 4, 3, 23, 59, 58, 0, time.UTC)
	if span > 24*time.Hour && spec.Step < 24*time.Hour {
		base = time.Date(2026, 4, 30, 23, 0, 0, 0, time.UTC)
	}
	return formatMRTGTimeTick(base, spec.Format)
}

func projectTimeToX(value int64, start int64, end int64, plot pngRect) int {
	if end <= start {
		return plot.X
	}
	ratio := float64(value-start) / float64(end-start)
	return plot.X + int(math.Round(float64(plot.Width)*ratio))
}

func projectValueToY(value float64, minValue float64, maxValue float64, plot pngRect) int {
	if maxValue <= minValue {
		return plot.Y + plot.Height
	}
	ratio := (value - minValue) / (maxValue - minValue)
	return plot.Y + plot.Height - int(math.Round(float64(plot.Height)*ratio))
}

func (r *pngRenderer) drawMRTGGrid(xTicks []pngTick, yTicks []pngTick) {
	for _, tick := range yTicks {
		r.drawDottedLine(r.layout.Plot.X, tick.Pos, r.layout.Plot.X+r.layout.Plot.Width, tick.Pos, r.theme.Grid, 1, 1, 2)
	}
	for _, tick := range xTicks {
		r.drawDottedLine(tick.Pos, r.layout.Plot.Y, tick.Pos, r.layout.Plot.Y+r.layout.Plot.Height, r.theme.Grid, 1, 1, 2)
	}
}

func (r *pngRenderer) drawMRTGAxes(xTicks []pngTick, yTicks []pngTick) {
	for _, tick := range yTicks {
		r.drawText(r.layout.Plot.X-18, tick.Pos+r.ascent/3, tick.Label, r.theme.Text, "right")
	}
	for _, tick := range selectMRTGVisibleXTicks(xTicks, r.measureTickLabelWidth) {
		r.drawText(tick.Pos, r.layout.XTickBaseY, tick.Label, r.theme.Text, "center")
	}
	r.fillCircle(r.layout.Plot.X+r.layout.Plot.Width, r.layout.Plot.Y+r.layout.Plot.Height, 2, r.theme.Annotation)
}

func (r *pngRenderer) drawMRTGTimeSeries(spec *Spec, seriesData []pngSeriesData, minValue float64, maxValue float64, start int64, end int64) {
	baselineY := projectValueToY(minValue, minValue, maxValue, r.layout.Plot)
	for idx := range seriesData {
		for i := range seriesData[idx].Points {
			seriesData[idx].Points[i].Y = projectValueToY(seriesData[idx].Points[i].Value, minValue, maxValue, r.layout.Plot)
		}
	}
	for _, annotation := range spec.Annotations {
		if annotation.Kind != AnnotationKindLine || !isXAxis(spec, annotation.Axis) {
			continue
		}
		ts, ok := parseTimeValueWithDomain(annotation.Value, spec.Domain)
		if !ok {
			continue
		}
		x := projectTimeToX(ts.UnixNano(), start, end, r.layout.Plot)
		r.drawLine(float64(x), float64(r.layout.Plot.Y), float64(x), float64(r.layout.Plot.Y+r.layout.Plot.Height), r.theme.Annotation, 1)
	}
	for _, series := range seriesData {
		if series.IsAreaFilled {
			r.fillArea(series.Points, baselineY, series.FillColor)
		}
		if !series.IsPrimary {
			r.strokePolyline(series.Points, series.LineColor, 1)
		}
	}
	for _, series := range spec.Series {
		if series.Representation.Kind != RepresentationEventRange {
			continue
		}
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
			from, okFrom := parseTimeValueWithDomain(values[fromIndex], spec.Domain)
			to, okTo := parseTimeValueWithDomain(values[toIndex], spec.Domain)
			if !okFrom || !okTo {
				continue
			}
			x1 := projectTimeToX(from.UnixNano(), start, end, r.layout.Plot)
			x2 := projectTimeToX(to.UnixNano(), start, end, r.layout.Plot)
			r.drawLine(float64(x1), float64(r.layout.Plot.Y), float64(x1), float64(r.layout.Plot.Y+r.layout.Plot.Height), r.theme.Annotation, 1)
			r.drawLine(float64(x2), float64(r.layout.Plot.Y), float64(x2), float64(r.layout.Plot.Y+r.layout.Plot.Height), r.theme.Annotation, 1)
		}
	}
}

func (r *pngRenderer) drawMRTGStats(seriesData []pngSeriesData, rect pngRect) {
	if len(seriesData) == 0 {
		return
	}
	r.fillRect(rectToImage(rect), r.theme.StatsBackground)
	rows := r.layout.Legend.Rows
	if len(rows) == 0 {
		return
	}
	totalHeight := len(rows)*r.lineHeight + maxInt(0, len(rows)-1)
	lineY := rect.Y + maxInt(r.lineHeight, (rect.Height-totalHeight)/2+r.ascent)
	for _, row := range rows {
		x := rect.X + maxInt(0, (rect.Width-mrtgLegendRowWidth(row))/2)
		for index, item := range row {
			for _, part := range item.Parts {
				r.drawText(x, lineY, part.Text, part.Color, "left")
				x += r.measureTextWidth(part.Text)
			}
			if index < len(row)-1 {
				x += r.measureTextWidth("   ")
			}
		}
		lineY += r.lineHeight + 1
	}
}

func (r *pngRenderer) fillArea(points []pngPoint, baselineY int, fill color.RGBA) {
	if len(points) == 0 {
		return
	}
	for i := 0; i < len(points)-1; i++ {
		left := points[i]
		right := points[i+1]
		if right.X < left.X {
			left, right = right, left
		}
		width := maxInt(1, right.X-left.X)
		for dx := 0; dx <= width; dx++ {
			x := left.X + dx
			ratio := float64(dx) / float64(width)
			y := int(math.Round(float64(left.Y) + float64(right.Y-left.Y)*ratio))
			if y > baselineY {
				y = baselineY
			}
			rect := image.Rect(x, y, x+1, baselineY)
			r.fillRect(rect, fill)
		}
	}
	last := points[len(points)-1]
	if last.Y < baselineY {
		r.fillRect(image.Rect(last.X, last.Y, last.X+1, baselineY), fill)
	}
}

func pngSeriesLabel(series Series) string {
	if series.Name != "" {
		return series.Name
	}
	if series.ID != "" {
		return series.ID
	}
	return "series"
}

func formatMRTGNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}

func formatMRTGStat(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}

func selectMRTGVisibleXTicks(ticks []pngTick, minGapFn func(string) int) []pngTick {
	if len(ticks) <= 1 {
		return ticks
	}
	ret := make([]pngTick, 0, len(ticks))
	lastRight := math.MinInt
	for i, tick := range ticks {
		halfWidth := minGapFn(tick.Label) / 2
		left := tick.Pos - halfWidth
		right := tick.Pos + halfWidth
		if i == 0 || i == len(ticks)-1 || left > lastRight {
			ret = append(ret, tick)
			lastRight = right
		}
	}
	if len(ret) == 0 {
		return []pngTick{ticks[0]}
	}
	return ret
}

func rendererMeasureTickLabelWidth(text string) int {
	if text == "" {
		return 8
	}
	return len(text)*7 + 8
}

func measurePNGTextWidth(text string) int {
	if text == "" {
		return 0
	}
	d := &font.Drawer{Face: basicfont.Face7x13}
	return d.MeasureString(text).Ceil()
}

func niceCeil(value float64) float64 {
	if value <= 0 {
		return 1
	}
	power := math.Pow(10, math.Floor(math.Log10(value)))
	for _, factor := range []float64{1, 1.3, 2, 2.5, 5, 10} {
		candidate := factor * power
		if candidate >= value {
			return candidate
		}
	}
	return 10 * power
}

func rectToImage(rect pngRect) image.Rectangle {
	return image.Rect(rect.X, rect.Y, rect.X+rect.Width, rect.Y+rect.Height)
}

func (r *pngRenderer) strokePolyline(points []pngPoint, lineColor color.RGBA, width int) {
	if len(points) < 2 {
		return
	}
	for i := 1; i < len(points); i++ {
		r.drawLine(float64(points[i-1].X), float64(points[i-1].Y), float64(points[i].X), float64(points[i].Y), lineColor, width)
	}
}

func (r *pngRenderer) fillRect(rect image.Rectangle, fill color.RGBA) {
	if rect.Empty() {
		return
	}
	stddraw.Draw(r.img, rect, image.NewUniform(fill), image.Point{}, stddraw.Over)
}

func (r *pngRenderer) strokeRect(rect image.Rectangle, stroke color.RGBA) {
	if rect.Empty() {
		return
	}
	r.fillRect(image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+1), stroke)
	r.fillRect(image.Rect(rect.Min.X, rect.Max.Y-1, rect.Max.X, rect.Max.Y), stroke)
	r.fillRect(image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+1, rect.Max.Y), stroke)
	r.fillRect(image.Rect(rect.Max.X-1, rect.Min.Y, rect.Max.X, rect.Max.Y), stroke)
}

func (r *pngRenderer) strokeRectInset(rect image.Rectangle, inset int, stroke color.RGBA) {
	if inset <= 0 {
		r.strokeRect(rect, stroke)
		return
	}
	inner := image.Rect(rect.Min.X+inset, rect.Min.Y+inset, rect.Max.X-inset, rect.Max.Y-inset)
	if inner.Empty() {
		return
	}
	r.strokeRect(inner, stroke)
}

func (r *pngRenderer) drawText(x int, y int, text string, fill color.RGBA, align string) {
	if text == "" {
		return
	}
	d := &font.Drawer{Dst: r.img, Src: image.NewUniform(fill), Face: r.face}
	width := d.MeasureString(text).Ceil()
	switch align {
	case "center":
		x -= width / 2
	case "right":
		x -= width
	}
	d.Dot = fixed.P(x, y)
	d.DrawString(text)
}

func (r *pngRenderer) drawRotatedTextCCW(x int, centerY int, text string, fill color.RGBA) {
	if text == "" {
		return
	}
	textWidth := r.measureTextWidth(text)
	textHeight := r.lineHeight
	tmp := image.NewRGBA(image.Rect(0, 0, textWidth+2, textHeight+2))
	d := &font.Drawer{
		Dst:  tmp,
		Src:  image.NewUniform(fill),
		Face: r.face,
		Dot:  fixed.P(1, 1+r.ascent),
	}
	d.DrawString(text)
	destX := x - textHeight/2
	destY := centerY - textWidth/2
	for sy := 0; sy < tmp.Bounds().Dy(); sy++ {
		for sx := 0; sx < tmp.Bounds().Dx(); sx++ {
			c := tmp.RGBAAt(sx, sy)
			if c.A == 0 {
				continue
			}
			dx := destX + sy
			dy := destY + (tmp.Bounds().Dx() - 1 - sx)
			if image.Pt(dx, dy).In(r.img.Bounds()) {
				r.img.SetRGBA(dx, dy, c)
			}
		}
	}
}

func (r *pngRenderer) drawLine(x1 float64, y1 float64, x2 float64, y2 float64, fill color.RGBA, width int) {
	if width < 1 {
		width = 1
	}
	x0 := int(math.Round(x1))
	y0 := int(math.Round(y1))
	xEnd := int(math.Round(x2))
	yEnd := int(math.Round(y2))
	dx := int(math.Abs(float64(xEnd - x0)))
	sx := -1
	if x0 < xEnd {
		sx = 1
	}
	dy := -int(math.Abs(float64(yEnd - y0)))
	sy := -1
	if y0 < yEnd {
		sy = 1
	}
	err := dx + dy
	for {
		r.drawBrush(x0, y0, fill, width)
		if x0 == xEnd && y0 == yEnd {
			break
		}
		e2 := err * 2
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func (r *pngRenderer) drawDottedLine(x1 int, y1 int, x2 int, y2 int, fill color.RGBA, width int, dot int, gap int) {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	stepX := dx / length
	stepY := dy / length
	for offset := 0.0; offset <= length; offset += float64(dot + gap) {
		segmentEnd := math.Min(offset+float64(dot), length)
		startX := float64(x1) + stepX*offset
		startY := float64(y1) + stepY*offset
		endX := float64(x1) + stepX*segmentEnd
		endY := float64(y1) + stepY*segmentEnd
		r.drawLine(startX, startY, endX, endY, fill, width)
	}
}

func (r *pngRenderer) drawBrush(x int, y int, fill color.RGBA, width int) {
	half := width / 2
	for yy := y - half; yy <= y+half; yy++ {
		for xx := x - half; xx <= x+half; xx++ {
			if image.Pt(xx, yy).In(r.img.Bounds()) {
				r.img.SetRGBA(xx, yy, fill)
			}
		}
	}
}

func (r *pngRenderer) fillCircle(cx int, cy int, radius int, fill color.RGBA) {
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= radius*radius && image.Pt(x, y).In(r.img.Bounds()) {
				r.img.SetRGBA(x, y, fill)
			}
		}
	}
}

func (r *pngRenderer) measureTextWidth(text string) int {
	if text == "" {
		return 0
	}
	d := &font.Drawer{Face: r.face}
	return d.MeasureString(text).Ceil()
}

func (r *pngRenderer) measureTickLabelWidth(text string) int {
	return r.measureTextWidth(text) + 8
}

func styleColor(style map[string]any, key string) (color.RGBA, bool) {
	if style == nil {
		return color.RGBA{}, false
	}
	raw, ok := style[key]
	if !ok {
		return color.RGBA{}, false
	}
	text, ok := raw.(string)
	if !ok {
		return color.RGBA{}, false
	}
	colorValue, err := parseSimpleColor(text)
	if err != nil {
		return color.RGBA{}, false
	}
	return colorValue, true
}

func withOpacity(base color.RGBA, opacity float64) color.RGBA {
	base.A = uint8(math.Round(float64(base.A) * clampOpacity(opacity)))
	return base
}

func clampOpacity(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func parseSimpleColor(value string) (color.RGBA, error) {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return color.RGBA{}, fmt.Errorf("advn: color must not be empty")
	}
	switch text {
	case "white":
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}, nil
	case "black":
		return color.RGBA{A: 255}, nil
	case "gray", "grey":
		return color.RGBA{R: 128, G: 128, B: 128, A: 255}, nil
	case "red":
		return color.RGBA{R: 255, A: 255}, nil
	case "green":
		return color.RGBA{G: 128, A: 255}, nil
	case "blue":
		return color.RGBA{B: 255, A: 255}, nil
	case "yellow":
		return color.RGBA{R: 255, G: 255, A: 255}, nil
	}
	if strings.HasPrefix(text, "#") {
		return parseHexColor(text)
	}
	return color.RGBA{}, fmt.Errorf("advn: unsupported color %q", value)
}

func parseHexColor(value string) (color.RGBA, error) {
	hex := strings.TrimPrefix(value, "#")
	switch len(hex) {
	case 3:
		return color.RGBA{R: duplicateHexNibble(hex[0]), G: duplicateHexNibble(hex[1]), B: duplicateHexNibble(hex[2]), A: 255}, nil
	case 6:
		n, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return color.RGBA{}, fmt.Errorf("advn: invalid color %q", value)
		}
		return color.RGBA{R: uint8(n >> 16), G: uint8((n >> 8) & 0xff), B: uint8(n & 0xff), A: 255}, nil
	case 8:
		n, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return color.RGBA{}, fmt.Errorf("advn: invalid color %q", value)
		}
		return color.RGBA{R: uint8(n >> 24), G: uint8((n >> 16) & 0xff), B: uint8((n >> 8) & 0xff), A: uint8(n & 0xff)}, nil
	default:
		return color.RGBA{}, fmt.Errorf("advn: invalid color %q", value)
	}
}

func duplicateHexNibble(value byte) uint8 {
	nibble, err := strconv.ParseUint(string([]byte{value}), 16, 8)
	if err != nil {
		return 0
	}
	return uint8(nibble<<4 | nibble)
}

func mustParseColor(value string) color.RGBA {
	ret, err := parseSimpleColor(value)
	if err != nil {
		panic(err)
	}
	return ret
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
