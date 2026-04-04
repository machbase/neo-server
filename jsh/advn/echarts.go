package advn

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type EChartsOptions struct {
	Timeformat string `json:"timeformat,omitempty"`
	TZ         string `json:"tz,omitempty"`
}

func ToEChartsOption(spec *Spec) (map[string]any, error) {
	return ToEChartsOptionWithOptions(spec, nil)
}

func ToEChartsOptionWithOptions(spec *Spec, options *EChartsOptions) (map[string]any, error) {
	if spec == nil {
		return nil, fmt.Errorf("advn: spec is nil")
	}
	spec = spec.Normalize()
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	outputTime := OutputTimeOptions{}
	if options != nil {
		outputTime = OutputTimeOptions{Timeformat: options.Timeformat, TZ: options.TZ}
	}
	timeOptions, err := resolveOutputTimeOptions(spec.Domain, outputTime)
	if err != nil {
		return nil, err
	}

	yAxes, yAxisIndex := buildEChartsYAxes(spec)
	seriesList := make([]map[string]any, 0, len(spec.Series))
	legendData := make([]string, 0, len(spec.Series))
	xAxis := buildEChartsXAxis(spec, timeOptions)

	for _, item := range spec.Series {
		built, err := buildEChartsSeries(item, yAxisIndex, spec.Domain, timeOptions)
		if err != nil {
			return nil, err
		}
		for _, one := range built {
			if override, ok := one["_advnXAxis"].(map[string]any); ok {
				xAxis = mergeStyle(xAxis, override)
				delete(one, "_advnXAxis")
			}
			seriesList = append(seriesList, one)
			if name, ok := one["name"].(string); ok && name != "" {
				legendData = append(legendData, name)
			}
		}
	}
	if len(seriesList) == 0 {
		return nil, fmt.Errorf("advn: no supported series")
	}

	applyEChartsAnnotations(spec, preferredAnnotationTarget(seriesList), timeOptions)

	option := map[string]any{
		"tooltip": map[string]any{"trigger": "axis"},
		"xAxis":   xAxis,
		"series":  seriesList,
	}
	if len(yAxes) == 1 {
		option["yAxis"] = yAxes[0]
	} else {
		option["yAxis"] = yAxes
	}
	if len(legendData) > 0 {
		option["legend"] = map[string]any{"data": legendData}
	}
	if len(spec.View.DefaultZoom) == 2 {
		option["dataZoom"] = []map[string]any{{
			"type":  "slider",
			"start": spec.View.DefaultZoom[0],
			"end":   spec.View.DefaultZoom[1],
		}}
	}
	return option, nil
}

func buildEChartsXAxis(spec *Spec, timeOptions resolvedOutputTimeOptions) map[string]any {
	axisType := detectDefaultXAxisType(spec)
	if spec.Axes.X.Type != "" {
		axisType = echartsAxisType(spec.Axes.X.Type)
	}
	if axisType == "time" {
		axisType = echartsTimeAxisType(timeOptions)
	}
	ret := map[string]any{
		"type": axisType,
	}
	if spec.Axes.X.Label != "" {
		ret["name"] = spec.Axes.X.Label
	} else if spec.Axes.X.ID != "" {
		ret["name"] = spec.Axes.X.ID
	}
	if spec.Axes.X.Extent != nil {
		if spec.Axes.X.Extent.Min != nil {
			if spec.Domain.Kind == DomainKindTime {
				ret["min"] = normalizeTimeValueForEChartsWithOptions(spec.Axes.X.Extent.Min, spec.Domain, timeOptions)
			} else {
				ret["min"] = spec.Axes.X.Extent.Min
			}
		}
		if spec.Axes.X.Extent.Max != nil {
			if spec.Domain.Kind == DomainKindTime {
				ret["max"] = normalizeTimeValueForEChartsWithOptions(spec.Axes.X.Extent.Max, spec.Domain, timeOptions)
			} else {
				ret["max"] = spec.Axes.X.Extent.Max
			}
		}
	}
	return ret
}

