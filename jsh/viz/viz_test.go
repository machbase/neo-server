package viz

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestRPCVizspecRenderNormalize(t *testing.T) {
	in := map[string]any{
		"schema": "advn/v1",
		"kind":   "",
		"x":      []any{"a", "b"},
		"series": []any{map[string]any{"name": "cpu", "data": []any{1, 2}}},
		"clientHint": map[string]any{
			"preferred": []any{"svg", "png", "bad"},
			"renderer":  "echarts",
		},
	}

	out, err := RPCVizspecRender(in)
	if err != nil {
		t.Fatalf("RPCVizspecRender() error: %v", err)
	}
	if out["schema"] != "vizspec/v1" {
		t.Fatalf("unexpected schema: %v", out["schema"])
	}
	if out["kind"] != "timeseries" {
		t.Fatalf("unexpected kind: %v", out["kind"])
	}
	if _, ok := out["data"].(map[string]any); !ok {
		t.Fatalf("expected normalized data map, got %T", out["data"])
	}
	meta, ok := out["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta map, got %T", out["meta"])
	}
	preferred, ok := anyToSlice(meta["preferred"])
	if !ok {
		t.Fatalf("expected preferred list")
	}
	if len(preferred) != 3 {
		t.Fatalf("expected 3 preferred items, got %d (%v)", len(preferred), preferred)
	}
}

func TestRPCVizspecRenderErrors(t *testing.T) {
	if _, err := RPCVizspecRender(nil); err == nil {
		t.Fatal("expected error for nil vizspec")
	}
	if _, err := RPCVizspecRender(map[string]any{"schema": "unknown/v1"}); err == nil {
		t.Fatal("expected unsupported schema error")
	}
}

func TestCloneMapFallback(t *testing.T) {
	in := map[string]any{
		"func": func() {},
		"num":  7,
	}
	out := cloneMap(in)
	if out == nil {
		t.Fatal("cloneMap returned nil")
	}
	if _, ok := out["func"].(func()); !ok {
		t.Fatalf("expected function to be copied by fallback path, got %T", out["func"])
	}
}

