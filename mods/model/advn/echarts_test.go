package advn

import (
	"encoding/json"
	"testing"
)

func TestToEChartsOption(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime, TZ: "UTC"},
		Axes: Axes{
			X: Axis{ID: "time", Type: AxisTypeTime, Label: "Time"},
			Y: []Axis{{ID: "value", Type: AxisTypeLinear, Label: "Temperature"}},
		},
		Series: []Series{{
			ID:   "sensor-1",
			Name: "sensor-1",
			Axis: "value",
			Style: map[string]any{
				"color":     "#3366cc",
				"bandColor": "#99bbff",
				"lineWidth": 2,
				"opacity":   0.25,
			},
			Representation: Representation{
				Kind:   RepresentationTimeBucketBand,
				Fields: []string{"time", "min", "max", "avg", "count"},
			},
			Data: []any{
				[]any{"2026-04-03T00:00:00Z", 18.1, 21.7, 19.8, 60},
				[]any{"2026-04-03T00:01:00Z", 18.0, 21.5, 19.7, 60},
			},
		}},
		Annotations: []Annotation{
			{Kind: AnnotationKindLine, Axis: "value", Value: 25.0, Label: "warning"},
			{Kind: AnnotationKindRange, Axis: "x", From: "2026-04-03T10:00:00Z", To: "2026-04-03T11:00:00Z", Label: "maintenance"},
		},
		View: View{DefaultZoom: []float64{0, 100}},
	}).Normalize()

	option, err := ToEChartsOption(spec)
	if err != nil {
		t.Fatalf("ToEChartsOption() returned unexpected error: %v", err)
	}
	if option["tooltip"] == nil {
		t.Fatal("expected tooltip in option")
	}
	if option["xAxis"] == nil {
		t.Fatal("expected xAxis in option")
	}
	if option["yAxis"] == nil {
		t.Fatal("expected yAxis in option")
	}
	seriesList, ok := option["series"].([]map[string]any)
	if !ok {
		t.Fatalf("expected series to be []map[string]any, got %T", option["series"])
	}
	if len(seriesList) != 3 {
		t.Fatalf("expected 3 echarts series for band output, got %d", len(seriesList))
	}
	if seriesList[0]["stack"] != "band:sensor-1" {
		t.Fatalf("expected lower bound stack %q, got %v", "band:sensor-1", seriesList[0]["stack"])
	}
	if seriesList[0]["silent"] != true {
		t.Fatalf("expected lower bound series to be silent, got %v", seriesList[0]["silent"])
	}
	if seriesList[1]["areaStyle"] == nil {
		t.Fatal("expected areaStyle on band range series")
	}
	areaStyle := seriesList[1]["areaStyle"].(map[string]any)
	if areaStyle["color"] != "#99bbff" {
		t.Fatalf("expected band color %q, got %v", "#99bbff", areaStyle["color"])
	}
	if areaStyle["opacity"] != 0.25 {
		t.Fatalf("expected band opacity %v, got %v", 0.25, areaStyle["opacity"])
	}
	if seriesList[1]["stack"] != "band:sensor-1" {
		t.Fatalf("expected range stack %q, got %v", "band:sensor-1", seriesList[1]["stack"])
	}
	if seriesList[2]["name"] != "sensor-1" {
		t.Fatalf("expected avg series name %q, got %v", "sensor-1", seriesList[2]["name"])
	}
	lineStyle := seriesList[2]["lineStyle"].(map[string]any)
	if lineStyle["color"] != "#3366cc" {
		t.Fatalf("expected avg line color %q, got %v", "#3366cc", lineStyle["color"])
	}
	if lineStyle["width"] != 2.0 {
		t.Fatalf("expected avg line width %v, got %v", 2.0, lineStyle["width"])
	}
	if seriesList[2]["markLine"] == nil {
		t.Fatal("expected markLine on avg series")
	}
	if seriesList[2]["markArea"] == nil {
		t.Fatal("expected markArea on avg series")
	}
	dataZoom, ok := option["dataZoom"].([]map[string]any)
	if !ok || len(dataZoom) != 1 {
		t.Fatalf("expected one dataZoom entry, got %T", option["dataZoom"])
	}
}

func TestToEChartsOptionHistogram(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Series: []Series{{
			ID:   "hist-1",
			Name: "hist-1",
			Style: map[string]any{
				"color":   "#ff8800",
				"opacity": 0.6,
			},
			Representation: Representation{
				Kind:   RepresentationDistributionHistogram,
				Fields: []string{"binStart", "binEnd", "count"},
			},
			Data: []any{
				[]any{0, 10, 3},
				[]any{10, 20, 8},
			},
		}},
	}).Normalize()

	option, err := ToEChartsOption(spec)
	if err != nil {
		t.Fatalf("ToEChartsOption() returned unexpected error: %v", err)
	}
	xAxis := option["xAxis"].(map[string]any)
	if xAxis["type"] != "category" {
		t.Fatalf("expected histogram xAxis type %q, got %v", "category", xAxis["type"])
	}
	if xAxis["data"].([]any)[0] != "0-10" {
		t.Fatalf("expected histogram label %q, got %v", "0-10", xAxis["data"].([]any)[0])
	}
	seriesList := option["series"].([]map[string]any)
	if seriesList[0]["type"] != "bar" {
		t.Fatalf("expected histogram series type %q, got %v", "bar", seriesList[0]["type"])
	}
	itemStyle := seriesList[0]["itemStyle"].(map[string]any)
	if itemStyle["color"] != "#ff8800" {
		t.Fatalf("expected histogram color %q, got %v", "#ff8800", itemStyle["color"])
	}
}

