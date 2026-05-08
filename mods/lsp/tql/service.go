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
	functions []functionInfo
	items     []base.CompletionItem
	hovers    map[string]string
}

type functionInfo struct {
	Name     string
	Category string
}

func NewService() *Service {
	functions := tqlFunctions()
	items := make([]base.CompletionItem, 0, len(functions))
	hovers := make(map[string]string, len(functions))
	for _, fn := range functions {
		detail := fn.Category
		insertText := fn.Name
		if !strings.HasSuffix(insertText, "()") {
			insertText += "()"
		}
		doc := "TQL function"
		items = append(items, base.CompletionItem{
			Label:         fn.Name,
			Kind:          base.CompletionFunction,
			Detail:        detail,
			Documentation: doc,
			InsertText:    insertText,
		})
		hovers[strings.ToUpper(fn.Name)] = fn.Name + "\n\n" + doc + "\n\nCategory: " + fn.Category
	}
	return &Service{functions: functions, items: items, hovers: hovers}
}

func tqlFunctions() []functionInfo {
	ret := make([]functionInfo, 0, len(coretql.FxDefinitions))
	category := "general"
	for _, def := range coretql.FxDefinitions {
		if strings.HasPrefix(def.Name, "//") {
			category = strings.TrimSpace(strings.TrimPrefix(def.Name, "//"))
			continue
		}
		ret = append(ret, functionInfo{Name: def.Name, Category: category})
	}
	return ret
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
