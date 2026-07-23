package chartext

import (
	"fmt"
	"strconv"
	"strings"
)

func parseInlineOptions(raw string) (map[string]any, error) {
	out := map[string]any{}
	parts, err := splitTopLevel(raw, ',')
	if err != nil {
		return nil, fmt.Errorf("invalid chart fence options: %w", err)
	}
	for _, p := range parts {
		entry := strings.TrimSpace(p)
		if entry == "" {
			continue
		}
		eq := strings.Index(entry, "=")
		if eq <= 0 || eq == len(entry)-1 {
			return nil, fmt.Errorf("invalid chart fence option entry: %q", entry)
		}
		key := strings.TrimSpace(entry[:eq])
		if key == "" {
			return nil, fmt.Errorf("invalid chart fence option key: %q", entry)
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
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func optionStringList(v any) ([]string, bool) {
	toList := func(in []string) []string {
		out := make([]string, 0, len(in))
		for _, item := range in {
			for _, part := range strings.Split(item, ",") {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	}

	switch vv := v.(type) {
	case string:
		return toList([]string{vv}), true
	case []string:
		return toList(vv), true
	case []any:
		tmp := make([]string, 0, len(vv))
		for _, item := range vv {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			tmp = append(tmp, s)
		}
		return toList(tmp), true
	default:
		return nil, false
	}
}
