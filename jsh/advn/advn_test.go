package advn

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeDefaults(t *testing.T) {
	var nilSpec *Spec
	normalizedNil := nilSpec.Normalize()
	if normalizedNil == nil {
		t.Fatal("Normalize() returned nil")
	}
	if normalizedNil.Version != Version1 {
		t.Fatalf("expected version %d, got %d", Version1, normalizedNil.Version)
	}
	if normalizedNil.Series == nil {
		t.Fatal("expected non-nil series slice")
	}
	if normalizedNil.Annotations == nil {
		t.Fatal("expected non-nil annotations slice")
	}
	if normalizedNil.Axes.Y == nil {
		t.Fatal("expected non-nil y axes slice")
	}

	spec := &Spec{}
	normalized := spec.Normalize()
	if normalized != spec {
		t.Fatal("Normalize() should normalize in place")
	}
	if spec.Version != Version1 {
		t.Fatalf("expected version %d, got %d", Version1, spec.Version)
	}
	if spec.Series == nil || spec.Annotations == nil || spec.Axes.Y == nil {
		t.Fatal("expected slices to be initialized")
	}
}

func TestValidateSuccess(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime},
		Axes: Axes{
			X: Axis{Type: AxisTypeTime},
			Y: []Axis{{ID: "value", Type: AxisTypeLinear}},
		},
		Series: []Series{{
			ID:   "sensor-1",
			Axis: "value",
			Representation: Representation{
				Kind:   RepresentationTimeBucketBand,
				Fields: []string{"time", "min", "max", "avg", "count"},
			},
			Source: Source{Kind: SourceKindRollup},
			Quality: Quality{
				Coverage:         1,
				RowCount:         1440,
				EstimatedPoints:  86400,
				DownsamplePolicy: "rollup",
			},
		}},
		Annotations: []Annotation{{Kind: AnnotationKindLine}},
	}).Normalize()

	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() returned unexpected error: %v", err)
	}
}

