package httpext

import (
	"strconv"
	"strings"
)

type FenceOptions struct {
	ShowRequest     bool
	ShowLineNumbers bool
	IndentJSON      bool
	ClassStyles     map[string]string
	Warnings        []string
}

var styleKeyToClassName = map[string]string{
	"method":            "httpext-method",
	"path":              "httpext-path",
	"param-name":        "httpext-param-name",
	"param-value":       "httpext-param-value",
	"request-protocol":  "httpext-request-protocol",
	"header-key":        "httpext-header-key",
	"header-value":      "httpext-header-value",
	"response-protocol": "httpext-response-protocol",
	"status-code":       "httpext-status-code",
	"status-message":    "httpext-status-message",
	"body":              "httpext-body",
	"json-key":          "httpext-json-key",
	"json-string":       "httpext-json-string",
	"json-number":       "httpext-json-number",
	"json-boolean":      "httpext-json-boolean",
	"json-null":         "httpext-json-null",
	"json-punct":        "httpext-json-punct",
	"csv-delim":         "httpext-csv-delim",
}

func parseFenceOptions(info string) FenceOptions {
	ret := FenceOptions{
		ShowRequest:     true,
		ShowLineNumbers: false,
		IndentJSON:      true,
		ClassStyles:     map[string]string{},
		Warnings:        []string{},
	}
	trimmed := strings.TrimSpace(info)
	if trimmed == "" {
		return ret
	}
	space := strings.IndexAny(trimmed, " \t")
	if space < 0 {
		return ret
	}
	meta := strings.TrimSpace(trimmed[space+1:])
	if !strings.HasPrefix(meta, "{") || !strings.HasSuffix(meta, "}") {
		return ret
	}
	body := strings.TrimSpace(meta[1 : len(meta)-1])
	if body == "" {
		return ret
	}

	for _, part := range splitTopLevel(body, ',') {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		eq := strings.Index(entry, "=")
		if eq <= 0 || eq == len(entry)-1 {
			continue
		}
		key := strings.TrimSpace(entry[:eq])
		valRaw := strings.TrimSpace(entry[eq+1:])
		val := strings.ToLower(unquote(valRaw))
		switch key {
		case "hide-request":
			ret.ShowRequest = !(val == "true" || val == "1" || val == "yes")
		case "line-numbers":
			ret.ShowLineNumbers = (val == "true" || val == "1" || val == "yes")
		case "indent":
			ret.IndentJSON = !(val == "false" || val == "0" || val == "no")
		default:
			if !strings.HasPrefix(key, "style-") {
				continue
			}
			styleKey := strings.TrimPrefix(key, "style-")
			className, ok := resolveStyleClassName(styleKey)
			if !ok {
				ret.Warnings = append(ret.Warnings, "httpext: unknown style key \"style-"+styleKey+"\"")
				continue
			}
			styleValue := strings.TrimSpace(unquote(valRaw))
			if styleValue == "" {
				continue
			}
			ret.ClassStyles[className] = styleValue
		}
	}
	return ret
}

func resolveStyleClassName(styleKey string) (string, bool) {
	if className, ok := styleKeyToClassName[styleKey]; ok {
		return className, true
	}
	if !strings.HasPrefix(styleKey, "csv-col-") {
		return "", false
	}
	idxText := strings.TrimPrefix(styleKey, "csv-col-")
	idx, err := strconv.Atoi(idxText)
	if err != nil || idx < 0 || idx > 255 {
		return "", false
	}
	return "httpext-csv-col-" + strconv.Itoa(idx), true
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) < 2 {
		return v
	}
	if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
		return v[1 : len(v)-1]
	}
	return v
}

func splitTopLevel(s string, sep rune) []string {
	parts := []string{}
	start := 0
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if r == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if r == sep && !inSingleQuote && !inDoubleQuote {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
