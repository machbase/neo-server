package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/jsh/advn"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/util/mdconv"
)

var (
	ginContextType = reflect.TypeOf((*gin.Context)(nil))
	contextType    = reflect.TypeOf((*context.Context)(nil)).Elem()
	webConsoleType = reflect.TypeOf((*WebConsole)(nil))
)

type rpcImplicitParamResolver func(paramType reflect.Type) (reflect.Value, bool)

var defaultJsonRpcController = &service.Controller{}

func buildRpcCallParams(handler any, rawParams []any, resolveImplicit rpcImplicitParamResolver) ([]reflect.Value, error) {
	return service.BuildRpcCallParams(handler, rawParams, service.JsonRpcImplicitParamResolver(resolveImplicit))
}

func rpcMarkdownRender(markdown string, darkMode bool) (string, error) {
	w := &strings.Builder{}
	conv := mdconv.New(mdconv.WithDarkMode(darkMode))
	if err := conv.ConvertString(markdown, w); err != nil {
		return "", err
	}
	return w.String(), nil
}

func rpcVizspecRender(vizspec map[string]any) (map[string]any, error) {
	if vizspec == nil {
		return map[string]any{}, nil
	}
	return vizspec, nil
}

func rpcVizspecExport(vizspec map[string]any, format string) (map[string]any, error) {
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
		raw, err := advn.ToSVG(spec, nil)
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
		raw, err := advn.ToPNG(spec, nil, nil)
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
		option, err := advn.ToEChartsOption(spec)
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

func mapToADVNSpec(vizspec map[string]any) (*advn.Spec, error) {
	if vizspec == nil {
		return nil, fmt.Errorf("vizspec is required")
	}

	// If payload already looks like ADVN spec, parse it directly.
	if _, ok := vizspec["version"]; ok {
		raw, err := json.Marshal(vizspec)
		if err != nil {
			return nil, err
		}
		return advn.Parse(raw)
	}

	parsed, err := parseVizspecTimeseries(vizspec)
	if err != nil {
		return nil, err
	}

	spec := (&advn.Spec{
		Version: advn.Version1,
		Domain: advn.Domain{
			Kind:       advn.DomainKindCategory,
			Categories: append([]string{}, parsed.X...),
		},
		Axes: advn.Axes{
			X: advn.Axis{ID: "x", Type: advn.AxisTypeCategory, Label: "X"},
			Y: []advn.Axis{{ID: "y", Type: advn.AxisTypeLinear, Label: "Y"}},
		},
		Series: []advn.Series{},
		Meta: advn.Meta{
			Producer: "vizspec.render",
		},
	}).Normalize()

	if parsed.Title != "" {
		spec.View = advn.View{PreferredRenderer: "vizspec"}
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
		spec.Series = append(spec.Series, advn.Series{
			ID:   fmt.Sprintf("s%d", i+1),
			Name: name,
			Axis: "y",
			Representation: advn.Representation{
				Kind:   advn.RepresentationRawPoint,
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
		clr := parseHexColor(vizColor(i))
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

func vizColor(i int) string {
	palette := []string{"#2f7cff", "#1f8f63", "#cc6a2d", "#8a43c7", "#e43f5a", "#1584a3"}
	if len(palette) == 0 {
		return "#2f7cff"
	}
	return palette[i%len(palette)]
}

func parseHexColor(s string) color.RGBA {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "#")
	if len(s) != 6 {
		return color.RGBA{R: 0x2f, G: 0x7c, B: 0xff, A: 0xff}
	}
	r, errR := strconv.ParseUint(s[0:2], 16, 8)
	g, errG := strconv.ParseUint(s[2:4], 16, 8)
	b, errB := strconv.ParseUint(s[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return color.RGBA{R: 0x2f, G: 0x7c, B: 0xff, A: 0xff}
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}
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

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
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

// handleHttpRpc handles HTTP POST requests for JSON-RPC
func (svr *httpd) handleHttpRpc(ctx *gin.Context) {
	var req eventbus.RPC

	// Parse JSON-RPC request
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// Invalid JSON-RPC request format
		rsp := map[string]any{
			"jsonrpc": "2.0",
			"id":      nil,
			"error": map[string]any{
				"code":    -32700,
				"message": "Parse error",
			},
		}
		ctx.JSON(http.StatusOK, rsp)
		return
	}

	rsp := map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
	}

	ctl := svr.rpcController
	if ctl == nil {
		ctl = defaultJsonRpcController
	}
	result, rpcErr := ctl.CallJsonRpc(req.Method, req.Params, func(paramType reflect.Type) (reflect.Value, bool) {
		switch {
		case paramType == ginContextType:
			return reflect.ValueOf(ctx), true
		case paramType == contextType:
			// Pass gin.Context as context.Context to preserve requester information.
			return reflect.ValueOf(ctx), true
		default:
			return reflect.Value{}, false
		}
	})
	if rpcErr == nil {
		rsp["result"] = result
	} else {
		code := rpcErr.Code
		message := rpcErr.Message
		if code == -32603 {
			code = -32000
		}
		if rpcErr.Code == -32601 {
			message = "Method not found"
		}
		rsp["error"] = map[string]any{
			"code":    code,
			"message": message,
		}
	}

	// Always return HTTP 200 as per JSON-RPC 2.0 specification
	ctx.JSON(http.StatusOK, rsp)
}
