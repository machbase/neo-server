package advn

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func (spec *Spec) Validate() error {
	if spec == nil {
		return fmt.Errorf("advn: spec is nil")
	}
	if spec.Version != Version1 {
		return fmt.Errorf("advn: unsupported version %d", spec.Version)
	}
	if err := spec.Domain.Validate(); err != nil {
		return err
	}
	if err := spec.Axes.Validate(); err != nil {
		return err
	}
	if err := spec.validateAxisReferences(); err != nil {
		return err
	}
	for index, series := range spec.Series {
		if err := series.Validate(); err != nil {
			return fmt.Errorf("advn: series[%d]: %w", index, err)
		}
	}
	for index, annotation := range spec.Annotations {
		if err := annotation.Validate(); err != nil {
			return fmt.Errorf("advn: annotations[%d]: %w", index, err)
		}
	}
	return nil
}

func (spec *Spec) validateAxisReferences() error {
	yAxes := map[string]struct{}{}
	for _, axis := range spec.Axes.Y {
		if axis.ID != "" {
			yAxes[axis.ID] = struct{}{}
		}
	}
	for index, series := range spec.Series {
		if series.Axis != "" {
			if _, ok := yAxes[series.Axis]; !ok && len(yAxes) > 0 {
				return fmt.Errorf("advn: series[%d]: axis %q is not defined", index, series.Axis)
			}
		}
	}
	for index, annotation := range spec.Annotations {
		if annotation.Axis == "" || annotation.Axis == "x" || annotation.Axis == spec.Axes.X.ID {
			continue
		}
		if _, ok := yAxes[annotation.Axis]; !ok {
			return fmt.Errorf("advn: annotations[%d]: axis %q is not defined", index, annotation.Axis)
		}
	}
	return nil
}

func (domain Domain) Validate() error {
	if domain.Kind == "" {
		if domain.TimeFormat != "" || domain.TimeUnit != "" {
			return fmt.Errorf("timeformat requires domain kind %q", DomainKindTime)
		}
		return nil
	}
	if !contains(domain.Kind, DomainKindTime, DomainKindNumeric, DomainKindCategory) {
		return fmt.Errorf("invalid domain kind %q", domain.Kind)
	}
	if domain.Kind != DomainKindTime {
		if domain.TimeFormat != "" || domain.TimeUnit != "" {
			return fmt.Errorf("timeformat requires domain kind %q", DomainKindTime)
		}
		return nil
	}
	if domain.TimeFormat != "" && !contains(domain.TimeFormat, TimeFormatRFC3339, TimeFormatEpoch, TimeFormatSecond, TimeFormatMilli, TimeFormatMicro, TimeFormatNano) {
		return fmt.Errorf("invalid timeformat %q", domain.TimeFormat)
	}
	if domain.TimeUnit != "" && !contains(domain.TimeUnit, TimeUnitSecond, TimeUnitMillisecond, TimeUnitMicrosecond, TimeUnitNanosecond) {
		return fmt.Errorf("invalid timeUnit %q", domain.TimeUnit)
	}
	if domain.TimeFormat == TimeFormatRFC3339 && domain.TimeUnit != "" {
		return fmt.Errorf("timeUnit is only valid for legacy epoch timeformat")
	}
	return nil
}

func (axes Axes) Validate() error {
	if err := axes.X.Validate(false); err != nil {
		return fmt.Errorf("x axis: %w", err)
	}
	for index, axis := range axes.Y {
		if err := axis.Validate(true); err != nil {
			return fmt.Errorf("y axis[%d]: %w", index, err)
		}
	}
	return nil
}

func (axis Axis) Validate(requireID bool) error {
	if requireID && axis.ID == "" {
		return fmt.Errorf("axis id is required")
	}
	if axis.Type == "" {
		return nil
	}
	if !contains(axis.Type, AxisTypeTime, AxisTypeLinear, AxisTypeLog, AxisTypeCategory) {
		return fmt.Errorf("invalid axis type %q", axis.Type)
	}
	return nil
}

func (series Series) Validate() error {
	if series.ID == "" {
		return fmt.Errorf("id is required")
	}
	if err := series.Representation.Validate(); err != nil {
		return err
	}
	if err := series.Source.Validate(); err != nil {
		return err
	}
	if err := series.Quality.Validate(); err != nil {
		return err
	}
	if err := validateStyle(series.Style, false); err != nil {
		return fmt.Errorf("style: %w", err)
	}
	return nil
}

func (representation Representation) Validate() error {
	if representation.Kind == "" {
		return fmt.Errorf("representation.kind is required")
	}
	if !contains(
		representation.Kind,
		RepresentationRawPoint,
		RepresentationTimeBucketValue,
		RepresentationTimeBucketBand,
		RepresentationDistributionHistogram,
		RepresentationDistributionBoxplot,
		RepresentationEventPoint,
		RepresentationEventRange,
	) {
		return fmt.Errorf("invalid representation.kind %q", representation.Kind)
	}
	if err := validateRepresentationFields(representation); err != nil {
		return err
	}
	return nil
}