func TestValidateFailures(t *testing.T) {
	tests := []struct {
		name string
		spec *Spec
		want string
	}{
		{
			name: "nil spec",
			spec: nil,
			want: "spec is nil",
		},
		{
			name: "unsupported version",
			spec: (&Spec{Version: 99}).Normalize(),
			want: "unsupported version",
		},
		{
			name: "invalid domain kind",
			spec: (&Spec{Version: Version1, Domain: Domain{Kind: "broken"}}).Normalize(),
			want: "invalid domain kind",
		},
		{
			name: "invalid time format",
			spec: (&Spec{Version: Version1, Domain: Domain{Kind: DomainKindTime, TimeFormat: "broken"}}).Normalize(),
			want: "invalid timeFormat",
		},
		{
			name: "invalid time unit",
			spec: (&Spec{Version: Version1, Domain: Domain{Kind: DomainKindTime, TimeFormat: TimeFormatEpoch, TimeUnit: "broken"}}).Normalize(),
			want: "invalid timeUnit",
		},
		{
			name: "time unit requires epoch",
			spec: (&Spec{Version: Version1, Domain: Domain{Kind: DomainKindTime, TimeFormat: TimeFormatRFC3339, TimeUnit: TimeUnitNanosecond}}).Normalize(),
			want: "timeUnit is only valid for epoch timeFormat",
		},
		{
			name: "missing y axis id",
			spec: (&Spec{Version: Version1, Axes: Axes{Y: []Axis{{Type: AxisTypeLinear}}}}).Normalize(),
			want: "axis id is required",
		},
		{
			name: "series missing id",
			spec: (&Spec{
				Version: Version1,
				Series:  []Series{{Representation: Representation{Kind: RepresentationRawPoint}}},
			}).Normalize(),
			want: "id is required",
		},
		{
			name: "representation missing kind",
			spec: (&Spec{Version: Version1, Series: []Series{{ID: "series-1"}}}).Normalize(),
			want: "representation.kind is required",
		},
		{
			name: "time-bucket-value missing field",
			spec: (&Spec{Version: Version1, Series: []Series{{
				ID:             "series-1",
				Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time"}},
			}}}).Normalize(),
			want: "requires field \"value\"",
		},
		{
			name: "time-bucket-band missing metric fields",
			spec: (&Spec{Version: Version1, Series: []Series{{
				ID:             "series-1",
				Representation: Representation{Kind: RepresentationTimeBucketBand, Fields: []string{"time", "count"}},
			}}}).Normalize(),
			want: "requires at least one of min, max, avg fields",
		},
		{
			name: "histogram missing required field",
			spec: (&Spec{Version: Version1, Series: []Series{{
				ID:             "series-1",
				Representation: Representation{Kind: RepresentationDistributionHistogram, Fields: []string{"binStart", "count"}},
			}}}).Normalize(),
			want: "requires field \"binEnd\"",
		},
		{
			name: "boxplot invalid outlier fields",
			spec: (&Spec{Version: Version1, Series: []Series{{
				ID: "series-1",
				Representation: Representation{
					Kind:          RepresentationDistributionBoxplot,
					Fields:        []string{"category", "low", "q1", "median", "q3", "high"},
					OutlierFields: []string{"category"},
				},
			}}}).Normalize(),
			want: "outlierFields requires field \"value\"",
		},
		{
			name: "event-range missing field",
			spec: (&Spec{Version: Version1, Series: []Series{{
				ID:             "series-1",
				Representation: Representation{Kind: RepresentationEventRange, Fields: []string{"from"}},
			}}}).Normalize(),
			want: "requires field \"to\"",
		},
		{
			name: "invalid source kind",
			spec: (&Spec{
				Version: Version1,
				Series: []Series{{
					ID:             "series-1",
					Representation: Representation{Kind: RepresentationRawPoint},
					Source:         Source{Kind: "broken"},
				}},
			}).Normalize(),
			want: "invalid source.kind",
		},
		{
			name: "invalid coverage",
			spec: (&Spec{
				Version: Version1,
				Series: []Series{{
					ID:             "series-1",
					Representation: Representation{Kind: RepresentationRawPoint},
					Quality:        Quality{Coverage: 2},
				}},
			}).Normalize(),
			want: "quality.coverage must be between 0 and 1",
		},
		{
			name: "invalid annotation kind",
			spec: (&Spec{Version: Version1, Annotations: []Annotation{{Kind: "broken"}}}).Normalize(),
			want: "invalid annotation kind",
		},
		{
			name: "undefined series axis",
			spec: (&Spec{Version: Version1, Axes: Axes{Y: []Axis{{ID: "value", Type: AxisTypeLinear}}}, Series: []Series{{
				ID:             "series-1",
				Axis:           "missing",
				Representation: Representation{Kind: RepresentationRawPoint, Fields: []string{"x", "y"}},
			}}}).Normalize(),
			want: "axis \"missing\" is not defined",
		},
		{
			name: "undefined annotation axis",
			spec: (&Spec{Version: Version1, Axes: Axes{Y: []Axis{{ID: "value", Type: AxisTypeLinear}}}, Annotations: []Annotation{{
				Kind: AnnotationKindLine,
				Axis: "missing",
			}}}).Normalize(),
			want: "axis \"missing\" is not defined",
		},
		{
			name: "invalid series style key",
			spec: (&Spec{
				Version: Version1,
				Series: []Series{{
					ID:             "series-1",
					Representation: Representation{Kind: RepresentationRawPoint},
					Style:          map[string]any{"unknown": true},
				}},
			}).Normalize(),
			want: "unsupported style key",
		},
		{
			name: "invalid series opacity range",
			spec: (&Spec{
				Version: Version1,
				Series: []Series{{
					ID:             "series-1",
					Representation: Representation{Kind: RepresentationRawPoint},
					Style:          map[string]any{"opacity": 2},
				}},
			}).Normalize(),
			want: "opacity must be between 0 and 1",
		},
		{
			name: "invalid annotation preferred renderer",
			spec: (&Spec{
				Version: Version1,
				Annotations: []Annotation{{
					Kind:  AnnotationKindLine,
					Style: map[string]any{"preferredRenderer": "line"},
				}},
			}).Normalize(),
			want: "unsupported style key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.spec.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestValidateStyleSuccess(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Series: []Series{{
			ID:             "sensor-1",
			Representation: Representation{Kind: RepresentationTimeBucketBand, Fields: []string{"time", "min", "max", "avg"}},
			Style: map[string]any{
				"preferredRenderer": "line-band",
				"color":             "#3366cc",
				"bandColor":         "#99bbff",
				"lineColor":         "#224499",
				"opacity":           0.25,
				"lineWidth":         2,
			},
		}},
		Annotations: []Annotation{{
			Kind:  AnnotationKindRange,
			Style: map[string]any{"color": "#ffcc00", "opacity": 0.3},
		}},
	}).Normalize()

	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() returned unexpected style error: %v", err)
	}
}

func TestValidateRepresentationFieldsSuccess(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Series: []Series{
			{
				ID:             "value-1",
				Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
			},
			{
				ID:             "band-1",
				Representation: Representation{Kind: RepresentationTimeBucketBand, Fields: []string{"time", "min", "avg"}},
			},
			{
				ID:             "hist-1",
				Representation: Representation{Kind: RepresentationDistributionHistogram, Fields: []string{"binStart", "binEnd", "count"}},
			},
			{
				ID: "box-1",
				Representation: Representation{
					Kind:          RepresentationDistributionBoxplot,
					Fields:        []string{"category", "low", "q1", "median", "q3", "high"},
					OutlierFields: []string{"category", "value"},
				},
			},
			{
				ID:             "event-1",
				Representation: Representation{Kind: RepresentationEventPoint, Fields: []string{"time", "value", "label"}},
			},
		},
	}).Normalize()

	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() returned unexpected representation field error: %v", err)
	}
}