func buildEChartsYAxes(spec *Spec) ([]map[string]any, map[string]int) {
	ordered := make([]Axis, 0, len(spec.Axes.Y)+len(spec.Series))
	indexByID := make(map[string]int, len(spec.Axes.Y)+len(spec.Series))

	addAxis := func(axis Axis) {
		axisID := axis.ID
		if axisID == "" {
			axisID = fmt.Sprintf("y%d", len(ordered))
			axis.ID = axisID
		}
		if _, exists := indexByID[axisID]; exists {
			return
		}
		indexByID[axisID] = len(ordered)
		ordered = append(ordered, axis)
	}

	for _, axis := range spec.Axes.Y {
		addAxis(axis)
	}
	for _, item := range spec.Series {
		axisID := item.Axis
		if axisID == "" {
			axisID = "y"
		}
		if _, exists := indexByID[axisID]; !exists {
			addAxis(Axis{ID: axisID, Type: AxisTypeLinear, Label: axisID})
		}
	}
	if len(ordered) == 0 {
		addAxis(Axis{ID: "y", Type: AxisTypeLinear, Label: "y"})
	}

	ret := make([]map[string]any, 0, len(ordered))
	for _, axis := range ordered {
		item := map[string]any{
			"type": echartsAxisType(axis.Type),
		}
		if axis.Type == "" {
			item["type"] = "value"
		}
		if axis.Label != "" {
			item["name"] = axis.Label
		} else if axis.ID != "" {
			item["name"] = axis.ID
		}
		if axis.Extent != nil {
			if axis.Extent.Min != nil {
				item["min"] = axis.Extent.Min
			}
			if axis.Extent.Max != nil {
				item["max"] = axis.Extent.Max
			}
		}
		ret = append(ret, item)
	}
	return ret, indexByID
}

func buildEChartsSeries(item Series, yAxisIndex map[string]int, domain Domain, timeOptions resolvedOutputTimeOptions) ([]map[string]any, error) {
	axisID := item.Axis
	if axisID == "" {
		axisID = "y"
	}
	idx, exists := yAxisIndex[axisID]
	if !exists {
		idx = 0
	}
	name := item.Name
	if name == "" {
		name = item.ID
	}

	switch item.Representation.Kind {
	case RepresentationRawPoint, RepresentationTimeBucketValue:
		lineStyle := mergeStyle(map[string]any{}, styleLineOptions(item.Style))
		data := item.Data
		if item.Representation.Kind == RepresentationRawPoint {
			data = selectPairs(item.Data, rawPointXIndex(item, domain), rawPointYIndex(item), domain, timeOptions)
		} else {
			data = selectPairs(item.Data, timeDomainXIndex(item, domain), rawPointYIndex(item), domain, timeOptions)
		}
		seriesItem := map[string]any{
			"type":       "line",
			"name":       name,
			"showSymbol": false,
			"yAxisIndex": idx,
			"data":       data,
		}
		if len(lineStyle) > 0 {
			seriesItem["lineStyle"] = lineStyle
		}
		return []map[string]any{seriesItem}, nil
	case RepresentationTimeBucketBand:
		return buildTimeBucketBandSeries(item, idx, name, domain, timeOptions)
	case RepresentationDistributionHistogram:
		return buildHistogramSeries(item, idx, name)
	case RepresentationDistributionBoxplot:
		return buildBoxplotSeries(item, idx, name)
	case RepresentationEventPoint:
		return buildEventPointSeries(item, idx, name, domain, timeOptions)
	case RepresentationEventRange:
		return buildEventRangeSeries(item, idx, name, domain, timeOptions)
	default:
		return nil, fmt.Errorf("advn: unsupported echarts representation %q", item.Representation.Kind)
	}
}

