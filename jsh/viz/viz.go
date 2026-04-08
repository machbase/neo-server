package viz

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func init() {

}

func RPCVizspecRender(vizspec map[string]any) (map[string]any, error) {
	if vizspec == nil {
		return nil, fmt.Errorf("vizspec is required")
	}

	normalized := cloneMap(vizspec)
	if normalized == nil {
		normalized = map[string]any{}
	}

	// Accept legacy schema alias and always return vizspec schema.
	schema := strings.ToLower(strings.TrimSpace(fmt.Sprint(normalized["schema"])))
	if schema == "" || schema == "advn/v1" || schema == "vizspec/v1" {
		normalized["schema"] = "vizspec/v1"
	}

	if strings.TrimSpace(fmt.Sprint(normalized["kind"])) == "" {
		normalized["kind"] = "timeseries"
	}

	normalizeLegacyDataShape(normalized)
	normalizePreferredHints(normalized)

	if _, err := mapToADVNSpec(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	raw, err := json.Marshal(src)
	if err != nil {
		ret := make(map[string]any, len(src))
		for k, v := range src {
			ret[k] = v
		}
		return ret
	}
	ret := map[string]any{}
	if err := json.Unmarshal(raw, &ret); err != nil {
		ret = make(map[string]any, len(src))
		for k, v := range src {
			ret[k] = v
		}
	}
	return ret
}

func normalizeLegacyDataShape(vizspec map[string]any) {
	if vizspec == nil {
		return
	}
	if _, hasData := vizspec["data"]; hasData {
		return
	}
	if _, hasX := vizspec["x"]; !hasX {
		return
	}
	if _, hasSeries := vizspec["series"]; !hasSeries {
		return
	}
	vizspec["data"] = map[string]any{
		"x":      vizspec["x"],
		"series": vizspec["series"],
	}
}

func normalizePreferredHints(vizspec map[string]any) {
	if vizspec == nil {
		return
	}
	meta, _ := vizspec["meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
		vizspec["meta"] = meta
	}

	preferred := []string{}
	seen := map[string]struct{}{}
	appendPreferred := func(values []any) {
		for _, one := range values {
			normalized := normalizePreferredValue(one)
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			preferred = append(preferred, normalized)
		}
	}

	if existing, ok := anyToSlice(meta["preferred"]); ok {
		appendPreferred(existing)
	}
	if hint, ok := vizspec["clientHint"].(map[string]any); ok {
		if hPreferred, ok := anyToSlice(hint["preferred"]); ok {
			appendPreferred(hPreferred)
		}
		if renderer, ok := hint["renderer"]; ok {
			appendPreferred([]any{renderer})
		}
	}

	if len(preferred) > 0 {
		meta["preferred"] = preferred
	}
}

func normalizePreferredValue(value any) string {
	v := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
	switch v {
	case "echarts", "svg", "png", "vizspec":
		return v
	default:
		return ""
	}
}

func RPCVizspecExport(vizspec map[string]any, format string) (map[string]any, error) {
	if vizspec == nil {
		return nil, fmt.Errorf("vizspec is required")
	}
	spec, err := mapToADVNSpec(vizspec)
	if err != nil {
		return nil, err
	}
	f := strings.ToLower(strings.TrimSpace(format))
	if f == "" {
		f = "svg"
	}
	switch f {
	case "svg":
		raw, err := ToSVG(spec, nil)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"schema":   "vizspec-export/v1",
			"format":   "svg",
			"mimeType": "image/svg+xml",
			"data":     string(raw),
		}, nil
	case "png":
		raw, err := ToPNG(spec, nil, nil)
		if err != nil {
			parsed, parseErr := parseVizspecTimeseries(vizspec)
			if parseErr != nil {
				return nil, err
			}
			raw, err = renderVizspecPNG(parsed)
			if err != nil {
				return nil, err
			}
		}
		return map[string]any{
			"schema":   "vizspec-export/v1",
			"format":   "png",
			"mimeType": "image/png",
			"data":     base64.StdEncoding.EncodeToString(raw),
		}, nil
	case "echarts":
		option, err := ToEChartsOption(spec)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"schema":   "vizspec-export/v1",
			"format":   "echarts",
			"mimeType": "application/json",
			"data":     option,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

func mapToADVNSpec(vizspec map[string]any) (*Spec, error) {
	if vizspec == nil {
		return nil, fmt.Errorf("vizspec is required")
	}

	// If payload already looks like ADVN spec, parse it directly.
	if _, ok := vizspec["version"]; ok {
		raw, err := json.Marshal(vizspec)
		if err != nil {
			return nil, err
		}
		return Parse(raw)
	}

	parsed, err := parseVizspecTimeseries(vizspec)
	if err != nil {
		return nil, err
	}

	spec := (&Spec{
		Version: Version1,
		Domain: Domain{
			Kind:       DomainKindCategory,
			Categories: append([]string{}, parsed.X...),
		},
		Axes: Axes{
			X: Axis{ID: "x", Type: AxisTypeCategory, Label: "X"},
			Y: []Axis{{ID: "y", Type: AxisTypeLinear, Label: "Y"}},
		},
		Series: []Series{},
		Meta: Meta{
			Producer: "vizspec.render",
		},
	}).Normalize()

	if parsed.Title != "" {
		spec.View = View{PreferredRenderer: "vizspec"}
		spec.Meta.LODGroup = parsed.Title
	}

	for i, s := range parsed.Series {
		data := []any{}
		for j := 0; j < len(parsed.X) && j < len(s.Values); j++ {
			n := s.Values[j]
			if math.IsNaN(n) || math.IsInf(n, 0) {
				continue
			}
			data = append(data, map[string]any{"x": parsed.X[j], "y": n})
		}
		if len(data) == 0 {
			continue
		}
		name := strings.TrimSpace(s.Name)
		if name == "" {
			name = fmt.Sprintf("series-%d", i+1)
		}
		spec.Series = append(spec.Series, Series{
			ID:   fmt.Sprintf("s%d", i+1),
			Name: name,
			Axis: "y",
			Representation: Representation{
				Kind:   RepresentationRawPoint,
				Fields: []string{"x", "y"},
			},
			Data: data,
		})
	}

	if len(spec.Series) == 0 {
		return nil, fmt.Errorf("vizspec has no series data")
	}

	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return spec, nil
}

type vizSeries struct {
	Name   string
	Values []float64
}

type vizParsed struct {
	X      []string
	Series []vizSeries
	Title  string
	Min    float64
	Max    float64
}

func parseVizspecTimeseries(vizspec map[string]any) (*vizParsed, error) {
	schema, _ := vizspec["schema"].(string)
	if schema != "vizspec/v1" && schema != "advn/v1" {
		return nil, fmt.Errorf("unsupported schema: %s", schema)
	}
	ret := &vizParsed{X: []string{}, Series: []vizSeries{}, Min: math.Inf(1), Max: math.Inf(-1)}
	if title, ok := vizspec["title"].(string); ok {
		ret.Title = title
	}
	if meta, ok := vizspec["meta"].(map[string]any); ok {
		if title, ok := meta["title"].(string); ok && title != "" {
			ret.Title = title
		}
	}
	data, _ := vizspec["data"].(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	if xVals, ok := anyToSlice(data["x"]); ok {
		for _, one := range xVals {
			ret.X = append(ret.X, fmt.Sprint(one))
		}
	}
	if rawSeries, ok := anyToSlice(data["series"]); ok {
		for _, s := range rawSeries {
			obj, ok := s.(map[string]any)
			if !ok {
				continue
			}
			name := fmt.Sprint(obj["name"])
			if name == "" {
				name = fmt.Sprintf("series-%d", len(ret.Series)+1)
			}
			valsAny, ok := anyToSlice(obj["data"])
			if !ok || len(valsAny) == 0 {
				continue
			}
			vals := make([]float64, 0, len(valsAny))
			for _, v := range valsAny {
				n, ok := toFloat64(v)
				if !ok {
					vals = append(vals, math.NaN())
					continue
				}
				vals = append(vals, n)
				if n < ret.Min {
					ret.Min = n
				}
				if n > ret.Max {
					ret.Max = n
				}
			}
			ret.Series = append(ret.Series, vizSeries{Name: name, Values: vals})
		}
	}
	if len(ret.X) == 0 {
		maxLen := 0
		for _, s := range ret.Series {
			if len(s.Values) > maxLen {
				maxLen = len(s.Values)
			}
		}
		for i := 0; i < maxLen; i++ {
			ret.X = append(ret.X, strconv.Itoa(i+1))
		}
	}
	if len(ret.Series) == 0 || len(ret.X) == 0 {
		return nil, fmt.Errorf("vizspec has no series data")
	}
	if !isFinite(ret.Min) || !isFinite(ret.Max) {
		ret.Min = 0
		ret.Max = 1
	}
	if ret.Min == ret.Max {
		ret.Min--
		ret.Max++
	}
	return ret, nil
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func renderVizspecPNG(v *vizParsed) ([]byte, error) {
	const width = 760
	const height = 280
	const padL = 48.0
	const padR = 16.0
	const padT = 20.0
	const padB = 34.0
	innerW := float64(width) - padL - padR
	innerH := float64(height) - padT - padB
	count := len(v.X)
	stepX := innerW
	if count > 1 {
		stepX = innerW / float64(count-1)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	axisColor := color.RGBA{R: 0x8f, G: 0xa0, B: 0x8f, A: 0xff}
	drawLine(img, int(padL), int(padT+innerH), int(padL+innerW), int(padT+innerH), axisColor)
	drawLine(img, int(padL), int(padT), int(padL), int(padT+innerH), axisColor)
	for i, s := range v.Series {
		clr, err := parseHexColor(vizColor(i))
		if err != nil {
			clr = color.RGBA{R: 0x2f, G: 0x7c, B: 0xff, A: 0xff}
		}
		var prevX, prevY int
		havePrev := false
		for j := 0; j < count && j < len(s.Values); j++ {
			n := s.Values[j]
			if math.IsNaN(n) || math.IsInf(n, 0) {
				havePrev = false
				continue
			}
			x := int(math.Round(padL + stepX*float64(j)))
			y := int(math.Round(padT + innerH*(1.0-(n-v.Min)/(v.Max-v.Min))))
			if havePrev {
				drawLine(img, prevX, prevY, x, y, clr)
			}
			prevX, prevY = x, y
			havePrev = true
		}
	}
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, clr color.Color) {
	dx := int(math.Abs(float64(x1 - x0)))
	dy := -int(math.Abs(float64(y1 - y0)))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(img.Bounds()) {
			img.Set(x0, y0, clr)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
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

func renderVizspecSVG(v *vizParsed) string {
	const width = 760.0
	const height = 280.0
	const padL = 48.0
	const padR = 16.0
	const padT = 20.0
	const padB = 34.0
	innerW := width - padL - padR
	innerH := height - padT - padB
	count := len(v.X)
	stepX := innerW
	if count > 1 {
		stepX = innerW / float64(count-1)
	}
	b := &strings.Builder{}
	b.WriteString(fmt.Sprintf("<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 %.0f %.0f\" width=\"%.0f\" height=\"%.0f\">", width, height, width, height))
	b.WriteString("<rect x=\"0\" y=\"0\" width=\"100%\" height=\"100%\" fill=\"#ffffff\"/>")
	b.WriteString(fmt.Sprintf("<path d=\"M %.2f %.2f L %.2f %.2f M %.2f %.2f L %.2f %.2f\" stroke=\"#8fa08f\" stroke-width=\"1\" fill=\"none\"/>",
		padL, padT+innerH, padL+innerW, padT+innerH, padL, padT, padL, padT+innerH))
	for i, s := range v.Series {
		color := vizColor(i)
		parts := []string{}
		for j := 0; j < count && j < len(s.Values); j++ {
			n := s.Values[j]
			if math.IsNaN(n) || math.IsInf(n, 0) {
				continue
			}
			x := padL + stepX*float64(j)
			y := padT + innerH*(1.0-(n-v.Min)/(v.Max-v.Min))
			if len(parts) == 0 {
				parts = append(parts, fmt.Sprintf("M %.2f %.2f", x, y))
			} else {
				parts = append(parts, fmt.Sprintf("L %.2f %.2f", x, y))
			}
		}
		if len(parts) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("<path d=\"%s\" stroke=\"%s\" stroke-width=\"2\" fill=\"none\"/>", strings.Join(parts, " "), color))
	}
	b.WriteString("</svg>")
	return b.String()
}

func vizColor(i int) string {
	palette := []string{"#2f7cff", "#1f8f63", "#cc6a2d", "#8a43c7", "#e43f5a", "#1584a3"}
	if len(palette) == 0 {
		return "#2f7cff"
	}
	return palette[i%len(palette)]
}

func anyToSlice(v any) ([]any, bool) {
	if v == nil {
		return nil, false
	}
	sv, ok := v.([]any)
	if ok {
		return sv, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	ret := make([]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		ret = append(ret, rv.Index(i).Interface())
	}
	return ret, true
}
