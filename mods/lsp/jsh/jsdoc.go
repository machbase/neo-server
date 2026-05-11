package jsh

import (
	"regexp"
	"sort"
	"strings"

	base "github.com/machbase/neo-server/v8/mods/lsp"
)

type jsdocInfo struct {
	Name        string
	Kind        base.CompletionItemKind
	Signature   *base.SignatureInfo
	Description string
	Returns     string
}

var (
	jsdocFunctionPattern    = regexp.MustCompile(`(?s)/\*\*(.*?)\*/\s*(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(([^)]*)\)`)
	jsdocClassPattern       = regexp.MustCompile(`(?s)/\*\*(.*?)\*/\s*class\s+([A-Za-z_$][\w$]*)`)
	moduleExportsPattern    = regexp.MustCompile(`(?s)module\.exports\s*=\s*\{(.*?)\};`)
	moduleExportNamePattern = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)\b\s*(?::\s*([A-Za-z_$][\w$]*))?\s*,?`)
	blockCommentPattern     = regexp.MustCompile(`(?s)/\*.*?\*/`)
	lineCommentPattern      = regexp.MustCompile(`(?m)//.*$`)
	paramPattern            = regexp.MustCompile(`^@param\s+(?:\{([^}]*)\}\s+)?([^\s-]+)\s*-?\s*(.*)$`)
	returnsPattern          = regexp.MustCompile(`^@returns?\s+(?:\{([^}]*)\}\s+)?(.*)$`)
)

func extractModuleSymbols(moduleID string, source string) []base.SymbolInfo {
	docs := extractJSDoc(source)
	exports := extractModuleExports(source)
	if len(exports) == 0 {
		for name := range docs {
			exports[name] = name
		}
	}

	labels := make([]string, 0, len(exports))
	for label := range exports {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	symbols := make([]base.SymbolInfo, 0, len(labels))
	for _, label := range labels {
		ref := exports[label]
		doc, ok := docs[ref]
		if !ok {
			doc = jsdocInfo{Name: ref, Kind: base.CompletionProperty, Description: "JSH module export"}
		}
		symbol := base.SymbolInfo{
			Label:         label,
			Kind:          doc.Kind,
			Category:      moduleID,
			Detail:        moduleID,
			Documentation: documentation(doc),
			InsertText:    label,
			Signature:     doc.Signature,
		}
		if symbol.Signature != nil && symbol.Signature.Label != "" {
			symbol.Detail = symbol.Signature.Label
		}
		symbols = append(symbols, symbol)
	}
	return symbols
}

func extractJSDoc(source string) map[string]jsdocInfo {
	docs := map[string]jsdocInfo{}
	for _, match := range jsdocFunctionPattern.FindAllStringSubmatch(source, -1) {
		name := match[2]
		params := parseParameters(match[3])
		doc := parseJSDoc(match[1], name, params)
		doc.Name = name
		doc.Kind = base.CompletionFunction
		docs[name] = doc
	}
	for _, match := range jsdocClassPattern.FindAllStringSubmatch(source, -1) {
		name := match[2]
		doc := parseJSDoc(match[1], name, nil)
		doc.Name = name
		doc.Kind = base.CompletionClass
		docs[name] = doc
	}
	return docs
}

func parseJSDoc(block string, name string, params []string) jsdocInfo {
	description := make([]string, 0)
	parameters := make([]base.ParameterInfo, 0)
	returns := ""
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if line == "" {
			continue
		}
		if match := paramPattern.FindStringSubmatch(line); match != nil {
			label := match[2]
			if match[1] != "" {
				label += ": " + match[1]
			}
			parameters = append(parameters, base.ParameterInfo{Label: label, Documentation: match[3]})
			continue
		}
		if match := returnsPattern.FindStringSubmatch(line); match != nil {
			returns = strings.TrimSpace(match[2])
			if match[1] != "" {
				returns = strings.TrimSpace(match[1] + " " + returns)
			}
			continue
		}
		if !strings.HasPrefix(line, "@") {
			description = append(description, line)
		}
	}
	if len(parameters) == 0 {
		for _, param := range params {
			parameters = append(parameters, base.ParameterInfo{Label: param})
		}
	}
	return jsdocInfo{
		Description: strings.Join(description, "\n"),
		Returns:     returns,
		Signature: &base.SignatureInfo{
			Label:         signatureLabel(name, parameters, returns),
			Documentation: strings.Join(description, "\n"),
			Parameters:    parameters,
		},
	}
}

func parseParameters(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	params := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			params = append(params, part)
		}
	}
	return params
}

func extractModuleExports(source string) map[string]string {
	ret := map[string]string{}
	match := moduleExportsPattern.FindStringSubmatch(source)
	if match == nil {
		return ret
	}
	body := lineCommentPattern.ReplaceAllString(blockCommentPattern.ReplaceAllString(match[1], ""), "")
	for _, exportMatch := range moduleExportNamePattern.FindAllStringSubmatch(body, -1) {
		label := exportMatch[1]
		ref := label
		if exportMatch[2] != "" {
			ref = exportMatch[2]
		}
		ret[label] = ref
	}
	return ret
}

func signatureLabel(name string, params []base.ParameterInfo, returns string) string {
	labels := make([]string, 0, len(params))
	for _, param := range params {
		labels = append(labels, param.Label)
	}
	label := name + "(" + strings.Join(labels, ", ") + ")"
	if returns != "" {
		label += ": " + returns
	}
	return label
}

func documentation(doc jsdocInfo) string {
	parts := make([]string, 0, 2)
	if doc.Signature != nil && doc.Signature.Label != "" {
		parts = append(parts, "```js\n"+doc.Signature.Label+"\n```")
	}
	if doc.Description != "" {
		parts = append(parts, doc.Description)
	}
	if doc.Returns != "" {
		parts = append(parts, "Returns: "+doc.Returns)
	}
	if len(parts) == 0 {
		return "JSH module export"
	}
	return strings.Join(parts, "\n\n")
}