func buildTimeBucketBandSeries(item Series, yAxisIndex int, name string, domain Domain, timeOptions resolvedOutputTimeOptions) ([]map[string]any, error) {
	timeIndex := fieldIndex(item.Representation.Fields, "time")
	if timeIndex < 0 {
		timeIndex = 0
	}
	minIndex := fieldIndex(item.Representation.Fields, "min")
	maxIndex := fieldIndex(item.Representation.Fields, "max")
	avgIndex := fieldIndex(item.Representation.Fields, "avg")

	if minIndex < 0 && maxIndex < 0 && avgIndex < 0 {
		return nil, fmt.Errorf("advn: time-bucket-band requires at least one of min, avg, max fields")
	}

	seriesList := make([]map[string]any, 0, 3)
	stackName := "band:" + item.ID
	if stackName == "band:" {
		stackName = "band:" + name
	}

	baseColor := styleString(item.Style, "color", "")
	bandColor := styleString(item.Style, "bandColor", baseColor)
	lineColor := styleString(item.Style, "lineColor", baseColor)
	bandOpacity := styleFloat(item.Style, "opacity", 0.18)
	lineWidth := styleFloat(item.Style, "lineWidth", 0)

	if minIndex >= 0 && maxIndex >= 0 {
		seriesList = append(seriesList, map[string]any{
			"type":       "line",
			"name":       "",
			"showSymbol": false,
			"yAxisIndex": yAxisIndex,
			"stack":      stackName,
			"silent":     true,
			"lineStyle":  map[string]any{"opacity": 0, "width": 0},
			"areaStyle":  map[string]any{"opacity": 0},
			"data":       selectPairs(item.Data, timeIndex, minIndex, domain, timeOptions),
		})
		areaStyle := map[string]any{"opacity": bandOpacity}
		if bandColor != "" {
			areaStyle["color"] = bandColor
		}
		seriesList = append(seriesList, map[string]any{
			"type":       "line",
			"name":       "",
			"showSymbol": false,
			"yAxisIndex": yAxisIndex,
			"stack":      stackName,
			"silent":     true,
			"lineStyle":  map[string]any{"opacity": 0, "width": 0},
			"areaStyle":  areaStyle,
			"data":       selectBandPairs(item.Data, timeIndex, minIndex, maxIndex, domain, timeOptions),
		})
	} else {
		if minIndex >= 0 {
			lineStyle := map[string]any{"opacity": 0.35}
			if lineColor != "" {
				lineStyle["color"] = lineColor
			}
			if lineWidth > 0 {
				lineStyle["width"] = lineWidth
			}
			seriesList = append(seriesList, map[string]any{
				"type":       "line",
				"name":       name + " min",
				"showSymbol": false,
				"yAxisIndex": yAxisIndex,
				"lineStyle":  lineStyle,
				"data":       selectPairs(item.Data, timeIndex, minIndex, domain, timeOptions),
			})
		}
		if maxIndex >= 0 {
			lineStyle := map[string]any{"opacity": 0.35}
			if lineColor != "" {
				lineStyle["color"] = lineColor
			}
			if lineWidth > 0 {
				lineStyle["width"] = lineWidth
			}
			seriesList = append(seriesList, map[string]any{
				"type":       "line",
				"name":       name + " max",
				"showSymbol": false,
				"yAxisIndex": yAxisIndex,
				"lineStyle":  lineStyle,
				"data":       selectPairs(item.Data, timeIndex, maxIndex, domain, timeOptions),
			})
		}
	}

	if avgIndex >= 0 {
		lineStyle := map[string]any{}
		if lineColor != "" {
			lineStyle["color"] = lineColor
		}
		if lineWidth > 0 {
			lineStyle["width"] = lineWidth
		}
		avgSeries := map[string]any{
			"type":       "line",
			"name":       name,
			"showSymbol": false,
			"yAxisIndex": yAxisIndex,
			"data":       selectPairs(item.Data, timeIndex, avgIndex, domain, timeOptions),
		}
		if len(lineStyle) > 0 {
			avgSeries["lineStyle"] = lineStyle
		}
		seriesList = append(seriesList, avgSeries)
	} else if minIndex < 0 && maxIndex >= 0 {
		lineStyle := map[string]any{"opacity": 0.65}
		if lineColor != "" {
			lineStyle["color"] = lineColor
		}
		if lineWidth > 0 {
			lineStyle["width"] = lineWidth
		}
		seriesList = append(seriesList, map[string]any{
			"type":       "line",
			"name":       name,
			"showSymbol": false,
			"yAxisIndex": yAxisIndex,
			"lineStyle":  lineStyle,
			"data":       selectPairs(item.Data, timeIndex, maxIndex, domain, timeOptions),
		})
	} else if maxIndex < 0 && minIndex >= 0 {
		lineStyle := map[string]any{"opacity": 0.65}
		if lineColor != "" {
			lineStyle["color"] = lineColor
		}
		if lineWidth > 0 {
			lineStyle["width"] = lineWidth
		}
		seriesList = append(seriesList, map[string]any{
			"type":       "line",
			"name":       name,
			"showSymbol": false,
			"yAxisIndex": yAxisIndex,
			"lineStyle":  lineStyle,
			"data":       selectPairs(item.Data, timeIndex, minIndex, domain, timeOptions),
		})
	}
	return seriesList, nil
}

