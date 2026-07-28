package sqlext

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Execute      bool
	Format       string
	Output       string
	Source       string
	Preview      int
	Timeout      time.Duration
	Params       []any
	Timeformat   string
	TimeLocation string
	BinaryFormat string
	Header       string
	Delimiter    string
}

func ParseOptions(info string) Options {
	ret := Options{
		Execute:      false,
		Format:       "table",
		Output:       "table",
		Source:       "hide",
		Preview:      10,
		Timeout:      0,
		Timeformat:   "ns",
		TimeLocation: "UTC",
		BinaryFormat: "hex",
		Header:       "",
		Delimiter:    ",",
	}
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
		case "output":
			ret.Output = normalizeStringValue(val)
			ret.Format = ret.Output
		case "source":
			ret.Source = normalizeStringValue(val)
		case "format":
			ret.Format = normalizeStringValue(val)
			ret.Output = ret.Format
		case "preview":
			ret.Preview = parsePreviewValue(val)
		case "timeout":
			if dur, err := time.ParseDuration(val); err == nil {
				ret.Timeout = dur
			}
		case "params", "p":
			ret.Params = parseParamValue(val)
		case "timeformat":
			ret.Timeformat = normalizeStringValue(val)
		case "tz":
			ret.TimeLocation = normalizeStringValue(val)
		case "binaryformat":
			ret.BinaryFormat = normalizeStringValue(val)
		case "header":
			ret.Header = normalizeStringValue(val)
		case "delimiter":
			ret.Delimiter = normalizeStringValue(val)
		}
	}
	return ret
}

func parsePreviewValue(val string) int {
	v := strings.ToLower(strings.TrimSpace(val))
	if v == "" || v == "unlimit" || v == "none" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return 10
}

func parseParamValue(val string) []any {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		var out []any
		if err := json.Unmarshal([]byte(val), &out); err == nil {
			return out
		}
	}
	return []any{normalizeStringValue(val)}
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

func normalizeStringValue(val string) string {
	val = strings.TrimSpace(val)
	if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
		return val[1 : len(val)-1]
	}
	return val
}

func SelectSourceLines(source string, spec string) string {
	if spec == "" || strings.EqualFold(spec, "hide") {
		return ""
	}
	if strings.EqualFold(spec, "all") || strings.EqualFold(spec, "show") {
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
