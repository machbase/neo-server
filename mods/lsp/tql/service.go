package tql

import (
	"context"
	"errors"
	"strings"
	"unicode"

	base "github.com/machbase/neo-server/v8/mods/lsp"
	coretql "github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/tql/expression"
)

type Service struct {
	metadata base.Metadata
	items    []base.CompletionItem
	hovers   map[string]string
}

func NewService() *Service {
	metadata := BuildMetadata()
	items := make([]base.CompletionItem, 0, len(metadata.Symbols))
	hovers := make(map[string]string, len(metadata.Symbols))
	for _, symbol := range metadata.Symbols {
		insertText := symbol.InsertText
		if insertText == "" {
			insertText = symbol.Label
		}
		items = append(items, base.CompletionItem{
			Label:         symbol.Label,
			Kind:          symbol.Kind,
			Detail:        symbol.Detail,
			Documentation: symbol.Documentation,
			InsertText:    insertText,
		})
		hovers[strings.ToUpper(symbol.Label)] = hoverContents(symbol)
	}
	return &Service{metadata: metadata, items: items, hovers: hovers}
}

func BuildMetadata() base.Metadata {
	symbols := make([]base.SymbolInfo, 0, len(coretql.FxDefinitions))
	keywords := make([]base.KeywordInfo, 0, len(coretql.FxDefinitions))
	category := "general"
	for _, def := range coretql.FxDefinitions {
		if strings.HasPrefix(def.Name, "//") {
			category = strings.TrimSpace(strings.TrimPrefix(def.Name, "//"))
			continue
		}
		symbol := base.SymbolInfo{
			Label:         def.Name,
			Kind:          base.CompletionFunction,
			Category:      category,
			Detail:        category,
			Documentation: "TQL function",
			InsertText:    insertText(def.Name),
		}
		if kind, ok := coretql.StatementKindByFunctionName(def.Name); ok {
			symbol.StatementKind = statementKindString(kind)
		}
		symbol.Signature = tqlSignature(def.Name)
		symbols = append(symbols, symbol)
		keywords = append(keywords, base.KeywordInfo{
			Label:         def.Name,
			Category:      category,
			Detail:        symbol.Detail,
			Documentation: symbol.Documentation,
		})
	}
	return base.Metadata{
		Language: base.LanguageTQL,
		Version:  "tql-fx-definitions-v1",
		Keywords: keywords,
		Symbols:  symbols,
	}
}

func (svc *Service) Metadata() base.Metadata {
	return svc.metadata
}

func insertText(label string) string {
	if strings.HasSuffix(label, "()") {
		return label
	}
	return label + "()"
}

func hoverContents(symbol base.SymbolInfo) string {
	contents := symbol.Label + "\n\n" + symbol.Documentation + "\n\nCategory: " + symbol.Category
	if symbol.StatementKind != "" {
		contents += "\n\nStatement: " + symbol.StatementKind
	}
	return contents
}

func tqlSignature(label string) *base.SignatureInfo {
	name := strings.TrimSuffix(label, "()")
	return &base.SignatureInfo{
		Label:         name + "(...)",
		Documentation: "TQL function",
		Parameters:    []base.ParameterInfo{{Label: "args", Documentation: "Function arguments"}},
	}
}

func statementKindString(kind coretql.StatementKind) string {
	switch kind {
	case coretql.StatementSource:
		return "source"
	case coretql.StatementMap:
		return "map"
	case coretql.StatementSink:
		return "sink"
	case coretql.StatementSourceOrMap:
		return "source_or_map"
	case coretql.StatementSourceOrSink:
		return "source_or_sink"
	default:
		return "unknown"
	}
}

func (svc *Service) Diagnostics(_ context.Context, doc base.Document) ([]base.Diagnostic, error) {
	script, err := coretql.ParseScript(doc.Text, nil)
	if err != nil {
		return []base.Diagnostic{diagnosticFromError(err)}, nil
	}
	if err := coretql.ValidateScriptStructure(script); err != nil {
		return []base.Diagnostic{diagnosticFromError(err)}, nil
	}
	return nil, nil
}