func buildHistogramSeries(item Series, yAxisIndex int, name string) ([]map[string]any, error) {
	startIndex := fieldIndex(item.Representation.Fields, "binStart")
	endIndex := fieldIndex(item.Representation.Fields, "binEnd")
	countIndex := fieldIndex(item.Representation.Fields, "count")
	if startIndex < 0 {
		startIndex = 0
	}
	if endIndex < 0 {
		endIndex = 1
	}
	if countIndex < 0 {
		countIndex = 2
	}
	labels := make([]any, 0, len(item.Data))
	counts := make([]any, 0, len(item.Data))
	for _, row := range item.Data {
		values, ok := row.([]any)
		if !ok || startIndex >= len(values) || endIndex >= len(values) || countIndex >= len(values) {
			continue
		}
		labels = append(labels, fmt.Sprintf("%v-%v", values[startIndex], values[endIndex]))
		counts = append(counts, values[countIndex])
	}
	if len(counts) == 0 {
		return nil, fmt.Errorf("advn: histogram requires bin data")
	}
	itemStyle := mergeStyle(map[string]any{}, styleItemOptions(item.Style))
	seriesItem := map[string]any{
		"type":       "bar",
		"name":       name,
		"yAxisIndex": yAxisIndex,
		"data":       counts,
		"_advnXAxis": map[string]any{"type": "category", "data": labels},
	}
	if len(itemStyle) > 0 {
		seriesItem["itemStyle"] = itemStyle
	}
	return []map[string]any{seriesItem}, nil
}

func buildBoxplotSeries(item Series, yAxisIndex int, name string) ([]map[string]any, error) {
	categoryIndex := fieldIndex(item.Representation.Fields, "category")
	lowIndex := fieldIndex(item.Representation.Fields, "low")
	q1Index := fieldIndex(item.Representation.Fields, "q1")
	medianIndex := fieldIndex(item.Representation.Fields, "median")
	q3Index := fieldIndex(item.Representation.Fields, "q3")
	highIndex := fieldIndex(item.Representation.Fields, "high")
	if categoryIndex < 0 {
		categoryIndex = 0
	}
	if lowIndex < 0 || q1Index < 0 || medianIndex < 0 || q3Index < 0 || highIndex < 0 {
		return nil, fmt.Errorf("advn: boxplot requires category, low, q1, median, q3, high fields")
	}
	categories := make([]any, 0, len(item.Data))
	boxData := make([]any, 0, len(item.Data))
	for _, row := range item.Data {
		values, ok := row.([]any)
		if !ok || highIndex >= len(values) {
			continue
		}
		categories = append(categories, values[categoryIndex])
		boxData = append(boxData, []any{values[lowIndex], values[q1Index], values[medianIndex], values[q3Index], values[highIndex]})
	}
	if len(boxData) == 0 {
		return nil, fmt.Errorf("advn: boxplot requires box data")
	}
	seriesList := []map[string]any{{
		"type":       "boxplot",
		"name":       name,
		"yAxisIndex": yAxisIndex,
		"data":       boxData,
		"_advnXAxis": map[string]any{"type": "category", "data": categories},
	}}
	itemStyle := mergeStyle(map[string]any{}, styleItemOptions(item.Style))
	if len(itemStyle) > 0 {
		seriesList[0]["itemStyle"] = itemStyle
	}
	if outliers, ok := item.Extra["outliers"].([]any); ok && len(outliers) > 0 {
		seriesList = append(seriesList, map[string]any{
			"type":       "scatter",
			"name":       name + " outliers",
			"yAxisIndex": yAxisIndex,
			"data":       outliers,
		})
	}
	return seriesList, nil
}