func TestRPCVizspecExport(t *testing.T) {
	vizspec := map[string]any{
		"schema": "vizspec/v1",
		"data": map[string]any{
			"x": []any{"1", "2", "3"},
			"series": []any{
				map[string]any{"name": "s1", "data": []any{1, 3, 2}},
			},
		},
	}

	svgOut, err := RPCVizspecExport(vizspec, "svg")
	if err != nil {
		t.Fatalf("svg export error: %v", err)
	}
	if svgOut["format"] != "svg" {
		t.Fatalf("unexpected svg format: %v", svgOut["format"])
	}
	svgData, _ := svgOut["data"].(string)
	if !strings.Contains(svgData, "<svg") {
		t.Fatalf("svg output does not contain <svg: %s", svgData)
	}

	echartsOut, err := RPCVizspecExport(vizspec, "echarts")
	if err != nil {
		t.Fatalf("echarts export error: %v", err)
	}
	if echartsOut["format"] != "echarts" {
		t.Fatalf("unexpected echarts format: %v", echartsOut["format"])
	}
	if _, ok := echartsOut["data"].(map[string]any); !ok {
		t.Fatalf("expected echarts option map, got %T", echartsOut["data"])
	}

	pngOut, err := RPCVizspecExport(vizspec, "png")
	if err != nil {
		t.Fatalf("png export error: %v", err)
	}
	pngData, _ := pngOut["data"].(string)
	rawPNG, err := base64.StdEncoding.DecodeString(pngData)
	if err != nil || len(rawPNG) == 0 {
		t.Fatalf("png output decode failed, err=%v len=%d", err, len(rawPNG))
	}

	if _, err := RPCVizspecExport(vizspec, "bad"); err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func TestMapToADVNSpecAndParseVizspec(t *testing.T) {
	vizspec := map[string]any{
		"schema": "vizspec/v1",
		"title":  "demo",
		"data": map[string]any{
			"series": []any{
				map[string]any{"name": "a", "data": []any{1, 2, 3}},
				map[string]any{"name": "b", "data": []any{3, "x", 1}},
			},
		},
	}

	parsed, err := parseVizspecTimeseries(vizspec)
	if err != nil {
		t.Fatalf("parseVizspecTimeseries() error: %v", err)
	}
	if len(parsed.X) != 3 {
		t.Fatalf("expected derived x length 3, got %d", len(parsed.X))
	}
	if !isFinite(parsed.Min) || !isFinite(parsed.Max) {
		t.Fatalf("expected finite min/max, got %v/%v", parsed.Min, parsed.Max)
	}

	spec, err := mapToADVNSpec(vizspec)
	if err != nil {
		t.Fatalf("mapToADVNSpec() error: %v", err)
	}
	if spec.Version != Version1 {
		t.Fatalf("unexpected spec version: %d", spec.Version)
	}
	if len(spec.Series) == 0 {
		t.Fatal("expected non-empty series")
	}

	if _, err := parseVizspecTimeseries(map[string]any{"schema": "bad/v1"}); err == nil {
		t.Fatal("expected unsupported schema error")
	}
	if _, err := mapToADVNSpec(map[string]any{"schema": "vizspec/v1", "data": map[string]any{}}); err == nil {
		t.Fatal("expected no series data error")
	}
}

func TestHelpersInVizGo(t *testing.T) {
	if normalizePreferredValue(" SVG ") != "svg" {
		t.Fatal("normalizePreferredValue failed")
	}
	if normalizePreferredValue("invalid") != "" {
		t.Fatal("normalizePreferredValue should ignore unknown value")
	}

	if isFinite(math.NaN()) || isFinite(math.Inf(1)) || !isFinite(1.0) {
		t.Fatal("isFinite() mismatch")
	}

	keys := sortedKeys(map[string]any{"b": 1, "a": 2})
	if !reflect.DeepEqual(keys, []string{"a", "b"}) {
		t.Fatalf("sortedKeys mismatch: %v", keys)
	}

	arr, ok := anyToSlice([]int{1, 2, 3})
	if !ok || len(arr) != 3 {
		t.Fatalf("anyToSlice() failed for []int: ok=%v len=%d", ok, len(arr))
	}
	if _, ok := anyToSlice(10); ok {
		t.Fatal("anyToSlice should reject scalar")
	}

	if vizColor(0) == "" || vizColor(100) == "" {
		t.Fatal("vizColor should always return a color")
	}
}

func TestRenderHelpers(t *testing.T) {
	v := &vizParsed{
		X: []string{"1", "2", "3"},
		Series: []vizSeries{
			{Name: "s1", Values: []float64{1, 3, 2}},
			{Name: "s2", Values: []float64{math.NaN(), 2, 4}},
		},
		Min: 1,
		Max: 4,
	}

	svg := renderVizspecSVG(v)
	if !strings.Contains(svg, "<svg") || !strings.Contains(svg, "<path") {
		t.Fatalf("unexpected svg content: %s", svg)
	}

	raw, err := renderVizspecPNG(v)
	if err != nil {
		t.Fatalf("renderVizspecPNG() error: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("renderVizspecPNG() returned empty bytes")
	}

	invalid := map[string]any{"schema": "vizspec/v1", "data": map[string]any{"x": []any{"1"}, "series": []any{map[string]any{"data": []any{"x"}}}}}
	parsed, err := parseVizspecTimeseries(invalid)
	if err != nil {
		t.Fatalf("expected parse success for all-nan, got error: %v", err)
	}
	if parsed.Min != 0 || parsed.Max != 1 {
		t.Fatalf("expected fallback min/max 0/1, got %v/%v", parsed.Min, parsed.Max)
	}
}

func TestMapToADVNSpecVersionPath(t *testing.T) {
	baseSpec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindCategory, Categories: []string{"a", "b"}},
		Axes: Axes{
			X: Axis{ID: "x", Type: AxisTypeCategory},
			Y: []Axis{{ID: "y", Type: AxisTypeLinear}},
		},
		Series: []Series{{
			ID:             "s1",
			Name:           "s1",
			Axis:           "y",
			Representation: Representation{Kind: RepresentationRawPoint, Fields: []string{"x", "y"}},
			Data:           []any{map[string]any{"x": "a", "y": 1}, map[string]any{"x": "b", "y": 2}},
		}},
	}).Normalize()

	raw, err := json.Marshal(baseSpec)
	if err != nil {
		t.Fatalf("marshal spec error: %v", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload error: %v", err)
	}

	out, err := mapToADVNSpec(payload)
	if err != nil {
		t.Fatalf("mapToADVNSpec(version path) error: %v", err)
	}
	if out.Version != Version1 || len(out.Series) != 1 {
		t.Fatalf("unexpected parsed spec: version=%d series=%d", out.Version, len(out.Series))
	}
}