func TestValidateAxisReferencesSuccess(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Axes: Axes{
			X: Axis{ID: "time", Type: AxisTypeTime},
			Y: []Axis{{ID: "value", Type: AxisTypeLinear}},
		},
		Series: []Series{{
			ID:             "series-1",
			Axis:           "value",
			Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
		}},
		Annotations: []Annotation{{Kind: AnnotationKindLine, Axis: "value"}, {Kind: AnnotationKindRange, Axis: "time"}},
	}).Normalize()

	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() returned unexpected axis reference error: %v", err)
	}
}

func TestMarshalOmitsZeroFields(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Series: []Series{{
			ID:             "sensor-1",
			Representation: Representation{Kind: RepresentationRawPoint},
		}},
	}).Normalize()

	buf, err := Marshal(spec)
	if err != nil {
		t.Fatalf("Marshal() returned unexpected error: %v", err)
	}

	got := string(buf)
	want := `{"version":1,"series":[{"id":"sensor-1","representation":{"kind":"raw-point"}}]}`
	if got != want {
		t.Fatalf("unexpected JSON\nwant: %s\ngot:  %s", want, got)
	}
}

func TestParseRoundTrip(t *testing.T) {
	input := `{
		"version": 1,
		"domain": {"kind": "time", "tz": "UTC"},
		"axes": {
			"x": {"type": "time"},
			"y": [{"id": "value", "type": "linear"}]
		},
		"series": [{
			"id": "sensor-1",
			"axis": "value",
			"representation": {"kind": "time-bucket-value", "aggregation": "avg", "fields": ["time", "value"]}
		}],
		"annotations": [{"kind": "range", "axis": "x"}]
	}`

	spec, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString() returned unexpected error: %v", err)
	}
	if spec.Version != Version1 {
		t.Fatalf("expected version %d, got %d", Version1, spec.Version)
	}
	if spec.Domain.Kind != DomainKindTime {
		t.Fatalf("expected domain kind %q, got %q", DomainKindTime, spec.Domain.Kind)
	}
	if len(spec.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(spec.Series))
	}
	if spec.Series[0].Representation.Kind != RepresentationTimeBucketValue {
		t.Fatalf("expected representation kind %q, got %q", RepresentationTimeBucketValue, spec.Series[0].Representation.Kind)
	}
	if len(spec.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(spec.Annotations))
	}

	buf, err := MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() returned unexpected error: %v", err)
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(buf, &decoded); err != nil {
		t.Fatalf("MarshalIndent() produced invalid JSON: %v", err)
	}
	if _, ok := decoded["series"]; !ok {
		t.Fatal("expected marshaled JSON to contain series")
	}
}

func TestParseEpochTimeNumbersPreserved(t *testing.T) {
	input := `{
		"version": 1,
		"domain": {
			"kind": "time",
			"timeFormat": "epoch",
			"timeUnit": "ns",
			"from": 1775174400000000000,
			"to": 1775217600000000000
		},
		"series": [{
			"id": "maintenance-window",
			"representation": {"kind": "event-range", "fields": ["from", "to", "label"]},
			"data": [[1775210400000000000, 1775214000000000000, "maintenance"]]
		}]
	}`

	spec, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString() returned unexpected error: %v", err)
	}
	from, ok := spec.Domain.From.(json.Number)
	if !ok {
		t.Fatalf("expected domain.from to be json.Number, got %T", spec.Domain.From)
	}
	if from.String() != "1775174400000000000" {
		t.Fatalf("expected exact epoch ns, got %s", from.String())
	}
	seriesFrom, ok := spec.Series[0].Data[0].([]any)[0].(json.Number)
	if !ok {
		t.Fatalf("expected series time value to be json.Number, got %T", spec.Series[0].Data[0].([]any)[0])
	}
	if seriesFrom.String() != "1775210400000000000" {
		t.Fatalf("expected exact series epoch ns, got %s", seriesFrom.String())
	}
	NormalizeSpecTimeValues(spec)
	if got, ok := spec.Domain.From.(string); !ok || got != "1775174400000000000" {
		t.Fatalf("expected normalized domain.from string, got %#v", spec.Domain.From)
	}
}

func TestParseRejectsInvalidJSONAndInvalidSpec(t *testing.T) {
	if _, err := ParseString("not-json"); err == nil {
		t.Fatal("expected invalid JSON error")
	}
	if _, err := ParseString(`{"version":1,"series":[{"id":"series-1"}]}`); err == nil {
		t.Fatal("expected invalid spec error")
	}
}