func buildEventPointSeries(item Series, yAxisIndex int, name string, domain Domain, timeOptions resolvedOutputTimeOptions) ([]map[string]any, error) {
	timeIndex := fieldIndex(item.Representation.Fields, "time")
	valueIndex := fieldIndex(item.Representation.Fields, "value")
	labelIndex := fieldIndex(item.Representation.Fields, "label")
	if timeIndex < 0 {
		timeIndex = 0
	}
	if valueIndex < 0 {
		valueIndex = 1
	}
	points := make([]any, 0, len(item.Data))
	for _, row := range item.Data {
		values, ok := row.([]any)
		if !ok || timeIndex >= len(values) || valueIndex >= len(values) {
			continue
		}
		point := map[string]any{
			"value": []any{normalizeTimeValueForEChartsWithOptions(values[timeIndex], domain, timeOptions), values[valueIndex]},
		}
		if labelIndex >= 0 && labelIndex < len(values) {
			if label, ok := values[labelIndex].(string); ok && label != "" {
				point["name"] = label
			}
		}
		points = append(points, point)
	}
	if len(points) == 0 {
		return nil, fmt.Errorf("advn: event-point requires point data")
	}
	seriesItem := map[string]any{
		"type":       "scatter",
		"name":       name,
		"yAxisIndex": yAxisIndex,
		"data":       points,
	}
	if len(item.Style) > 0 {
		seriesItem["itemStyle"] = mergeStyle(map[string]any{}, styleItemOptions(item.Style))
	}
	return []map[string]any{seriesItem}, nil
}

func buildEventRangeSeries(item Series, yAxisIndex int, name string, domain Domain, timeOptions resolvedOutputTimeOptions) ([]map[string]any, error) {
	fromIndex := fieldIndex(item.Representation.Fields, "from")
	toIndex := fieldIndex(item.Representation.Fields, "to")
	labelIndex := fieldIndex(item.Representation.Fields, "label")
	if fromIndex < 0 {
		fromIndex = 0
	}
	if toIndex < 0 {
		toIndex = 1
	}
	areas := make([]any, 0, len(item.Data))
	for _, row := range item.Data {
		values, ok := row.([]any)
		if !ok || fromIndex >= len(values) || toIndex >= len(values) {
			continue
		}
		from := map[string]any{"xAxis": normalizeTimeValueForEChartsWithOptions(values[fromIndex], domain, timeOptions)}
		to := map[string]any{"xAxis": normalizeTimeValueForEChartsWithOptions(values[toIndex], domain, timeOptions)}
		if labelIndex >= 0 && labelIndex < len(values) {
			if label, ok := values[labelIndex].(string); ok && label != "" {
				from["name"] = label
			}
		}
		if len(item.Style) > 0 {
			from["itemStyle"] = mergeStyle(map[string]any{}, styleItemOptions(item.Style))
		}
		areas = append(areas, []map[string]any{from, to})
	}
	if len(areas) == 0 {
		return nil, fmt.Errorf("advn: event-range requires range data")
	}
	seriesItem := map[string]any{
		"type":       "line",
		"name":       name,
		"yAxisIndex": yAxisIndex,
		"silent":     true,
		"showSymbol": false,
		"lineStyle":  map[string]any{"opacity": 0, "width": 0},
		"data":       []any{},
		"markArea":   map[string]any{"data": areas},
	}
	return []map[string]any{seriesItem}, nil
}