func TestToEChartsOptionBoxplot(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Series: []Series{{
			ID:   "box-1",
			Name: "box-1",
			Representation: Representation{
				Kind:   RepresentationDistributionBoxplot,
				Fields: []string{"category", "low", "q1", "median", "q3", "high"},
			},
			Data: []any{
				[]any{"A", 1, 2, 3, 4, 5},
				[]any{"B", 2, 3, 4, 5, 6},
			},
			Extra: map[string]any{
				"outliers": []any{
					[]any{"A", 7},
				},
			},
		}},
	}).Normalize()

	option, err := ToEChartsOption(spec)
	if err != nil {
		t.Fatalf("ToEChartsOption() returned unexpected error: %v", err)
	}
	xAxis := option["xAxis"].(map[string]any)
	if xAxis["type"] != "category" {
		t.Fatalf("expected boxplot xAxis type %q, got %v", "category", xAxis["type"])
	}
	seriesList := option["series"].([]map[string]any)
	if len(seriesList) != 2 {
		t.Fatalf("expected boxplot + outlier series, got %d", len(seriesList))
	}
	if seriesList[0]["type"] != "boxplot" {
		t.Fatalf("expected first series type %q, got %v", "boxplot", seriesList[0]["type"])
	}
	if seriesList[1]["type"] != "scatter" {
		t.Fatalf("expected second series type %q, got %v", "scatter", seriesList[1]["type"])
	}
}

func TestToEChartsOptionEventSeries(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime},
		Series: []Series{
			{
				ID:   "event-point-1",
				Name: "alerts",
				Style: map[string]any{
					"color":   "#ff3300",
					"opacity": 0.9,
				},
				Representation: Representation{
					Kind:   RepresentationEventPoint,
					Fields: []string{"time", "value", "label", "severity"},
				},
				Data: []any{
					[]any{"2026-04-03T10:15:00Z", 93.0, "threshold exceeded", "warn"},
				},
			},
			{
				ID:   "event-range-1",
				Name: "maintenance",
				Style: map[string]any{
					"color":   "#ffcc00",
					"opacity": 0.3,
				},
				Representation: Representation{
					Kind:   RepresentationEventRange,
					Fields: []string{"from", "to", "label"},
				},
				Data: []any{
					[]any{"2026-04-03T10:00:00Z", "2026-04-03T11:00:00Z", "maintenance"},
				},
			},
		},
	}).Normalize()

	option, err := ToEChartsOption(spec)
	if err != nil {
		t.Fatalf("ToEChartsOption() returned unexpected error: %v", err)
	}
	seriesList := option["series"].([]map[string]any)
	if len(seriesList) != 2 {
		t.Fatalf("expected 2 event series, got %d", len(seriesList))
	}
	if seriesList[0]["type"] != "scatter" {
		t.Fatalf("expected event-point series type %q, got %v", "scatter", seriesList[0]["type"])
	}
	if seriesList[1]["type"] != "line" {
		t.Fatalf("expected event-range series type %q, got %v", "line", seriesList[1]["type"])
	}
	if seriesList[0]["itemStyle"].(map[string]any)["color"] != "#ff3300" {
		t.Fatalf("expected event-point color %q, got %v", "#ff3300", seriesList[0]["itemStyle"].(map[string]any)["color"])
	}
	if seriesList[1]["markArea"] == nil {
		t.Fatal("expected markArea on event-range series")
	}
}

func TestToEChartsOptionRejectsUnsupportedRepresentation(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Series: []Series{{
			ID:             "hist-1",
			Representation: Representation{Kind: RepresentationDistributionHistogram},
		}},
	}).Normalize()

	if _, err := ToEChartsOption(spec); err == nil {
		t.Fatal("expected unsupported representation error")
	}
}

func TestToEChartsOptionEpochNanoseconds(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain: Domain{
			Kind:       DomainKindTime,
			TimeFormat: TimeFormatEpoch,
			TimeUnit:   TimeUnitNanosecond,
			From:       json.Number("1775174400000000000"),
			To:         json.Number("1775217600000000000"),
		},
		Series: []Series{{
			ID:   "event-range-1",
			Name: "maintenance",
			Representation: Representation{
				Kind:   RepresentationEventRange,
				Fields: []string{"from", "to", "label"},
			},
			Data: []any{
				[]any{json.Number("1775210400000000000"), json.Number("1775214000000000000"), "maintenance"},
			},
		}},
	}).Normalize()

	option, err := ToEChartsOption(spec)
	if err != nil {
		t.Fatalf("ToEChartsOption() returned unexpected error: %v", err)
	}
	seriesList := option["series"].([]map[string]any)
	markArea := seriesList[0]["markArea"].(map[string]any)
	areas := markArea["data"].([]any)
	points := areas[0].([]map[string]any)
	if points[0]["xAxis"] != "2026-04-03T10:00:00Z" {
		t.Fatalf("expected normalized from time, got %v", points[0]["xAxis"])
	}
	if points[1]["xAxis"] != "2026-04-03T11:00:00Z" {
		t.Fatalf("expected normalized to time, got %v", points[1]["xAxis"])
	}
}
