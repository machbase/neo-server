package jsh

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
	jshlib "github.com/machbase/neo-server/v8/jsh/lib"
	base "github.com/machbase/neo-server/v8/mods/lsp"
)

type Service struct {
	metadata base.Metadata
	items    []base.CompletionItem
	hovers   map[string]string
	modules  map[string]base.ModuleInfo
}

type symbolInfo struct {
	Name          string
	Kind          base.CompletionItemKind
	Detail        string
	Documentation string
}

var restrictedSyntaxPattern = regexp.MustCompile(`\b(await|import)\b`)

func NewService() *Service {
	metadata := BuildMetadata()
	symbols := metadata.Symbols
	items := make([]base.CompletionItem, 0, len(symbols))
	hovers := make(map[string]string, len(symbols))
	modules := make(map[string]base.ModuleInfo, len(metadata.Modules))
	for _, module := range metadata.Modules {
		modules[module.ID] = module
	}
	for _, symbol := range symbols {
		items = append(items, base.CompletionItem{
			Label:         symbol.Label,
			Kind:          symbol.Kind,
			Detail:        symbol.Detail,
			Documentation: symbol.Documentation,
			InsertText:    symbol.InsertText,
		})
		hovers[symbol.Label] = symbol.Label + "\n\n" + symbol.Documentation
	}
	return &Service{metadata: metadata, items: items, hovers: hovers, modules: modules}
}

func jshSymbols() []symbolInfo {
	return []symbolInfo{
		{Name: "require", Kind: base.CompletionFunction, Detail: "JSH module loader", Documentation: "Loads JSH JavaScript modules and native modules."},
		{Name: "console", Kind: base.CompletionVariable, Detail: "JSH console", Documentation: "JSH console object with log, print, println, printf, debug, info, warn, and error methods."},
		{Name: "process", Kind: base.CompletionVariable, Detail: "JSH process", Documentation: "JSH process object provided by the runtime."},
		{Name: "Buffer", Kind: base.CompletionClass, Detail: "buffer", Documentation: "Buffer is available implicitly in the JSH goja runtime."},
		{Name: "URL", Kind: base.CompletionClass, Detail: "url", Documentation: "URL is available implicitly in the JSH goja runtime."},
	}
}

func BuildMetadata() base.Metadata {
	symbols := make([]base.SymbolInfo, 0)
	keywords := make([]base.KeywordInfo, 0)
	modules := make([]base.ModuleInfo, 0)
	for _, symbol := range jshSymbols() {
		info := base.SymbolInfo{
			Label:         symbol.Name,
			Kind:          symbol.Kind,
			Category:      "runtime",
			Detail:        symbol.Detail,
			Documentation: symbol.Documentation,
			InsertText:    symbol.Name,
		}
		symbols = append(symbols, info)
		keywords = append(keywords, base.KeywordInfo{Label: symbol.Name, Category: "runtime", Detail: symbol.Detail, Documentation: symbol.Documentation})
	}

	moduleIDs := make([]string, 0)
	moduleFiles := jshlib.UserModuleFiles()
	for path := range moduleFiles {
		moduleID := moduleIDFromFile(path)
		if moduleID == "" || strings.HasPrefix(moduleID, "@jsh/") {
			continue
		}
		moduleIDs = append(moduleIDs, moduleID)
	}
	sort.Strings(moduleIDs)

	seenModules := map[string]bool{}
	for _, moduleID := range moduleIDs {
		if seenModules[moduleID] {
			continue
		}
		seenModules[moduleID] = true
		source := string(moduleFiles[moduleID+".js"])
		if source == "" {
			source = string(moduleFiles[moduleID])
		}
		if source == "" {
			for path, content := range moduleFiles {
				if moduleIDFromFile(path) == moduleID {
					source = string(content)
					break
				}
			}
		}
		exports := extractModuleSymbols(moduleID, source)
		modules = append(modules, base.ModuleInfo{ID: moduleID, Detail: "JSH module", Documentation: "JSH " + moduleID + " module", Exports: exports})
		symbols = append(symbols, base.SymbolInfo{Label: moduleID, Kind: base.CompletionModule, Category: "module", Detail: "JSH module", Documentation: "JSH " + moduleID + " module", InsertText: moduleID})
	}

	return base.Metadata{Language: base.LanguageJSH, Version: "jsh-jsdoc-v1", Keywords: keywords, Symbols: symbols, Modules: modules}
}