func preferredAnnotationTarget(seriesList []map[string]any) map[string]any {
	for _, item := range seriesList {
		if name, ok := item["name"].(string); ok && name != "" {
			return item
		}
	}
	return seriesList[0]
}

func detectDefaultXAxisType(spec *Spec) string {
	for _, item := range spec.Series {
		switch item.Representation.Kind {
		case RepresentationDistributionHistogram, RepresentationDistributionBoxplot:
			return "category"
		}
	}
	if spec.Domain.Kind == DomainKindTime {
		return "time"
	}
	return "value"
}

func applyEChartsAnnotations(spec *Spec, target map[string]any, timeOptions resolvedOutputTimeOptions) {
	markLine := []map[string]any{}
	markArea := []any{}
	markPoint := []map[string]any{}

	for _, annotation := range spec.Annotations {
		switch annotation.Kind {
		case AnnotationKindLine:
			item := map[string]any{}
			if isXAxis(spec, annotation.Axis) {
				item["xAxis"] = normalizeTimeValueForEChartsWithOptions(annotation.Value, spec.Domain, timeOptions)
			} else {
				item["yAxis"] = annotation.Value
			}
			if annotation.Label != "" {
				item["name"] = annotation.Label
				item["label"] = map[string]any{"formatter": annotation.Label}
			}
			markLine = append(markLine, item)
		case AnnotationKindRange:
			if isXAxis(spec, annotation.Axis) {
				from := map[string]any{"xAxis": normalizeTimeValueForEChartsWithOptions(annotation.From, spec.Domain, timeOptions)}
				to := map[string]any{"xAxis": normalizeTimeValueForEChartsWithOptions(annotation.To, spec.Domain, timeOptions)}
				if annotation.Label != "" {
					from["name"] = annotation.Label
				}
				if len(annotation.Style) > 0 {
					from["itemStyle"] = annotation.Style
				}
				markArea = append(markArea, []map[string]any{from, to})
			}
		case AnnotationKindPoint:
			item := map[string]any{}
			if annotation.At != nil && annotation.Value != nil {
				if isXAxis(spec, annotation.Axis) {
					item["coord"] = []any{normalizeTimeValueForEChartsWithOptions(annotation.At, spec.Domain, timeOptions), annotation.Value}
				} else {
					item["coord"] = []any{annotation.At, annotation.Value}
				}
			}
			if annotation.Label != "" {
				item["name"] = annotation.Label
			}
			if len(annotation.Style) > 0 {
				item["itemStyle"] = annotation.Style
			}
			if len(item) > 0 {
				markPoint = append(markPoint, item)
			}
		}
	}

	if len(markLine) > 0 {
		target["markLine"] = map[string]any{"symbol": []string{"none", "none"}, "data": markLine}
	}
	if len(markArea) > 0 {
		target["markArea"] = map[string]any{"data": markArea}
	}
	if len(markPoint) > 0 {
		target["markPoint"] = map[string]any{"data": markPoint}
	}
}

func isXAxis(spec *Spec, axis string) bool {
	return axis == "" || axis == "x" || axis == spec.Axes.X.ID
}

func echartsAxisType(axisType string) string {
	switch axisType {
	case AxisTypeTime:
		return "time"
	case AxisTypeCategory:
		return "category"
	case AxisTypeLog:
		return "log"
	default:
		return "value"
	}
}

func fieldIndex(fields []string, name string) int {
	for index, field := range fields {
		if field == name {
			return index
		}
	}
	return -1
}

