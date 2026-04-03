package advn

import (
	"strings"
	"testing"
	"time"
)

func boolPtr(value bool) *bool {
	return &value
}

func TestToSVGTimeBandAndAnnotations(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain: Domain{
			Kind:       DomainKindTime,
			TimeFormat: TimeFormatNano,
			From:       "1712102400000000000",
			To:         "1712102520000000000",
		},
		Axes: Axes{
			X: Axis{ID: "time", Type: AxisTypeTime, Label: "Time"},
			Y: []Axis{{ID: "value", Type: AxisTypeLinear, Label: "Value"}},
		},
		Series: []Series{
			{
				ID:   "sensor-band",
				Name: "sensor-band",
				Axis: "value",
				Style: map[string]any{
					"color":     "#3366cc",
					"bandColor": "#99bbff",
					"lineWidth": 2,
					"opacity":   0.22,
				},
				Representation: Representation{Kind: RepresentationTimeBucketBand, Fields: []string{"time", "min", "max", "avg"}},
				Data: []any{
					[]any{"1712102400000000000", 10.0, 14.0, 12.0},
					[]any{"1712102460000000000", 11.0, 15.0, 13.0},
					[]any{"1712102520000000000", 9.0, 16.0, 12.5},
				},
			},
			{
				ID:   "maintenance",
				Name: "maintenance",
				Style: map[string]any{
					"color":   "#ffcc00",
					"opacity": 0.25,
				},
				Representation: Representation{Kind: RepresentationEventRange, Fields: []string{"from", "to", "label"}},
				Data: []any{
					[]any{"1712102430000000000", "1712102490000000000", "maintenance"},
				},
			},
		},
		Annotations: []Annotation{{Kind: AnnotationKindLine, Axis: "value", Value: 13.5, Style: map[string]any{"color": "#cf222e"}}},
	}).Normalize()

	output, err := ToSVG(spec, &SVGOptions{Title: "ADVN SVG", ShowLegend: boolPtr(true), Width: 800, Height: 360, Timeformat: TimeFormatRFC3339, TZ: "UTC"})
	if err != nil {
		t.Fatalf("ToSVG() returned unexpected error: %v", err)
	}
	text := string(output)
	for _, expected := range []string{
		"<svg ",
		"data-advn-role=\"background\"",
		"data-advn-role=\"axes\"",
		"data-advn-role=\"series\"",
		"data-advn-role=\"annotations\"",
		"data-advn-role=\"legend\"",
		"ADVN SVG",
		"sensor-band",
		"fill=\"#99bbff\"",
		"00:0",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected SVG output to contain %q", expected)
		}
	}
	if strings.Contains(text, "1712102400000000000") {
		t.Fatal("expected epoch nanoseconds to be normalized for axis labels, not rendered raw")
	}
}

func TestToSVGHistogram(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Series: []Series{{
			ID:             "hist",
			Name:           "hist",
			Style:          map[string]any{"color": "#ff8800", "opacity": 0.6},
			Representation: Representation{Kind: RepresentationDistributionHistogram, Fields: []string{"binStart", "binEnd", "count"}},
			Data: []any{
				[]any{0, 10, 3},
				[]any{10, 20, 8},
			},
		}},
		Axes: Axes{Y: []Axis{{ID: "y", Type: AxisTypeLinear, Label: "Count"}}},
	}).Normalize()

	output, err := ToSVG(spec, &SVGOptions{Width: 720, Height: 320, ShowLegend: boolPtr(false)})
	if err != nil {
		t.Fatalf("ToSVG() returned unexpected error: %v", err)
	}
	text := string(output)
	if strings.Count(text, "<rect ") < 3 {
		t.Fatalf("expected histogram to render multiple rects, got %q", text)
	}
	for _, expected := range []string{"fill=\"#ff8800\"", "data-advn-role=\"axes\"", "data-advn-role=\"series\""} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected SVG output to contain %q", expected)
		}
	}
}

