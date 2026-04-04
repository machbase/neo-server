package advn

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

func canonicalTimeformat(timeformat string, _ string) string {
	return timeformat
}

const timeValueKindEpoch = "epoch"

func isEpochUnit(value string) bool {
	return contains(value, TimeformatSecond, TimeformatMilli, TimeformatMicro, TimeformatNano)
}

func effectiveTimeformat(domain Domain, value any) string {
	switch canonicalTimeformat(domain.Timeformat, "") {
	case TimeformatRFC3339:
		return TimeformatRFC3339
	case TimeformatSecond, TimeformatMilli, TimeformatMicro, TimeformatNano:
		return timeValueKindEpoch
	}
	switch typed := value.(type) {
	case time.Time:
		return TimeformatRFC3339
	case *time.Time:
		if typed != nil {
			return TimeformatRFC3339
		}
	case string:
		if _, err := time.Parse(time.RFC3339Nano, typed); err == nil {
			return TimeformatRFC3339
		}
		if _, ok := numericText(typed); ok {
			return timeValueKindEpoch
		}
	case json.Number:
		return timeValueKindEpoch
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return timeValueKindEpoch
	}
	return ""
}

func effectiveTimeUnit(domain Domain, value any) string {
	switch canonicalTimeformat(domain.Timeformat, "") {
	case TimeformatSecond, TimeformatMilli, TimeformatMicro, TimeformatNano:
		return canonicalTimeformat(domain.Timeformat, "")
	}
	if effectiveTimeformat(domain, value) != timeValueKindEpoch {
		return ""
	}
	return inferEpochTimeUnit(value)
}

func inferEpochTimeUnit(value any) string {
	text, ok := numericText(value)
	if !ok {
		return TimeformatNano
	}
	whole := text
	if idx := strings.IndexByte(whole, '.'); idx >= 0 {
		whole = whole[:idx]
	}
	whole = strings.TrimLeft(whole, "-")
	if len(whole) >= 18 {
		return TimeformatNano
	}
	if len(whole) >= 15 {
		return TimeformatMicro
	}
	if len(whole) >= 12 {
		return TimeformatMilli
	}
	return TimeformatSecond
}

func parseTimeValueWithDomain(value any, domain Domain) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), true
	case *time.Time:
		if typed != nil {
			return typed.UTC(), true
		}
	}
	switch effectiveTimeformat(domain, value) {
	case TimeformatRFC3339:
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return time.Time{}, false
		}
		ret, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return time.Time{}, false
		}
		return ret.UTC(), true
	case timeValueKindEpoch:
		unixNano, ok := timeValueToUnixNano(value, effectiveTimeUnit(domain, value))
		if !ok {
			return time.Time{}, false
		}
		return time.Unix(0, unixNano).UTC(), true
	default:
		if text, ok := value.(string); ok {
			if ret, err := time.Parse(time.RFC3339Nano, text); err == nil {
				return ret.UTC(), true
			}
		}
		unixNano, ok := timeValueToUnixNano(value, inferEpochTimeUnit(value))
		if !ok {
			return time.Time{}, false
		}
		return time.Unix(0, unixNano).UTC(), true
	}
}

func timeValueToUnixNano(value any, unit string) (int64, bool) {
	text, ok := numericText(value)
	if !ok {
		return 0, false
	}
	if !strings.Contains(text, ".") {
		base, err := strconv.ParseInt(text, 10, 64)
		if err == nil {
			return scaleUnixNano(base, unit), true
		}
	}
	base, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(base) || math.IsInf(base, 0) {
		return 0, false
	}
	return int64(math.Round(base * timeUnitMultiplier(unit))), true
}

func normalizeTimeValueForJS(value any, domain Domain) any {
	if effectiveTimeformat(domain, value) != timeValueKindEpoch {
		return value
	}
	if text, ok := numericText(value); ok {
		return text
	}
	return value
}

func normalizeTimeValueForECharts(value any, domain Domain) any {
	if ret, ok := parseTimeValueWithDomain(value, domain); ok {
		return ret.Format(time.RFC3339Nano)
	}
	return value
}

func formatTimeValue(value any, domain Domain) string {
	if ret, ok := parseTimeValueWithDomain(value, domain); ok {
		return ret.Format(time.RFC3339Nano)
	}
	return formatAny(value)
}