func validateRepresentationFields(representation Representation) error {
	switch representation.Kind {
	case RepresentationRawPoint:
		if len(representation.Fields) > 0 && len(representation.Fields) < 2 {
			return fmt.Errorf("raw-point requires at least 2 fields when fields is provided")
		}
	case RepresentationTimeBucketValue:
		if err := requireFields(representation.Fields, "time", "value"); err != nil {
			return fmt.Errorf("time-bucket-value %w", err)
		}
	case RepresentationTimeBucketBand:
		if err := requireFields(representation.Fields, "time"); err != nil {
			return fmt.Errorf("time-bucket-band %w", err)
		}
		if !hasAnyField(representation.Fields, "min", "max", "avg") {
			return fmt.Errorf("time-bucket-band requires at least one of min, max, avg fields")
		}
	case RepresentationDistributionHistogram:
		if err := requireFields(representation.Fields, "binStart", "binEnd", "count"); err != nil {
			return fmt.Errorf("distribution-histogram %w", err)
		}
	case RepresentationDistributionBoxplot:
		if err := requireFields(representation.Fields, "category", "low", "q1", "median", "q3", "high"); err != nil {
			return fmt.Errorf("distribution-boxplot %w", err)
		}
		if len(representation.OutlierFields) > 0 {
			if err := requireFields(representation.OutlierFields, "category", "value"); err != nil {
				return fmt.Errorf("distribution-boxplot outlierFields %w", err)
			}
		}
	case RepresentationEventPoint:
		if err := requireFields(representation.Fields, "time", "value"); err != nil {
			return fmt.Errorf("event-point %w", err)
		}
	case RepresentationEventRange:
		if err := requireFields(representation.Fields, "from", "to"); err != nil {
			return fmt.Errorf("event-range %w", err)
		}
	}
	return nil
}

func requireFields(fields []string, required ...string) error {
	if len(fields) == 0 {
		return fmt.Errorf("requires fields %q", strings.Join(required, ", "))
	}
	for _, field := range required {
		if !containsField(fields, field) {
			return fmt.Errorf("requires field %q", field)
		}
	}
	return nil
}

func hasAnyField(fields []string, candidates ...string) bool {
	for _, candidate := range candidates {
		if containsField(fields, candidate) {
			return true
		}
	}
	return false
}

func containsField(fields []string, target string) bool {
	for _, field := range fields {
		if field == target {
			return true
		}
	}
	return false
}

func (source Source) Validate() error {
	if source.Kind == "" {
		return nil
	}
	if !contains(source.Kind, SourceKindRaw, SourceKindRollup, SourceKindSampled, SourceKindDerived) {
		return fmt.Errorf("invalid source.kind %q", source.Kind)
	}
	return nil
}

func (quality Quality) Validate() error {
	if quality.Coverage < 0 || quality.Coverage > 1 {
		return fmt.Errorf("quality.coverage must be between 0 and 1")
	}
	if quality.RowCount < 0 {
		return fmt.Errorf("quality.rowCount must be 0 or larger")
	}
	if quality.EstimatedPoints < 0 {
		return fmt.Errorf("quality.estimatedPoints must be 0 or larger")
	}
	return nil
}

func (annotation Annotation) Validate() error {
	if annotation.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if !contains(annotation.Kind, AnnotationKindPoint, AnnotationKindLine, AnnotationKindRange) {
		return fmt.Errorf("invalid annotation kind %q", annotation.Kind)
	}
	if err := validateStyle(annotation.Style, true); err != nil {
		return fmt.Errorf("style: %w", err)
	}
	return nil
}

func validateStyle(style map[string]any, allowPreferredRenderer bool) error {
	if style == nil {
		return nil
	}
	for key, value := range style {
		switch key {
		case "color", "lineColor", "bandColor":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%s must be a string", key)
			}
		case "preferredRenderer":
			if !allowPreferredRenderer {
				if _, ok := value.(string); !ok {
					return fmt.Errorf("%s must be a string", key)
				}
				continue
			}
			return fmt.Errorf("unsupported style key %q", key)
		case "opacity", "lineWidth":
			if _, ok := styleNumber(value); !ok {
				return fmt.Errorf("%s must be a number", key)
			}
		default:
			return fmt.Errorf("unsupported style key %q", key)
		}
	}
	if opacity, ok := styleNumber(style["opacity"]); ok {
		if opacity < 0 || opacity > 1 {
			return fmt.Errorf("opacity must be between 0 and 1")
		}
	}
	if lineWidth, ok := styleNumber(style["lineWidth"]); ok {
		if lineWidth < 0 {
			return fmt.Errorf("lineWidth must be 0 or larger")
		}
	}
	for _, key := range []string{"color", "lineColor", "bandColor"} {
		if value, ok := style[key].(string); ok {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s must not be empty", key)
			}
		}
	}
	if allowPreferredRenderer {
		if value, exists := style["preferredRenderer"]; exists {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("preferredRenderer must be a string")
			}
		}
	}
	return nil
}

func styleNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		ret, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		return ret, true
	case string:
		ret, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, false
		}
		return ret, true
	default:
		return 0, false
	}
}

func contains(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
