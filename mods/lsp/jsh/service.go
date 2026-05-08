package jsh

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
	base "github.com/machbase/neo-server/v8/mods/lsp"
)

type Service struct {
	items  []base.CompletionItem
	hovers map[string]string
}

type symbolInfo struct {
	Name          string
	Kind          base.CompletionItemKind
	Detail        string
	Documentation string
}

var restrictedSyntaxPattern = regexp.MustCompile(`\b(await|import)\b`)

func NewService() *Service {
	symbols := jshSymbols()
	items := make([]base.CompletionItem, 0, len(symbols))
	hovers := make(map[string]string, len(symbols))
	for _, symbol := range symbols {
		items = append(items, base.CompletionItem{
			Label:         symbol.Name,
			Kind:          symbol.Kind,
			Detail:        symbol.Detail,
			Documentation: symbol.Documentation,
			InsertText:    symbol.Name,
		})
		hovers[symbol.Name] = symbol.Name + "\n\n" + symbol.Documentation
	}
	return &Service{items: items, hovers: hovers}
}

func jshSymbols() []symbolInfo {
	return []symbolInfo{
		{Name: "require", Kind: base.CompletionFunction, Detail: "JSH module loader", Documentation: "Loads JSH JavaScript modules and native modules."},
		{Name: "console", Kind: base.CompletionVariable, Detail: "JSH console", Documentation: "JSH console object with log, print, println, printf, debug, info, warn, and error methods."},
		{Name: "process", Kind: base.CompletionVariable, Detail: "JSH process", Documentation: "JSH process object provided by the runtime."},
		{Name: "Buffer", Kind: base.CompletionClass, Detail: "buffer", Documentation: "Buffer is available implicitly in the JSH goja runtime."},
		{Name: "URL", Kind: base.CompletionClass, Detail: "url", Documentation: "URL is available implicitly in the JSH goja runtime."},
		{Name: "@jsh/process", Kind: base.CompletionModule, Detail: "native module", Documentation: "JSH native process module."},
		{Name: "@jsh/fs", Kind: base.CompletionModule, Detail: "native module", Documentation: "JSH native filesystem module."},
		{Name: "@jsh/machcli", Kind: base.CompletionModule, Detail: "native module", Documentation: "JSH native Machbase client module."},
		{Name: "@jsh/session", Kind: base.CompletionModule, Detail: "native module", Documentation: "JSH native session module."},
		{Name: "@jsh/pretty", Kind: base.CompletionModule, Detail: "native module", Documentation: "JSH native pretty formatting module."},
		{Name: "machcli", Kind: base.CompletionModule, Detail: "module", Documentation: "JSH Machbase client module wrapper."},
		{Name: "pretty", Kind: base.CompletionModule, Detail: "module", Documentation: "JSH pretty formatting module wrapper."},
	}
}

func (svc *Service) Diagnostics(_ context.Context, doc base.Document) ([]base.Diagnostic, error) {
	diagnostics := restrictedSyntaxDiagnostics(doc.Text)
	_, err := goja.Compile(doc.URI, doc.Text, false)
	if err != nil {
		diagnostics = append(diagnostics, diagnosticFromCompileError(err))
	}
	return diagnostics, nil
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
	contents, ok := svc.hovers[word]
	if !ok {
		return nil, nil
	}
	return &base.Hover{Range: rng, Contents: contents}, nil
}

func restrictedSyntaxDiagnostics(text string) []base.Diagnostic {
	lines := strings.Split(text, "\n")
	diagnostics := make([]base.Diagnostic, 0)
	for lineIdx, line := range lines {
		matches := restrictedSyntaxPattern.FindAllStringIndex(line, -1)
		for _, match := range matches {
			keyword := line[match[0]:match[1]]
			diagnostics = append(diagnostics, base.Diagnostic{
				Range: base.Range{
					Start: base.Position{Line: lineIdx + 1, Column: match[0] + 1},
					End:   base.Position{Line: lineIdx + 1, Column: match[1] + 1},
				},
				Severity: base.SeverityError,
				Code:     "unsupported_" + keyword,
				Source:   "jsh",
				Message:  keyword + " is not supported in the JSH goja runtime",
			})
		}
	}
	return diagnostics
}

func diagnosticFromCompileError(err error) base.Diagnostic {
	var errorList parser.ErrorList
	if errors.As(err, &errorList) && len(errorList) > 0 {
		return diagnosticFromParserError(errorList[0])
	}
	var parserErr *parser.Error
	if errors.As(err, &parserErr) {
		return diagnosticFromParserError(parserErr)
	}
	var compilerErr *goja.CompilerSyntaxError
	if errors.As(err, &compilerErr) {
		if compilerErr.File == nil {
			return base.Diagnostic{
				Range:    normalizeRange(1, 1, 2),
				Severity: base.SeverityError,
				Code:     "syntax_error",
				Source:   "jsh",
				Message:  compilerErr.Message,
			}
		}
		position := compilerErr.File.Position(compilerErr.Offset)
		return base.Diagnostic{
			Range:    normalizeRange(position.Line, position.Column, position.Column+1),
			Severity: base.SeverityError,
			Code:     "syntax_error",
			Source:   "jsh",
			Message:  compilerErr.Message,
		}
	}
	return base.Diagnostic{
		Range:    normalizeRange(1, 1, 2),
		Severity: base.SeverityError,
		Code:     "syntax_error",
		Source:   "jsh",
		Message:  err.Error(),
	}
}

func diagnosticFromParserError(parserErr *parser.Error) base.Diagnostic {
	return base.Diagnostic{
		Range:    normalizeRange(parserErr.Position.Line, parserErr.Position.Column, parserErr.Position.Column+1),
		Severity: base.SeverityError,
		Code:     "syntax_error",
		Source:   "jsh",
		Message:  parserErr.Message,
	}
}

func normalizeRange(line int, startColumn int, endColumn int) base.Range {
	if line <= 0 {
		line = 1
	}
	if startColumn <= 0 {
		startColumn = 1
	}
	if endColumn <= startColumn {
		endColumn = startColumn + 1
	}
	return base.Range{
		Start: base.Position{Line: line, Column: startColumn},
		End:   base.Position{Line: line, Column: endColumn},
	}
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
	return r == '_' || r == '$' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9')
}