func selectPairs(rows []any, xIndex, yIndex int, domain Domain, timeOptions resolvedOutputTimeOptions) []any {
	ret := make([]any, 0, len(rows))
	for _, row := range rows {
		values, ok := row.([]any)
		if !ok {
			continue
		}
		if xIndex < 0 || yIndex < 0 || xIndex >= len(values) || yIndex >= len(values) {
			continue
		}
		xValue := values[xIndex]
		if domain.Kind == DomainKindTime {
			xValue = normalizeTimeValueForEChartsWithOptions(values[xIndex], domain, timeOptions)
		}
		ret = append(ret, []any{xValue, values[yIndex]})
	}
	return ret
}

func selectBandPairs(rows []any, xIndex, minIndex, maxIndex int, domain Domain, timeOptions resolvedOutputTimeOptions) []any {
	ret := make([]any, 0, len(rows))
	for _, row := range rows {
		values, ok := row.([]any)
		if !ok {
			continue
		}
		if xIndex < 0 || minIndex < 0 || maxIndex < 0 || xIndex >= len(values) || minIndex >= len(values) || maxIndex >= len(values) {
			continue
		}
		minValue, minOK := toFloat64(values[minIndex])
		maxValue, maxOK := toFloat64(values[maxIndex])
		if !minOK || !maxOK {
			continue
		}
		xValue := values[xIndex]
		if domain.Kind == DomainKindTime {
			xValue = normalizeTimeValueForEChartsWithOptions(values[xIndex], domain, timeOptions)
		}
		ret = append(ret, []any{xValue, maxValue - minValue})
	}
	return ret
}

func timeDomainXIndex(item Series, domain Domain) int {
	if domain.Kind != DomainKindTime {
		return 0
	}
	if index := fieldIndex(item.Representation.Fields, "time"); index >= 0 {
		return index
	}
	return 0
}

func rawPointXIndex(item Series, domain Domain) int {
	if index := fieldIndex(item.Representation.Fields, "x"); index >= 0 {
		return index
	}
	if index := fieldIndex(item.Representation.Fields, "time"); index >= 0 {
		return index
	}
	if domain.Kind == DomainKindTime {
		return 0
	}
	return 0
}

func rawPointYIndex(item Series) int {
	if index := fieldIndex(item.Representation.Fields, "y"); index >= 0 {
		return index
	}
	if index := fieldIndex(item.Representation.Fields, "value"); index >= 0 {
		return index
	}
	return 1
}

func styleString(style map[string]any, key string, fallback string) string {
	if style == nil {
		return fallback
	}
	if value, ok := style[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func styleFloat(style map[string]any, key string, fallback float64) float64 {
	if style == nil {
		return fallback
	}
	if value, ok := toFloat64(style[key]); ok {
		return value
	}
	return fallback
}

func styleLineOptions(style map[string]any) map[string]any {
	ret := map[string]any{}
	if color := styleString(style, "lineColor", styleString(style, "color", "")); color != "" {
		ret["color"] = color
	}
	if width := styleFloat(style, "lineWidth", 0); width > 0 {
		ret["width"] = width
	}
	if opacity := styleFloat(style, "lineOpacity", -1); opacity >= 0 {
		ret["opacity"] = opacity
	}
	return ret
}

func styleItemOptions(style map[string]any) map[string]any {
	ret := map[string]any{}
	if color := styleString(style, "color", ""); color != "" {
		ret["color"] = color
	}
	if opacity := styleFloat(style, "opacity", -1); opacity >= 0 {
		ret["opacity"] = opacity
	}
	return ret
}

func mergeStyle(base map[string]any, override map[string]any) map[string]any {
	for key, value := range override {
		base[key] = value
	}
	return base
}

func toFloat64(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	case json.Number:
		ret, err := number.Float64()
		if err != nil {
			return 0, false
		}
		return ret, true
	case string:
		ret, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return 0, false
		}
		return ret, true
	default:
		return 0, false
	}
}