func (svc *Service) Completion(_ context.Context, _ base.Document, _ base.Position) ([]base.CompletionItem, error) {
	items := make([]base.CompletionItem, len(svc.items))
	copy(items, svc.items)
	return items, nil
}

func (svc *Service) Hover(_ context.Context, doc base.Document, pos base.Position) (*base.Hover, error) {
	word, rng := wordAtPosition(doc.Text, pos)
	if word == "" {
		return nil, nil
	}
	contents, ok := svc.hovers[strings.ToUpper(word)]
	if !ok {
		return nil, nil
	}
	return &base.Hover{Range: rng, Contents: contents}, nil
}

func (svc *Service) SignatureHelp(_ context.Context, doc base.Document, pos base.Position) (*base.SignatureHelp, error) {
	callee, activeParameter, ok := callExpressionAtPosition(doc.Text, pos)
	if !ok {
		return nil, nil
	}
	callee = strings.ToUpper(callee)
	for _, symbol := range svc.metadata.Symbols {
		if strings.ToUpper(strings.TrimSuffix(symbol.Label, "()")) == callee && symbol.Signature != nil {
			return signatureHelp(symbol.Signature, activeParameter), nil
		}
	}
	return nil, nil
}

func diagnosticFromError(err error) base.Diagnostic {
	var scriptErr *coretql.ScriptError
	if errors.As(err, &scriptErr) {
		return base.Diagnostic{
			Range:    normalizeRange(rangeFromSpan(scriptErr.Span)),
			Severity: base.SeverityError,
			Code:     scriptErr.Kind,
			Source:   "tql",
			Message:  scriptErr.Message,
		}
	}
	var parseErr *expression.ParseError
	if errors.As(err, &parseErr) {
		message := parseErr.Message
		if parseErr.Near != "" {
			message += " near " + parseErr.Near
		}
		return base.Diagnostic{
			Range:    normalizeRange(rangeFromSpan(parseErr.Span)),
			Severity: base.SeverityError,
			Code:     parseErr.Kind,
			Source:   "tql",
			Message:  message,
		}
	}
	return base.Diagnostic{
		Range:    normalizeRange(base.Range{}),
		Severity: base.SeverityError,
		Source:   "tql",
		Message:  err.Error(),
	}
}

func rangeFromSpan(span expression.SourceSpan) base.Range {
	return base.Range{
		Start: base.Position{Line: span.Start.Line, Column: span.Start.Column},
		End:   base.Position{Line: span.End.Line, Column: span.End.Column},
	}
}

func normalizeRange(rng base.Range) base.Range {
	if rng.Start.Line <= 0 {
		rng.Start.Line = 1
	}
	if rng.Start.Column <= 0 {
		rng.Start.Column = 1
	}
	if rng.End.Line <= 0 {
		rng.End.Line = rng.Start.Line
	}
	if rng.End.Column <= 0 {
		rng.End.Column = rng.Start.Column + 1
	}
	if rng.End.Line < rng.Start.Line || (rng.End.Line == rng.Start.Line && rng.End.Column <= rng.Start.Column) {
		rng.End.Line = rng.Start.Line
		rng.End.Column = rng.Start.Column + 1
	}
	return rng
}

func wordAtPosition(text string, pos base.Position) (string, base.Range) {
	lines := strings.Split(text, "\n")
	if pos.Line <= 0 || pos.Line > len(lines) {
		return "", base.Range{}
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
	start := idx
	for start > 0 && isWordRune(line[start-1]) {
		start--
	}
	end := idx
	for end < len(line) && isWordRune(line[end]) {
		end++
	}
	if start == end {
		return "", base.Range{}
	}
	return string(line[start:end]), base.Range{
		Start: base.Position{Line: pos.Line, Column: start + 1},
		End:   base.Position{Line: pos.Line, Column: end + 1},
	}
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
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
	for idx >= 0 && unicode.IsSpace(rune(prefix[idx])) {
		idx--
	}
	end := idx + 1
	for idx >= 0 {
		r := rune(prefix[idx])
		if !(r == '_' || r == '$' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			break
		}
		idx--
	}
	return prefix[idx+1 : end]
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
