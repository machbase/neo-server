package geomapext

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var sizePattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:px|%|vh|vw|rem|em)?$`)

type renderConfig struct {
	Width      string
	Height     string
	Tile       string
	TileOption string
	Fit        string
	Center     [2]float64
	HasCenter  bool
	Zoom       int
	Grayscale  float64
	Loader     string
	LeafletSrc string
	LeafletCSS string
	CDNSrc     string
	CDNCSS     string
}

func defaultRenderConfig(_ bool) renderConfig {
	return renderConfig{
		Width:      "100%",
		Height:     "400px",
		Tile:       "https://tile.openstreetmap.org/{z}/{x}/{y}.png",
		Fit:        "auto",
		Center:     [2]float64{51.505, -0.09},
		HasCenter:  true,
		Zoom:       13,
		Grayscale:  0,
		Loader:     "auto",
		LeafletSrc: "/web/geomap/leaflet.js",
		LeafletCSS: "/web/geomap/leaflet.css",
		CDNSrc:     "https://unpkg.com/leaflet@1.9.4/dist/leaflet.js",
		CDNCSS:     "https://unpkg.com/leaflet@1.9.4/dist/leaflet.css",
	}
}

func applyFenceOptions(cfg *renderConfig, opts map[string]any) error {
	if len(opts) == 0 {
		return nil
	}
	if width, ok := opts["width"]; ok {
		s, ok := optionString(width)
		if !ok {
			return fmt.Errorf("geomap width must be a string")
		}
		if !sizePattern.MatchString(s) {
			return fmt.Errorf("invalid geomap width value: %q", s)
		}
		cfg.Width = s
	}
	if height, ok := opts["height"]; ok {
		s, ok := optionString(height)
		if !ok {
			return fmt.Errorf("geomap height must be a string")
		}
		if !sizePattern.MatchString(s) {
			return fmt.Errorf("invalid geomap height value: %q", s)
		}
		cfg.Height = s
	}
	if tile, ok := opts["tile"]; ok {
		s, ok := optionString(tile)
		if !ok {
			return fmt.Errorf("geomap tile must be a string")
		}
		s = strings.TrimSpace(s)
		if strings.EqualFold(s, "default") {
			cfg.Tile = "https://tile.openstreetmap.org/{z}/{x}/{y}.png"
		} else {
			if !isTileTemplateURL(s) {
				return fmt.Errorf("geomap tile must be default or a URL template with {z},{x},{y}")
			}
			cfg.Tile = s
		}
	}
	if tileOption, ok := opts["tileOption"]; ok {
		s, ok := optionString(tileOption)
		if !ok {
			return fmt.Errorf("geomap tileOption must be a string")
		}
		cfg.TileOption = s
	}
	if fit, ok := opts["fit"]; ok {
		s, ok := optionString(fit)
		if !ok {
			return fmt.Errorf("geomap fit must be a string")
		}
		s = strings.ToLower(s)
		if s != "auto" && s != "bounds" && s != "center" {
			return fmt.Errorf("geomap fit must be auto, bounds or center")
		}
		cfg.Fit = s
	}
	if center, ok := opts["center"]; ok {
		arr, ok := optionFloat64Pair(center)
		if !ok {
			return fmt.Errorf("geomap center must be [lat,lon]")
		}
		cfg.Center = arr
		cfg.HasCenter = true
	}
	if zoom, ok := opts["zoom"]; ok {
		z, ok := optionInt(zoom)
		if !ok {
			return fmt.Errorf("geomap zoom must be a number")
		}
		cfg.Zoom = z
	}
	if grayscale, ok := opts["grayscale"]; ok {
		v, ok := optionFloat64(grayscale)
		if !ok {
			return fmt.Errorf("geomap grayscale must be a number")
		}
		if v < 0 || v > 1 {
			return fmt.Errorf("geomap grayscale must be between 0 and 1")
		}
		cfg.Grayscale = v
	}
	if loader, ok := opts["loader"]; ok {
		s, ok := optionString(loader)
		if !ok {
			return fmt.Errorf("geomap loader must be a string")
		}
		s = strings.ToLower(s)
		if s != "none" && s != "local" && s != "auto" {
			return fmt.Errorf("geomap loader must be none, local or auto")
		}
		cfg.Loader = s
	}
	if src, ok := opts["leafletSrc"]; ok {
		s, ok := optionString(src)
		if !ok {
			return fmt.Errorf("geomap leafletSrc must be a string")
		}
		cfg.LeafletSrc = s
	}
	if css, ok := opts["leafletCss"]; ok {
		s, ok := optionString(css)
		if !ok {
			return fmt.Errorf("geomap leafletCss must be a string")
		}
		cfg.LeafletCSS = s
	}
	if src, ok := opts["cdnSrc"]; ok {
		s, ok := optionString(src)
		if !ok {
			return fmt.Errorf("geomap cdnSrc must be a string")
		}
		cfg.CDNSrc = s
	}
	if css, ok := opts["cdnCss"]; ok {
		s, ok := optionString(css)
		if !ok {
			return fmt.Errorf("geomap cdnCss must be a string")
		}
		cfg.CDNCSS = s
	}
	return nil
}

func isTileTemplateURL(v string) bool {
	if !(strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")) {
		return false
	}
	return strings.Contains(v, "{z}") && strings.Contains(v, "{x}") && strings.Contains(v, "{y}")
}

func parseInlineOptions(raw string) (map[string]any, error) {
	out := map[string]any{}
	parts, err := splitTopLevel(raw, ',')
	if err != nil {
		return nil, fmt.Errorf("invalid geomap fence options: %w", err)
	}
	for _, p := range parts {
		entry := strings.TrimSpace(p)
		if entry == "" {
			continue
		}
		eq := strings.Index(entry, "=")
		if eq <= 0 || eq == len(entry)-1 {
			return nil, fmt.Errorf("invalid geomap fence option entry: %q", entry)
		}
		key := strings.TrimSpace(entry[:eq])
		if key == "" {
			return nil, fmt.Errorf("invalid geomap fence option key: %q", entry)
		}
		val, err := parseOptionValue(strings.TrimSpace(entry[eq+1:]))
		if err != nil {
			return nil, err
		}
		out[key] = val
	}

	return out, nil
}

func splitTopLevel(s string, sep rune) ([]string, error) {
	var parts []string
	start := 0
	depthBrace := 0
	depthBracket := 0
	inQuote := false
	escaped := false

	for i, r := range s {
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inQuote = false
			}
			continue
		}

		switch r {
		case '"':
			inQuote = true
		case '{':
			depthBrace++
		case '}':
			depthBrace--
			if depthBrace < 0 {
				return nil, fmt.Errorf("unexpected closing brace")
			}
		case '[':
			depthBracket++
		case ']':
			depthBracket--
			if depthBracket < 0 {
				return nil, fmt.Errorf("unexpected closing bracket")
			}
		default:
			if r == sep && depthBrace == 0 && depthBracket == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}

	if inQuote {
		return nil, fmt.Errorf("unterminated quote")
	}
	if depthBrace != 0 || depthBracket != 0 {
		return nil, fmt.Errorf("unbalanced option delimiters")
	}

	parts = append(parts, s[start:])
	return parts, nil
}

func parseOptionValue(raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty option value")
	}

	if strings.HasPrefix(raw, "\"") {
		v, err := strconv.Unquote(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid quoted value %q: %w", raw, err)
		}
		return v, nil
	}

	if strings.HasPrefix(raw, "[") {
		if !strings.HasSuffix(raw, "]") {
			return nil, fmt.Errorf("invalid array value %q", raw)
		}
		inner := strings.TrimSpace(raw[1 : len(raw)-1])
		if inner == "" {
			return []any{}, nil
		}
		items, err := splitTopLevel(inner, ',')
		if err != nil {
			return nil, fmt.Errorf("invalid array value %q: %w", raw, err)
		}
		ret := make([]any, 0, len(items))
		for _, item := range items {
			parsed, err := parseOptionValue(item)
			if err != nil {
				return nil, err
			}
			ret = append(ret, parsed)
		}
		return ret, nil
	}

	if raw == "true" || raw == "false" {
		return raw == "true", nil
	}
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f, nil
	}

	return raw, nil
}

func optionString(v any) (string, bool) {
	s, ok := optionStringConvertible(v)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func optionStringConvertible(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case bool:
		return strconv.FormatBool(val), true
	case int:
		return strconv.Itoa(val), true
	case int64:
		return strconv.FormatInt(val, 10), true
	case int32:
		return strconv.FormatInt(int64(val), 10), true
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32), true
	default:
		return "", false
	}
}

func optionFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int64:
		return float64(val), true
	case int:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func optionInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func optionFloat64Pair(v any) ([2]float64, bool) {
	var ret [2]float64
	switch val := v.(type) {
	case []any:
		if len(val) != 2 {
			return ret, false
		}
		v0, ok0 := optionFloat64(val[0])
		v1, ok1 := optionFloat64(val[1])
		if !ok0 || !ok1 {
			return ret, false
		}
		ret[0], ret[1] = v0, v1
		return ret, true
	case []float64:
		if len(val) != 2 {
			return ret, false
		}
		ret[0], ret[1] = val[0], val[1]
		return ret, true
	default:
		return ret, false
	}
}