func TestToSVGBoxplot(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Series: []Series{{
			ID:    "box",
			Name:  "box",
			Style: map[string]any{"color": "#00aa88"},
			Representation: Representation{
				Kind:          RepresentationDistributionBoxplot,
				Fields:        []string{"category", "low", "q1", "median", "q3", "high"},
				OutlierFields: []string{"category", "value"},
			},
			Data: []any{
				[]any{"A", 1, 2, 3, 4, 5},
				[]any{"B", 2, 3, 4, 5, 6},
			},
			Extra: map[string]any{"outliers": []any{[]any{"A", 7}}},
		}},
		Axes: Axes{Y: []Axis{{ID: "y", Type: AxisTypeLinear, Label: "Value"}}},
	}).Normalize()

	output, err := ToSVG(spec, &SVGOptions{Width: 720, Height: 320, ShowLegend: boolPtr(false)})
	if err != nil {
		t.Fatalf("ToSVG() returned unexpected error: %v", err)
	}
	text := string(output)
	for _, expected := range []string{"fill=\"#00aa88\"", ">A<", ">B<", "<circle ", "<line "} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected SVG output to contain %q", expected)
		}
	}
}

func TestToSVGRejectsInvalidOptions(t *testing.T) {
	_, err := ToSVG((&Spec{Version: Version1}).Normalize(), &SVGOptions{Width: -1})
	if err == nil || !strings.Contains(err.Error(), "width") {
		t.Fatalf("expected width validation error, got %v", err)
	}
}

func TestToSVGMultipleYAxesAndLegendWrap(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime},
		Axes: Axes{
			X: Axis{ID: "time", Type: AxisTypeTime, Label: "Time"},
			Y: []Axis{
				{ID: "temp", Type: AxisTypeLinear, Label: "Temperature"},
				{ID: "load", Type: AxisTypeLinear, Label: "Load"},
			},
		},
		Series: []Series{
			{
				ID:             "temperature-long-name-series",
				Name:           "temperature-long-name-series",
				Axis:           "temp",
				Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
				Data: []any{
					[]any{"2026-04-03T00:00:00Z", 20},
					[]any{"2026-04-03T01:00:00Z", 24},
				},
			},
			{
				ID:             "load-long-name-series",
				Name:           "load-long-name-series",
				Axis:           "load",
				Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
				Data: []any{
					[]any{"2026-04-03T00:00:00Z", 0.4},
					[]any{"2026-04-03T01:00:00Z", 0.7},
				},
			},
			{
				ID:             "network-long-name-series",
				Name:           "network-long-name-series",
				Axis:           "temp",
				Representation: Representation{Kind: RepresentationEventPoint, Fields: []string{"time", "value", "label"}},
				Data: []any{
					[]any{"2026-04-03T00:30:00Z", 22, "spike"},
				},
			},
		},
	}).Normalize()

	output, err := ToSVG(spec, &SVGOptions{Width: 540, Height: 300, ShowLegend: boolPtr(true)})
	if err != nil {
		t.Fatalf("ToSVG() returned unexpected error: %v", err)
	}
	text := string(output)
	for _, expected := range []string{
		"data-advn-axis=\"temp\" data-advn-side=\"left\"",
		"data-advn-axis=\"load\" data-advn-side=\"right\"",
		"Temperature",
		"Load",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected SVG output to contain %q", expected)
		}
	}
	if strings.Count(text, "data-advn-legend-row=") < 2 {
		t.Fatalf("expected wrapped legend rows, got %q", text)
	}
}

func TestToSVGTimeTickFormatting(t *testing.T) {
	oldLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() {
		time.Local = oldLocal
	})

	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime},
		Axes:    Axes{X: Axis{ID: "time", Type: AxisTypeTime, Label: "Time"}},
		Series: []Series{{
			ID:             "short-span",
			Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
			Data: []any{
				[]any{"2026-04-03T00:00:00Z", 1},
				[]any{"2026-04-03T00:01:00Z", 2},
				[]any{"2026-04-03T00:02:00Z", 3},
			},
		}},
	}).Normalize()

	output, err := ToSVG(spec, &SVGOptions{Width: 640, Height: 280, ShowLegend: boolPtr(false)})
	if err != nil {
		t.Fatalf("ToSVG() returned unexpected error: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, ">00:00<") && !strings.Contains(text, ">00:01<") {
		t.Fatalf("expected short-span time ticks to use compact time labels, got %q", text)
	}
	if strings.Contains(text, "T00:00:00Z") {
		t.Fatalf("expected compact time ticks instead of full RFC3339 labels, got %q", text)
	}
}

