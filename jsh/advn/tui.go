package advn

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const tuiDefaultWidth = 40
const tuiDefaultTableRows = 8
const tuiSparklineHeight = 3

type TUIOptions struct {
	Width   int  `json:"width,omitempty"`
	Rows    int  `json:"rows,omitempty"`
	Compact bool `json:"compact,omitempty"`
}

type TUIBlock struct {
	Type    string         `json:"type"`
	Title   string         `json:"title,omitempty"`
	Stats   []TUIStat      `json:"stats,omitempty"`
	Lines   []string       `json:"lines,omitempty"`
	Columns []string       `json:"columns,omitempty"`
	Rows    []any          `json:"rows,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

type TUIStat struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func ToTUIBlocks(spec *Spec) ([]TUIBlock, error) {
	return ToTUIBlocksWithOptions(spec, nil)
}

func ToTUIBlocksWithOptions(spec *Spec, options *TUIOptions) ([]TUIBlock, error) {
	if spec == nil {
		return nil, fmt.Errorf("advn: spec is nil")
	}
	resolved := normalizeTUIOptions(options)
	spec = spec.Normalize()
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	blocks := []TUIBlock{buildTUISummaryBlock(spec)}
	for _, series := range spec.Series {
		seriesBlocks, err := buildTUISeriesBlocks(spec, series, resolved)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, seriesBlocks...)
	}
	if len(spec.Annotations) > 0 {
		blocks = append(blocks, buildTUIAnnotationBlock(spec, spec.Annotations))
	}
	return blocks, nil
}

func normalizeTUIOptions(options *TUIOptions) TUIOptions {
	resolved := TUIOptions{}
	if options != nil {
		resolved = *options
	}
	if resolved.Width <= 0 {
		resolved.Width = tuiDefaultWidth
	}
	if resolved.Rows <= 0 {
		resolved.Rows = tuiDefaultTableRows
	}
	return resolved
}

func buildTUISummaryBlock(spec *Spec) TUIBlock {
	stats := []TUIStat{{Label: "series", Value: strconv.Itoa(len(spec.Series))}, {Label: "annotations", Value: strconv.Itoa(len(spec.Annotations))}}
	if spec.Domain.Kind != "" {
		stats = append(stats, TUIStat{Label: "domain", Value: spec.Domain.Kind})
	}
	if spec.Meta.Producer != "" {
		stats = append(stats, TUIStat{Label: "producer", Value: spec.Meta.Producer})
	}
	lines := []string{}
	if spec.Domain.From != nil || spec.Domain.To != nil {
		lines = append(lines, fmt.Sprintf("range: %s -> %s", formatTimeValue(spec.Domain.From, spec.Domain), formatTimeValue(spec.Domain.To, spec.Domain)))
	}
	return TUIBlock{Type: "summary", Title: "ADVN", Stats: stats, Lines: lines}
}

func buildTUISeriesBlocks(spec *Spec, series Series, options TUIOptions) ([]TUIBlock, error) {
	name := series.Name
	if name == "" {
		name = series.ID
	}
	blocks := []TUIBlock{}
	if !options.Compact {
		blocks = append(blocks, TUIBlock{
			Type:  "series-summary",
			Title: name,
			Stats: buildTUISeriesStats(series),
			Meta: map[string]any{
				"representation": series.Representation.Kind,
				"axis":           series.Axis,
			},
		})
	}
	vizBlock, err := buildTUIVisualizationBlock(spec, series, name, options)
	if err != nil {
		return nil, err
	}
	if vizBlock.Type != "" {
		blocks = append(blocks, vizBlock)
	}
	if !options.Compact {
		tableBlock := buildTUITableBlock(series, name, options.Rows)
		if len(tableBlock.Rows) > 0 {
			blocks = append(blocks, tableBlock)
		}
	}
	return blocks, nil
}

func buildTUISeriesStats(series Series) []TUIStat {
	stats := []TUIStat{{Label: "kind", Value: series.Representation.Kind}, {Label: "rows", Value: strconv.Itoa(len(series.Data))}}
	if series.Quality.RowCount > 0 {
		stats = append(stats, TUIStat{Label: "rowCount", Value: strconv.Itoa(series.Quality.RowCount)})
	}
	if series.Quality.Coverage > 0 {
		stats = append(stats, TUIStat{Label: "coverage", Value: formatFloat(series.Quality.Coverage)})
	}
	switch series.Representation.Kind {
	case RepresentationRawPoint, RepresentationTimeBucketValue:
		if minValue, maxValue, avgValue, ok := numericSeriesStats(series, "value"); ok {
			stats = append(stats,
				TUIStat{Label: "min", Value: formatFloat(minValue)},
				TUIStat{Label: "max", Value: formatFloat(maxValue)},
				TUIStat{Label: "avg", Value: formatFloat(avgValue)},
			)
		}
	case RepresentationTimeBucketBand:
		if minValue, maxValue, avgValue, ok := bandSeriesStats(series); ok {
			stats = append(stats,
				TUIStat{Label: "min", Value: formatFloat(minValue)},
				TUIStat{Label: "max", Value: formatFloat(maxValue)},
				TUIStat{Label: "avg", Value: formatFloat(avgValue)},
			)
		}
	case RepresentationDistributionHistogram:
		if total, bins, ok := histogramStats(series); ok {
			stats = append(stats, TUIStat{Label: "bins", Value: strconv.Itoa(bins)}, TUIStat{Label: "total", Value: formatFloat(total)})
		}
	case RepresentationDistributionBoxplot:
		stats = append(stats, TUIStat{Label: "groups", Value: strconv.Itoa(len(series.Data))})
		if outliers, ok := series.Extra["outliers"].([]any); ok {
			stats = append(stats, TUIStat{Label: "outliers", Value: strconv.Itoa(len(outliers))})
		}
	case RepresentationEventPoint, RepresentationEventRange:
		stats = append(stats, TUIStat{Label: "events", Value: strconv.Itoa(len(series.Data))})
	}
	return stats
}

func buildTUIVisualizationBlock(spec *Spec, series Series, name string, options TUIOptions) (TUIBlock, error) {
	switch series.Representation.Kind {
	case RepresentationRawPoint, RepresentationTimeBucketValue:
		values := collectNumericField(series, "value", 1)
		return TUIBlock{Type: "sparkline", Title: name, Lines: buildSparklineLines(spec, series, values, options.Width)}, nil
	case RepresentationTimeBucketBand:
		minValues := collectNumericField(series, "min", -1)
		avgValues := collectNumericField(series, "avg", -1)
		maxValues := collectNumericField(series, "max", -1)
		lines := []string{}
		if len(maxValues) > 0 {
			lines = append(lines, "max "+renderSparkline(maxValues, options.Width))
		}
		if len(avgValues) > 0 {
			lines = append(lines, "avg "+renderSparkline(avgValues, options.Width))
		}
		if len(minValues) > 0 {
			lines = append(lines, "min "+renderSparkline(minValues, options.Width))
		}
		return TUIBlock{Type: "bandline", Title: name, Lines: lines}, nil
	case RepresentationDistributionHistogram:
		return TUIBlock{Type: "bars", Title: name, Lines: buildHistogramLines(series, options.Rows, options.Width)}, nil
	case RepresentationDistributionBoxplot:
		return TUIBlock{Type: "box-summary", Title: name, Lines: buildBoxplotLines(series, options.Rows)}, nil
	case RepresentationEventPoint:
		return TUIBlock{Type: "event-list", Title: name, Lines: buildEventPointLines(series, spec.Domain, options.Rows)}, nil
	case RepresentationEventRange:
		lines := buildEventRangeLines(spec, series, options.Width, options.Rows)
		return TUIBlock{Type: "timeline", Title: name, Lines: lines}, nil
	default:
		return TUIBlock{}, fmt.Errorf("advn: unsupported tui representation %q", series.Representation.Kind)
	}
}

func buildTUITableBlock(series Series, name string, limit int) TUIBlock {
	rows := []any{}
	rowLimit := len(series.Data)
	if rowLimit > limit {
		rowLimit = limit
	}
	for index := 0; index < rowLimit; index++ {
		rows = append(rows, series.Data[index])
	}
	meta := map[string]any{"totalRows": len(series.Data)}
	if len(series.Data) > rowLimit {
		meta["truncated"] = true
	}
	return TUIBlock{
		Type:    "table",
		Title:   name + " data",
		Columns: series.Representation.Fields,
		Rows:    rows,
		Meta:    meta,
	}
}

func buildTUIAnnotationBlock(spec *Spec, annotations []Annotation) TUIBlock {
	lines := make([]string, 0, len(annotations))
	for _, annotation := range annotations {
		switch annotation.Kind {
		case AnnotationKindLine:
			value := formatAny(annotation.Value)
			if isXAxis(spec, annotation.Axis) {
				value = formatTimeValue(annotation.Value, spec.Domain)
			}
			lines = append(lines, fmt.Sprintf("line %s=%s %s", annotation.Axis, value, annotation.Label))
		case AnnotationKindRange:
			from := formatAny(annotation.From)
			to := formatAny(annotation.To)
			if isXAxis(spec, annotation.Axis) {
				from = formatTimeValue(annotation.From, spec.Domain)
				to = formatTimeValue(annotation.To, spec.Domain)
			}
			lines = append(lines, fmt.Sprintf("range %s %s -> %s %s", annotation.Axis, from, to, annotation.Label))
		case AnnotationKindPoint:
			at := formatAny(annotation.At)
			if isXAxis(spec, annotation.Axis) {
				at = formatTimeValue(annotation.At, spec.Domain)
			}
			lines = append(lines, fmt.Sprintf("point %s at %s value=%s %s", annotation.Axis, at, formatAny(annotation.Value), annotation.Label))
		}
	}
	return TUIBlock{Type: "annotations", Title: "Annotations", Lines: lines}
}

func numericSeriesStats(series Series, field string) (float64, float64, float64, bool) {
	values := collectNumericField(series, field, 1)
	if len(values) == 0 {
		return 0, 0, 0, false
	}
	return summarizeFloats(values)
}

func bandSeriesStats(series Series) (float64, float64, float64, bool) {
	minValues := collectNumericField(series, "min", -1)
	maxValues := collectNumericField(series, "max", -1)
	avgValues := collectNumericField(series, "avg", -1)
	if len(minValues) == 0 && len(maxValues) == 0 && len(avgValues) == 0 {
		return 0, 0, 0, false
	}
	minValue := 0.0
	maxValue := 0.0
	avgValue := 0.0
	haveMin := false
	haveMax := false
	haveAvg := false
	if len(minValues) > 0 {
		minValue, _, _, _ = summarizeFloats(minValues)
		haveMin = true
	}
	if len(maxValues) > 0 {
		_, maxValue, _, _ = summarizeFloats(maxValues)
		haveMax = true
	}
	if len(avgValues) > 0 {
		_, _, avgValue, _ = summarizeFloats(avgValues)
		haveAvg = true
	}
	if !haveMin && haveAvg {
		minValue = avgValue
	}
	if !haveMax && haveAvg {
		maxValue = avgValue
	}
	if !haveAvg && haveMin && haveMax {
		avgValue = (minValue + maxValue) / 2
	}
	return minValue, maxValue, avgValue, true
}

func histogramStats(series Series) (float64, int, bool) {
	countIndex := fieldIndex(series.Representation.Fields, "count")
	if countIndex < 0 {
		countIndex = 2
	}
	total := 0.0
	bins := 0
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || countIndex >= len(values) {
			continue
		}
		count, ok := toFloat64(values[countIndex])
		if !ok {
			continue
		}
		total += count
		bins++
	}
	return total, bins, bins > 0
}

func buildHistogramLines(series Series, limit int, width int) []string {
	startIndex := fieldIndex(series.Representation.Fields, "binStart")
	endIndex := fieldIndex(series.Representation.Fields, "binEnd")
	countIndex := fieldIndex(series.Representation.Fields, "count")
	if startIndex < 0 {
		startIndex = 0
	}
	if endIndex < 0 {
		endIndex = 1
	}
	if countIndex < 0 {
		countIndex = 2
	}
	maxCount := 0.0
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || countIndex >= len(values) {
			continue
		}
		count, ok := toFloat64(values[countIndex])
		if ok && count > maxCount {
			maxCount = count
		}
	}
	if maxCount <= 0 {
		return nil
	}
	barWidth := width - 18
	if barWidth < 8 {
		barWidth = 8
	}
	lines := []string{}
	for index, row := range series.Data {
		if index >= limit {
			lines = append(lines, fmt.Sprintf("... %d more bins", len(series.Data)-limit))
			break
		}
		values, ok := row.([]any)
		if !ok || startIndex >= len(values) || endIndex >= len(values) || countIndex >= len(values) {
			continue
		}
		count, ok := toFloat64(values[countIndex])
		if !ok {
			continue
		}
		barLen := int(math.Round((count / maxCount) * float64(barWidth)))
		if barLen == 0 && count > 0 {
			barLen = 1
		}
		label := fmt.Sprintf("%v-%v", values[startIndex], values[endIndex])
		lines = append(lines, fmt.Sprintf("%-12s | %-*s %s", truncate(label, 12), barWidth, strings.Repeat("#", barLen), formatFloat(count)))
	}
	return lines
}

func buildBoxplotLines(series Series, limit int) []string {
	categoryIndex := fieldIndex(series.Representation.Fields, "category")
	lowIndex := fieldIndex(series.Representation.Fields, "low")
	q1Index := fieldIndex(series.Representation.Fields, "q1")
	medianIndex := fieldIndex(series.Representation.Fields, "median")
	q3Index := fieldIndex(series.Representation.Fields, "q3")
	highIndex := fieldIndex(series.Representation.Fields, "high")
	if categoryIndex < 0 {
		categoryIndex = 0
	}
	lines := []string{}
	for index, row := range series.Data {
		if index >= limit {
			lines = append(lines, fmt.Sprintf("... %d more groups", len(series.Data)-limit))
			break
		}
		values, ok := row.([]any)
		if !ok || highIndex >= len(values) {
			continue
		}
		lines = append(lines, fmt.Sprintf("%v | %v [%v | %v | %v] %v", values[categoryIndex], values[lowIndex], values[q1Index], values[medianIndex], values[q3Index], values[highIndex]))
	}
	if outliers, ok := series.Extra["outliers"].([]any); ok && len(outliers) > 0 {
		lines = append(lines, fmt.Sprintf("outliers: %d", len(outliers)))
	}
	return lines
}

func buildEventPointLines(series Series, domain Domain, limit int) []string {
	timeIndex := fieldIndex(series.Representation.Fields, "time")
	valueIndex := fieldIndex(series.Representation.Fields, "value")
	labelIndex := fieldIndex(series.Representation.Fields, "label")
	severityIndex := fieldIndex(series.Representation.Fields, "severity")
	if timeIndex < 0 {
		timeIndex = 0
	}
	if valueIndex < 0 {
		valueIndex = 1
	}
	lines := []string{}
	for index, row := range series.Data {
		if index >= limit {
			lines = append(lines, fmt.Sprintf("... %d more events", len(series.Data)-limit))
			break
		}
		values, ok := row.([]any)
		if !ok || valueIndex >= len(values) {
			continue
		}
		parts := []string{formatTimeValue(values[timeIndex], domain), formatAny(values[valueIndex])}
		if labelIndex >= 0 && labelIndex < len(values) {
			parts = append(parts, formatAny(values[labelIndex]))
		}
		if severityIndex >= 0 && severityIndex < len(values) {
			parts = append(parts, formatAny(values[severityIndex]))
		}
		lines = append(lines, strings.Join(parts, " | "))
	}
	return lines
}

func buildEventRangeLines(spec *Spec, series Series, width int, limit int) []string {
	fromIndex := fieldIndex(series.Representation.Fields, "from")
	toIndex := fieldIndex(series.Representation.Fields, "to")
	labelIndex := fieldIndex(series.Representation.Fields, "label")
	if fromIndex < 0 {
		fromIndex = 0
	}
	if toIndex < 0 {
		toIndex = 1
	}
	if start, end, ok := timeRange(spec.Domain.From, spec.Domain.To, spec.Domain); ok {
		strip := make([]byte, width)
		for index := range strip {
			strip[index] = '.'
		}
		lines := []string{}
		hidden := 0
		for _, row := range series.Data {
			values, ok := row.([]any)
			if !ok || toIndex >= len(values) {
				continue
			}
			fromTime, fromOK := parseTimeValueWithDomain(values[fromIndex], spec.Domain)
			toTime, toOK := parseTimeValueWithDomain(values[toIndex], spec.Domain)
			if !fromOK || !toOK || !toTime.After(start) || !end.After(fromTime) {
				continue
			}
			fromPos := clampPosition(fromTime, start, end, width)
			toPos := clampPosition(toTime, start, end, width)
			if toPos <= fromPos {
				toPos = fromPos + 1
			}
			if toPos > width {
				toPos = width
			}
			for index := fromPos; index < toPos; index++ {
				strip[index] = '='
			}
			if len(lines) >= limit {
				hidden++
				continue
			}
			if labelIndex >= 0 && labelIndex < len(values) {
				lines = append(lines, fmt.Sprintf("%s -> %s | %s", formatTimeValue(values[fromIndex], spec.Domain), formatTimeValue(values[toIndex], spec.Domain), formatAny(values[labelIndex])))
			} else {
				lines = append(lines, fmt.Sprintf("%s -> %s", formatTimeValue(values[fromIndex], spec.Domain), formatTimeValue(values[toIndex], spec.Domain)))
			}
		}
		if hidden > 0 {
			lines = append(lines, fmt.Sprintf("... %d more events", hidden))
		}
		return append([]string{string(strip)}, lines...)
	}
	lines := []string{}
	hidden := 0
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || toIndex >= len(values) {
			continue
		}
		if len(lines) >= limit {
			hidden++
			continue
		}
		if labelIndex >= 0 && labelIndex < len(values) {
			lines = append(lines, fmt.Sprintf("%s -> %s | %s", formatTimeValue(values[fromIndex], spec.Domain), formatTimeValue(values[toIndex], spec.Domain), formatAny(values[labelIndex])))
		} else {
			lines = append(lines, fmt.Sprintf("%s -> %s", formatTimeValue(values[fromIndex], spec.Domain), formatTimeValue(values[toIndex], spec.Domain)))
		}
	}
	if hidden > 0 {
		lines = append(lines, fmt.Sprintf("... %d more events", hidden))
	}
	return lines
}

func collectNumericField(series Series, field string, fallback int) []float64 {
	index := fieldIndex(series.Representation.Fields, field)
	if index < 0 {
		index = fallback
	}
	ret := []float64{}
	for _, row := range series.Data {
		values, ok := row.([]any)
		if !ok || index < 0 || index >= len(values) {
			continue
		}
		value, ok := toFloat64(values[index])
		if ok {
			ret = append(ret, value)
		}
	}
	return ret
}

func buildSparklineLines(spec *Spec, series Series, values []float64, width int) []string {
	sampled, minValue, maxValue, ok := prepareSparklineValues(values, width)
	if !ok {
		return nil
	}
	labels := buildSparklineYLabels(minValue, maxValue)
	labelWidth := 0
	for _, label := range labels {
		if len(label) > labelWidth {
			labelWidth = len(label)
		}
	}
	chart := renderSparklineChart(sampled, minValue, maxValue)
	lines := make([]string, 0, len(chart)+1)
	if xAxis := buildSparklineXAxis(spec, series, len(sampled)); xAxis != "" {
		lines = append(lines, strings.Repeat(" ", labelWidth+3)+xAxis)
	}
	for index, row := range chart {
		lines = append(lines, fmt.Sprintf("%*s ┤ %s", labelWidth, labels[index], row))
	}
	return lines
}

func prepareSparklineValues(values []float64, width int) ([]float64, float64, float64, bool) {
	if len(values) == 0 {
		return nil, 0, 0, false
	}
	sampled := sampleFloats(values, width)
	minValue, maxValue, _, _ := summarizeFloats(sampled)
	return sampled, minValue, maxValue, true
}

func renderSparkline(values []float64, width int) string {
	sampled, minValue, maxValue, ok := prepareSparklineValues(values, width)
	if !ok {
		return ""
	}
	if maxValue == minValue {
		return strings.Repeat("█", len(sampled))
	}
	levels := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	buf := make([]rune, len(sampled))
	for index, value := range sampled {
		ratio := (value - minValue) / (maxValue - minValue)
		level := int(math.Round(ratio * float64(len(levels)-1)))
		if level < 0 {
			level = 0
		}
		if level >= len(levels) {
			level = len(levels) - 1
		}
		buf[index] = levels[level]
	}
	return string(buf)
}

func renderSparklineChart(values []float64, minValue float64, maxValue float64) []string {
	rows := make([][]rune, tuiSparklineHeight)
	for row := range rows {
		rows[row] = make([]rune, len(values))
		fill := ' '
		if row == 1 {
			fill = '─'
		}
		for col := range rows[row] {
			rows[row][col] = fill
		}
	}
	if maxValue == minValue {
		for col := range rows[1] {
			rows[1][col] = '█'
		}
		return sparklineRowsToStrings(rows)
	}
	levels := []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	totalLevels := tuiSparklineHeight * (len(levels) - 1)
	for col, value := range values {
		filled := int(math.Round((value-minValue)/(maxValue-minValue)*float64(totalLevels-1))) + 1
		if filled < 1 {
			filled = 1
		}
		if filled > totalLevels {
			filled = totalLevels
		}
		remaining := filled
		for row := tuiSparklineHeight - 1; row >= 0; row-- {
			level := remaining
			if level > len(levels)-1 {
				level = len(levels) - 1
			}
			if level > 0 {
				rows[row][col] = levels[level]
			}
			remaining -= len(levels) - 1
			if remaining <= 0 {
				break
			}
		}
	}
	return sparklineRowsToStrings(rows)
}

func sparklineRowsToStrings(rows [][]rune) []string {
	ret := make([]string, 0, len(rows))
	for _, row := range rows {
		ret = append(ret, string(row))
	}
	return ret
}

func buildSparklineYLabels(minValue float64, maxValue float64) []string {
	baseline := 0.0
	if minValue > 0 || maxValue < 0 {
		baseline = (minValue + maxValue) / 2
	}
	return []string{formatFloat(maxValue), formatFloat(baseline), formatFloat(minValue)}
}

func buildSparklineXAxis(spec *Spec, series Series, width int) string {
	leftLabel, rightLabel, ok := sparklineXAxisLabels(spec, series)
	if !ok {
		return ""
	}
	return fitSparklineAxisLabels(leftLabel, rightLabel, width)
}

func sparklineXAxisLabels(spec *Spec, series Series) (string, string, bool) {
	if spec != nil && spec.Domain.Kind == DomainKindTime {
		if start, end, ok := timeRange(spec.Domain.From, spec.Domain.To, spec.Domain); ok {
			span := end.Sub(start)
			return formatSparklineTimeTick(start, span), formatSparklineTimeTick(end, span), true
		}
	}
	if len(series.Data) == 0 {
		return "", "", false
	}
	xIndex := 0
	switch series.Representation.Kind {
	case RepresentationTimeBucketValue:
		xIndex = timeDomainXIndex(series, spec.Domain)
	case RepresentationRawPoint:
		xIndex = rawPointXIndex(series, spec.Domain)
	}
	first, okFirst := sparklineRowValue(series.Data[0], xIndex)
	last, okLast := sparklineRowValue(series.Data[len(series.Data)-1], xIndex)
	if !okFirst || !okLast {
		return "", "", false
	}
	if spec != nil && spec.Domain.Kind == DomainKindTime {
		firstTime, okFirstTime := parseTimeValueWithDomain(first, spec.Domain)
		lastTime, okLastTime := parseTimeValueWithDomain(last, spec.Domain)
		if okFirstTime && okLastTime {
			span := lastTime.Sub(firstTime)
			if span < 0 {
				span = -span
			}
			return formatSparklineTimeTick(firstTime, span), formatSparklineTimeTick(lastTime, span), true
		}
	}
	return formatAny(first), formatAny(last), true
}

func sparklineRowValue(row any, index int) (any, bool) {
	values, ok := row.([]any)
	if !ok || index < 0 || index >= len(values) {
		return nil, false
	}
	return values[index], true
}

func fitSparklineAxisLabels(left string, right string, width int) string {
	if width <= 0 {
		return ""
	}
	if left == "" && right == "" {
		return ""
	}
	if left == "" {
		return fmt.Sprintf("%*s", width, truncate(right, width))
	}
	if right == "" {
		return truncate(left, width)
	}
	if len(left)+len(right) >= width {
		leftWidth := width - len(right) - 1
		if leftWidth <= 0 {
			return truncate(left, width)
		}
		return truncate(left, leftWidth) + " " + right
	}
	return left + strings.Repeat(" ", width-len(left)-len(right)) + right
}

func formatSparklineTimeTick(value time.Time, span time.Duration) string {
	switch {
	case span <= time.Minute:
		return value.UTC().Format("15:04:05")
	case span <= 6*time.Hour:
		return value.UTC().Format("15:04")
	case span <= 48*time.Hour:
		return value.UTC().Format("01-02 15:04")
	case span <= 180*24*time.Hour:
		return value.UTC().Format("2006-01-02")
	case span <= 2*365*24*time.Hour:
		return value.UTC().Format("2006-01")
	default:
		return value.UTC().Format("2006")
	}
}

func sampleFloats(values []float64, width int) []float64 {
	if len(values) <= width || width <= 0 {
		ret := make([]float64, len(values))
		copy(ret, values)
		return ret
	}
	ret := make([]float64, 0, width)
	step := float64(len(values)) / float64(width)
	for index := 0; index < width; index++ {
		start := int(math.Floor(float64(index) * step))
		end := int(math.Floor(float64(index+1) * step))
		if end <= start {
			end = start + 1
		}
		if end > len(values) {
			end = len(values)
		}
		segment := values[start:end]
		_, _, avgValue, _ := summarizeFloats(segment)
		ret = append(ret, avgValue)
	}
	return ret
}

func summarizeFloats(values []float64) (float64, float64, float64, bool) {
	if len(values) == 0 {
		return 0, 0, 0, false
	}
	minValue := values[0]
	maxValue := values[0]
	total := 0.0
	for _, value := range values {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
		total += value
	}
	return minValue, maxValue, total / float64(len(values)), true
}

func timeRange(from any, to any, domain Domain) (time.Time, time.Time, bool) {
	start, okStart := parseTimeValueWithDomain(from, domain)
	end, okEnd := parseTimeValueWithDomain(to, domain)
	if !okStart || !okEnd || !end.After(start) {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func clampPosition(at, start, end time.Time, width int) int {
	if !end.After(start) || width <= 0 {
		return 0
	}
	ratio := float64(at.Sub(start)) / float64(end.Sub(start))
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	pos := int(math.Floor(ratio * float64(width)))
	if pos < 0 {
		return 0
	}
	if pos >= width {
		return width - 1
	}
	return pos
}

func formatAny(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return formatFloat(typed)
	case float32:
		return formatFloat(float64(typed))
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func formatFloat(value float64) string {
	if math.Abs(value-math.Round(value)) < 1e-9 {
		return strconv.FormatFloat(value, 'f', 0, 64)
	}
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return text[:limit-3] + "..."
}
