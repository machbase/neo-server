package jsh

import (
	"regexp"
	"strings"

	base "github.com/machbase/neo-server/v8/mods/lsp"
)

var (
	requireCallPrefixPattern = regexp.MustCompile(`require\(\s*$`)
	requireBindingPattern    = regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*require\(\s*['"]([^'"]+)['"]\s*\)`)
)

func isRequireStringPosition(text string, pos base.Position) bool {
	prefix, ok := textBeforePosition(text, pos)
	if !ok {
		return false
	}
	quoteIdx := strings.LastIndexAny(prefix, "'\"")
	if quoteIdx < 0 {
		return false
	}
	beforeQuote := prefix[:quoteIdx]
	return requireCallPrefixPattern.FindStringIndex(beforeQuote) != nil
}

func moduleMemberContext(text string, pos base.Position) (string, bool) {
	prefix, ok := textBeforePosition(text, pos)
	if !ok {
		return "", false
	}
	dotIdx := strings.LastIndex(prefix, ".")
	if dotIdx < 0 {
		return "", false
	}
	objectEnd := dotIdx
	objectStart := objectEnd
	for objectStart > 0 && isWordRune(rune(prefix[objectStart-1])) {
		objectStart--
	}
	if objectStart == objectEnd {
		return "", false
	}
	localName := prefix[objectStart:objectEnd]
	return requireBindings(text)[localName], requireBindings(text)[localName] != ""
}

func moduleMemberAtPosition(text string, pos base.Position) (string, string, base.Range, bool) {
	word, rng := wordAtPosition(text, pos)
	if word == "" {
		return "", "", base.Range{}, false
	}
	prefix, ok := textBeforePosition(text, base.Position{Line: rng.Start.Line, Column: rng.Start.Column})
	if !ok || !strings.HasSuffix(prefix, ".") {
		return "", "", base.Range{}, false
	}
	objectEnd := len(prefix) - 1
	objectStart := objectEnd
	for objectStart > 0 && isWordRune(rune(prefix[objectStart-1])) {
		objectStart--
	}
	if objectStart == objectEnd {
		return "", "", base.Range{}, false
	}
	localName := prefix[objectStart:objectEnd]
	moduleID := requireBindings(text)[localName]
	if moduleID == "" {
		return "", "", base.Range{}, false
	}
	return moduleID, word, rng, true
}

func moduleMemberCallee(text string, callee string) (string, string, bool) {
	dotIdx := strings.LastIndex(callee, ".")
	if dotIdx <= 0 || dotIdx == len(callee)-1 {
		return "", "", false
	}
	localName := callee[:dotIdx]
	member := callee[dotIdx+1:]
	moduleID := requireBindings(text)[localName]
	if moduleID == "" {
		return "", "", false
	}
	return moduleID, member, true
}

func requireBindings(text string) map[string]string {
	bindings := map[string]string{}
	for _, match := range requireBindingPattern.FindAllStringSubmatch(text, -1) {
		if !strings.HasPrefix(match[2], "@jsh/") {
			bindings[match[1]] = match[2]
		}
	}
	return bindings
}

func textBeforePosition(text string, pos base.Position) (string, bool) {
	lines := strings.Split(text, "\n")
	if pos.Line <= 0 || pos.Line > len(lines) {
		return "", false
	}
	line := []rune(lines[pos.Line-1])
	column := pos.Column
	if column <= 0 {
		column = 1
	}
	idx := column - 1
	if idx > len(line) {
		idx = len(line)
	}
	var builder strings.Builder
	for i := 0; i < pos.Line-1; i++ {
		builder.WriteString(lines[i])
		builder.WriteByte('\n')
	}
	builder.WriteString(string(line[:idx]))
	return builder.String(), true
}

func findModuleSymbol(symbols []base.SymbolInfo, label string) (base.SymbolInfo, bool) {
	for _, symbol := range symbols {
		if symbol.Label == label {
			return symbol, true
		}
	}
	return base.SymbolInfo{}, false
}

type callFrame struct {
	open   int
	commas int
}

func callExpressionAtPosition(text string, pos base.Position) (string, int, bool) {
	prefix, ok := textBeforePosition(text, pos)
	if !ok {
		return "", 0, false
	}
	frames := make([]callFrame, 0)
	quote := rune(0)
	escaped := false
	for idx, r := range prefix {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' || r == '`' {
			quote = r
			continue
		}
		switch r {
		case '(':
			frames = append(frames, callFrame{open: idx})
		case ')':
			if len(frames) > 0 {
				frames = frames[:len(frames)-1]
			}
		case ',':
			if len(frames) > 0 {
				frames[len(frames)-1].commas++
			}
		}
	}
	if len(frames) == 0 {
		return "", 0, false
	}
	frame := frames[len(frames)-1]
	callee := calleeBeforeOpen(prefix, frame.open)
	if callee == "" {
		return "", 0, false
	}
	return callee, frame.commas, true
}

func calleeBeforeOpen(prefix string, open int) string {
	idx := open - 1
	for idx >= 0 && isSpaceRune(rune(prefix[idx])) {
		idx--
	}
	end := idx + 1
	for idx >= 0 {
		r := rune(prefix[idx])
		if !(r == '_' || r == '$' || r == '.' || isWordRune(r)) {
			break
		}
		idx--
	}
	return prefix[idx+1 : end]
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

func signatureHelp(signature *base.SignatureInfo, activeParameter int) *base.SignatureHelp {
	if signature == nil {
		return nil
	}
	if len(signature.Parameters) > 0 && activeParameter >= len(signature.Parameters) {
		activeParameter = len(signature.Parameters) - 1
	}
	if activeParameter < 0 {
		activeParameter = 0
	}
	return &base.SignatureHelp{Signatures: []base.SignatureInfo{*signature}, ActiveSignature: 0, ActiveParameter: activeParameter}
}