func (svc *Service) Metadata() base.Metadata {
	return svc.metadata
}

func moduleIDFromFile(path string) string {
	if !strings.HasSuffix(path, ".js") {
		return ""
	}
	return strings.TrimSuffix(filepath.ToSlash(path), ".js")
}

func (svc *Service) Diagnostics(_ context.Context, doc base.Document) ([]base.Diagnostic, error) {
	diagnostics := restrictedSyntaxDiagnostics(doc.Text)
	_, err := goja.Compile(doc.URI, doc.Text, false)
	if err != nil {
		diagnostics = append(diagnostics, diagnosticFromCompileError(err))
	}
	return diagnostics, nil
}

func (svc *Service) Completion(_ context.Context, doc base.Document, pos base.Position) ([]base.CompletionItem, error) {
	if isRequireStringPosition(doc.Text, pos) {
		return svc.moduleCompletionItems(), nil
	}
	if moduleID, ok := moduleMemberContext(doc.Text, pos); ok {
		if module, exists := svc.modules[moduleID]; exists {
			return symbolCompletionItems(module.Exports), nil
		}
	}
	items := make([]base.CompletionItem, len(svc.items))
	copy(items, svc.items)
	return items, nil
}

func (svc *Service) Hover(_ context.Context, doc base.Document, pos base.Position) (*base.Hover, error) {
	if moduleID, member, rng, ok := moduleMemberAtPosition(doc.Text, pos); ok {
		if module, exists := svc.modules[moduleID]; exists {
			if symbol, found := findModuleSymbol(module.Exports, member); found {
				return &base.Hover{Range: rng, Contents: symbolHoverContents(module.ID, symbol)}, nil
			}
		}
	}
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

func (svc *Service) SignatureHelp(_ context.Context, doc base.Document, pos base.Position) (*base.SignatureHelp, error) {
	callee, activeParameter, ok := callExpressionAtPosition(doc.Text, pos)
	if !ok {
		return nil, nil
	}
	if callee == "require" {
		return signatureHelp(requireSignature(), activeParameter), nil
	}
	if moduleID, member, ok := moduleMemberCallee(doc.Text, callee); ok {
		if module, exists := svc.modules[moduleID]; exists {
			if symbol, found := findModuleSymbol(module.Exports, member); found && symbol.Signature != nil {
				return signatureHelp(symbol.Signature, activeParameter), nil
			}
		}
	}
	return nil, nil
}

func (svc *Service) moduleCompletionItems() []base.CompletionItem {
	items := make([]base.CompletionItem, 0, len(svc.metadata.Modules))
	for _, module := range svc.metadata.Modules {
		items = append(items, base.CompletionItem{Label: module.ID, Kind: base.CompletionModule, Detail: module.Detail, Documentation: module.Documentation, InsertText: module.ID})
	}
	return items
}

func symbolCompletionItems(symbols []base.SymbolInfo) []base.CompletionItem {
	items := make([]base.CompletionItem, 0, len(symbols))
	for _, symbol := range symbols {
		insertText := symbol.InsertText
		if insertText == "" {
			insertText = symbol.Label
		}
		items = append(items, base.CompletionItem{Label: symbol.Label, Kind: symbol.Kind, Detail: symbol.Detail, Documentation: symbol.Documentation, InsertText: insertText})
	}
	return items
}

func symbolHoverContents(moduleID string, symbol base.SymbolInfo) string {
	contents := moduleID + "." + symbol.Label
	if symbol.Documentation != "" {
		contents += "\n\n" + symbol.Documentation
	}
	return contents
}

func requireSignature() *base.SignatureInfo {
	return &base.SignatureInfo{
		Label:         "require(module)",
		Documentation: "Loads a JSH JavaScript module.",
		Parameters:    []base.ParameterInfo{{Label: "module", Documentation: "User-facing module id"}},
	}
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
