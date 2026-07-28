package jshext

import (
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Execute bool
	Source  string
	Result  string
	Timeout time.Duration
}

func ParseOptions(info string) Options {
	ret := Options{Execute: false, Source: "hide", Result: "default", Timeout: 0}
	raw := strings.TrimSpace(info)
	if start := strings.Index(raw, "{"); start >= 0 {
		if end := strings.LastIndex(raw, "}"); end > start {
			raw = raw[start+1 : end]
		}
	}
	for _, part := range splitTopLevelCSV(raw) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "execute":
			ret.Execute = strings.EqualFold(val, "true")
		case "source":
			ret.Source = normalizeSourceValue(val)
		case "result":
			ret.Result = normalizeStringValue(val)
		case "timeout":
			if dur, err := time.ParseDuration(val); err == nil {
				ret.Timeout = dur
			}
		}
	}
	return ret
}

func splitTopLevelCSV(raw string) []string {
	var parts []string
	var buf strings.Builder
	depth := 0
	inQuote := false
	for _, r := range raw {
		switch r {
		case '[':
			depth++
			buf.WriteRune(r)
		case ']':
			if depth > 0 {
				depth--
			}
			buf.WriteRune(r)
		case '"':
			inQuote = !inQuote
			buf.WriteRune(r)
		case ',':
			if depth == 0 && !inQuote {
				part := strings.TrimSpace(buf.String())
				if part != "" {
					parts = append(parts, part)
				}
				buf.Reset()
				continue
			}
			buf.WriteRune(r)
		default:
			buf.WriteRune(r)
		}
	}
	if part := strings.TrimSpace(buf.String()); part != "" {
		parts = append(parts, part)
	}
	return parts
}

func normalizeSourceValue(val string) string {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		inner := strings.TrimSpace(val[1 : len(val)-1])
		if inner == "" {
			return "all"
		}
		parts := splitTopLevelCSV(inner)
		for i, p := range parts {
			parts[i] = normalizeStringValue(p)
		}
		return strings.Join(parts, ",")
	}
	return normalizeStringValue(val)
}

func normalizeStringValue(val string) string {
	val = strings.TrimSpace(val)
	if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
		return val[1 : len(val)-1]
	}
	return val
}

func SelectSourceLines(source string, spec string) string {
	if spec == "" || strings.EqualFold(spec, "all") {
		return source
	}
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	var out []string
	for _, token := range splitTopLevelCSV(spec) {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if strings.Contains(token, "-") {
			parts := strings.SplitN(token, "-", 2)
			if len(parts) == 2 {
				start, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
				end, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
				if start > 0 && end >= start {
					for i := start - 1; i < end && i < len(lines); i++ {
						out = append(out, lines[i])
					}
				}
			}
			continue
		}
		if idx, err := strconv.Atoi(token); err == nil && idx > 0 && idx <= len(lines) {
			out = append(out, lines[idx-1])
		}
	}
	return strings.Join(out, "\n")
}