func TestToSVGWithGoTimeDomainValues(t *testing.T) {
	start := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	spec := (&Spec{
		Version: Version1,
		Domain: Domain{
			Kind: DomainKindTime,
			From: start,
			To:   end,
		},
		Axes: Axes{
			X: Axis{ID: "time", Type: AxisTypeTime, Label: "Time"},
			Y: []Axis{{ID: "value", Type: AxisTypeLinear, Label: "Value"}},
		},
		Series: []Series{{
			ID:             "series-1",
			Name:           "series-1",
			Axis:           "value",
			Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
			Data: []any{
				[]any{start, 10},
				[]any{start.Add(time.Minute), 12},
				[]any{end, 11},
			},
		}},
	}).Normalize()

	output, err := ToSVG(spec, &SVGOptions{Width: 640, Height: 280, ShowLegend: boolPtr(false)})
	if err != nil {
		t.Fatalf("ToSVG() returned unexpected error for Go time domain values: %v", err)
	}
	text := string(output)
	for _, expected := range []string{"<svg ", "series-1", "Time", "Value"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected SVG output to contain %q, got %q", expected, text)
		}
	}
}

func TestToSVGWithTimeOverrides(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain: Domain{
			Kind:       DomainKindTime,
			TimeFormat: TimeFormatNano,
			From:       "1712102400000000000",
			To:         "1712102520000000000",
		},
		Axes: Axes{
			X: Axis{ID: "time", Type: AxisTypeTime, Label: "Time"},
			Y: []Axis{{ID: "value", Type: AxisTypeLinear, Label: "Value"}},
		},
		Series: []Series{{
			ID:             "series-1",
			Axis:           "value",
			Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
			Data: []any{
				[]any{"1712102400000000000", 10},
				[]any{"1712102460000000000", 12},
			},
		}},
	}).Normalize()

	output, err := ToSVG(spec, &SVGOptions{Width: 640, Height: 280, ShowLegend: boolPtr(false), Timeformat: TimeFormatRFC3339, TZ: "Asia/Seoul"})
	if err != nil {
		t.Fatalf("ToSVG() returned unexpected error: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, ">09:00<") && !strings.Contains(text, ">09:01<") {
		t.Fatalf("expected timezone-adjusted SVG tick labels, got %q", text)
	}
}

func TestBuildSVGLayoutAddsTopInset(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime},
		Axes: Axes{
			X: Axis{ID: "time", Type: AxisTypeTime, Label: "Time"},
			Y: []Axis{{ID: "value", Type: AxisTypeLinear, Label: "Value"}},
		},
		Series: []Series{{
			ID:             "series-1",
			Axis:           "value",
			Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
			Data: []any{
				[]any{"2026-04-03T00:00:00Z", 10},
				[]any{"2026-04-03T00:01:00Z", 12},
			},
		}},
	}).Normalize()

	options, err := normalizeSVGOptions(&SVGOptions{Width: 960, Height: 420, ShowLegend: boolPtr(false)})
	if err != nil {
		t.Fatalf("resolveSVGOptions() returned unexpected error: %v", err)
	}
	layout, err := buildSVGLayout(spec, options)
	if err != nil {
		t.Fatalf("buildSVGLayout() returned unexpected error: %v", err)
	}
	baseTop := float64(options.Padding)
	if layout.Plot.Y <= baseTop {
		t.Fatalf("expected plot y to move below base top %v, got %v", baseTop, layout.Plot.Y)
	}
	bottom := layout.Plot.Y + layout.Plot.Height
	expectedBottom := float64(options.Height) - float64(options.Padding) - (float64(options.FontSize)*2 + 16)
	if bottom != expectedBottom {
		t.Fatalf("expected bottom edge to remain at %v, got %v", expectedBottom, bottom)
	}
}