func numericText(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", false
		}
		if _, err := strconv.ParseFloat(trimmed, 64); err != nil {
			return "", false
		}
		return trimmed, true
	case json.Number:
		return typed.String(), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64), true
	case int:
		return strconv.FormatInt(int64(typed), 10), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	default:
		return "", false
	}
}

func scaleUnixNano(base int64, unit string) int64 {
	switch unit {
	case TimeformatMilli:
		return base * int64(time.Millisecond)
	case TimeformatMicro:
		return base * int64(time.Microsecond)
	case TimeformatNano:
		return base
	default:
		return base * int64(time.Second)
	}
}

func timeUnitMultiplier(unit string) float64 {
	switch unit {
	case TimeformatMilli:
		return float64(time.Millisecond)
	case TimeformatMicro:
		return float64(time.Microsecond)
	case TimeformatNano:
		return 1
	default:
		return float64(time.Second)
	}
}

func NormalizeSpecTimeValues(spec *Spec) {
	if spec == nil || spec.Domain.Kind != DomainKindTime {
		return
	}
	spec.Domain.From = normalizeTimeValueForJS(spec.Domain.From, spec.Domain)
	spec.Domain.To = normalizeTimeValueForJS(spec.Domain.To, spec.Domain)
	if spec.Axes.X.Extent != nil {
		spec.Axes.X.Extent.Min = normalizeTimeValueForJS(spec.Axes.X.Extent.Min, spec.Domain)
		spec.Axes.X.Extent.Max = normalizeTimeValueForJS(spec.Axes.X.Extent.Max, spec.Domain)
	}
	for seriesIndex := range spec.Series {
		normalizeSeriesTimeValues(&spec.Series[seriesIndex], spec.Domain)
	}
	for annotationIndex := range spec.Annotations {
		normalizeAnnotationTimeValues(spec, &spec.Annotations[annotationIndex])
	}
}

func normalizeSeriesTimeValues(series *Series, domain Domain) {
	if series == nil {
		return
	}
	indexes := timeFieldIndexes(*series, domain)
	if len(indexes) == 0 {
		return
	}
	for rowIndex := range series.Data {
		values, ok := series.Data[rowIndex].([]any)
		if !ok {
			continue
		}
		for _, index := range indexes {
			if index >= 0 && index < len(values) {
				values[index] = normalizeTimeValueForJS(values[index], domain)
			}
		}
	}
}

func normalizeAnnotationTimeValues(spec *Spec, annotation *Annotation) {
	if annotation == nil || !isXAxis(spec, annotation.Axis) {
		return
	}
	switch annotation.Kind {
	case AnnotationKindLine:
		annotation.Value = normalizeTimeValueForJS(annotation.Value, spec.Domain)
	case AnnotationKindRange:
		annotation.From = normalizeTimeValueForJS(annotation.From, spec.Domain)
		annotation.To = normalizeTimeValueForJS(annotation.To, spec.Domain)
	case AnnotationKindPoint:
		annotation.At = normalizeTimeValueForJS(annotation.At, spec.Domain)
	}
}

func timeFieldIndexes(series Series, domain Domain) []int {
	indexes := []int{}
	addIndex := func(index int) {
		if index < 0 {
			return
		}
		for _, existing := range indexes {
			if existing == index {
				return
			}
		}
		indexes = append(indexes, index)
	}
	fields := series.Representation.Fields
	switch series.Representation.Kind {
	case RepresentationRawPoint:
		if index := fieldIndex(fields, "time"); index >= 0 {
			addIndex(index)
		} else if domain.Kind == DomainKindTime {
			if index := fieldIndex(fields, "x"); index >= 0 {
				addIndex(index)
			} else if len(fields) == 0 {
				addIndex(0)
			}
		}
	case RepresentationTimeBucketValue, RepresentationTimeBucketBand, RepresentationEventPoint:
		if index := fieldIndex(fields, "time"); index >= 0 {
			addIndex(index)
		} else {
			addIndex(0)
		}
	case RepresentationEventRange:
		if index := fieldIndex(fields, "from"); index >= 0 {
			addIndex(index)
		} else {
			addIndex(0)
		}
		if index := fieldIndex(fields, "to"); index >= 0 {
			addIndex(index)
		} else {
			addIndex(1)
		}
	}
	return indexes
}
