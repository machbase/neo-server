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
	symbols  map[string]base.SymbolInfo
}

var tqlLanguageSymbols = []base.SymbolInfo{
	{
		Label:         "true",
		Kind:          base.CompletionValue,
		Category:      "primitive",
		Detail:        "boolean",
		Documentation: "Boolean literal.",
		InsertText:    "true",
	},
	{
		Label:         "false",
		Kind:          base.CompletionValue,
		Category:      "primitive",
		Detail:        "boolean",
		Documentation: "Boolean literal.",
		InsertText:    "false",
	},
	{
		Label:         "NULL",
		Kind:          base.CompletionValue,
		Category:      "primitive",
		Detail:        "null",
		Documentation: "Null literal used in comparisons and default expressions.",
		InsertText:    "NULL",
	},
	{
		Label:         "nil",
		Kind:          base.CompletionValue,
		Category:      "primitive",
		Detail:        "null",
		Documentation: "Null literal alias.",
		InsertText:    "nil",
	},
	{
		Label:         "PI",
		Kind:          base.CompletionValue,
		Category:      "primitive",
		Detail:        "number",
		Documentation: "Numeric constant for pi.",
		InsertText:    "PI",
	},
	{
		Label:         "in",
		Kind:          base.CompletionKeyword,
		Category:      "operator",
		Detail:        "relational",
		Documentation: "Returns true when the left operand is contained in the argument list.",
		InsertText:    "in",
	},
	{
		Label:         "??",
		Kind:          base.CompletionKeyword,
		Category:      "operator",
		Detail:        "nil coalescing",
		Documentation: "Returns the left operand when it is defined, otherwise returns the right operand.",
		InsertText:    "??",
	},
	{
		Label:         "?",
		Kind:          base.CompletionKeyword,
		Category:      "operator",
		Detail:        "ternary",
		Documentation: "Starts a conditional expression in the form condition ? trueValue : falseValue.",
		InsertText:    "?",
	},
	{
		Label:         ":",
		Kind:          base.CompletionKeyword,
		Category:      "operator",
		Detail:        "ternary",
		Documentation: "Separates true and false branches in a ternary expression.",
		InsertText:    ":",
	},
	{
		Label:         "log-level",
		Kind:          base.CompletionKeyword,
		Category:      "pragma",
		Detail:        "TRACE | DEBUG | INFO | WARN | ERROR",
		Documentation: "Pragma that sets the TQL execution log level.",
		InsertText:    "//+ log-level=ERROR",
	},
	{
		Label:         "sql-thread-lock",
		Kind:          base.CompletionKeyword,
		Category:      "pragma",
		Detail:        "SQL thread lock",
		Documentation: "Pragma that runs SQL() on a dedicated native thread for the script execution.",
		InsertText:    "//+ sql-thread-lock",
	},
}

func NewService() *Service {
	metadata := BuildMetadata()
	items := make([]base.CompletionItem, 0, len(metadata.Symbols))
	hovers := make(map[string]string, len(metadata.Symbols))
	symbols := make(map[string]base.SymbolInfo, len(metadata.Symbols))
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
		hovers[symbol.Label] = hoverContents(symbol)
		symbols[symbol.Label] = symbol
		upperLabel := strings.ToUpper(symbol.Label)
		if _, exists := hovers[upperLabel]; !exists {
			hovers[upperLabel] = hoverContents(symbol)
		}
		if _, exists := symbols[upperLabel]; !exists {
			symbols[upperLabel] = symbol
		}
	}
	return &Service{metadata: metadata, items: items, hovers: hovers, symbols: symbols}
}

func BuildMetadata() base.Metadata {
	symbols := make([]base.SymbolInfo, 0, len(coretql.FxDefinitions)+len(tqlLanguageSymbols))
	keywords := make([]base.KeywordInfo, 0, len(coretql.FxDefinitions)+len(tqlLanguageSymbols))
	category := "general"
	for _, def := range coretql.FxDefinitions {
		if strings.HasPrefix(def.Name, "//") {
			category = strings.TrimSpace(strings.TrimPrefix(def.Name, "//"))
			continue
		}
		if doc, ok := generatedTqlDocs[def.Name]; ok && doc.Draft {
			continue
		}
		documentation := tqlFunctionDocumentation(def.Name, category)
		signature := tqlSignature(def.Name)
		if doc, ok := generatedTqlDocs[def.Name]; ok && tqlDocHasContent(doc) {
			documentation = doc.Description
			if docSignature := signatureFromDoc(doc); docSignature != nil {
				signature = docSignature
			}
		}
		symbol := base.SymbolInfo{
			Label:         def.Name,
			Kind:          base.CompletionFunction,
			Category:      category,
			Detail:        category,
			Documentation: documentation,
			InsertText:    insertText(def.Name),
		}
		if kind, ok := tqlStatementKind(def.Name); ok {
			symbol.StatementKind = statementKindString(kind)
		}
		symbol.Signature = signature
		symbols = append(symbols, symbol)
		keywords = append(keywords, base.KeywordInfo{
			Label:         def.Name,
			Category:      category,
			Detail:        symbol.Detail,
			Documentation: symbol.Documentation,
		})
	}
	for _, symbol := range tqlLanguageSymbols {
		symbols = append(symbols, symbol)
		keywords = append(keywords, base.KeywordInfo{
			Label:         symbol.Label,
			Category:      symbol.Category,
			Detail:        symbol.Detail,
			Documentation: symbol.Documentation,
		})
	}
	return base.Metadata{
		Language: base.LanguageTQL,
		Version:  "tql-fx-definitions-v2",
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
	return hoverContentsForRole(symbol, "")
}

func hoverContentsForRole(symbol base.SymbolInfo, role string) string {
	if doc, ok := generatedTqlDocs[symbol.Label]; ok && tqlDocHasContent(doc) {
		if variant, ok := tqlDocVariantForRole(doc, role); ok && tqlDocVariantHasContent(variant) {
			return variant.Markdown
		}
		return doc.Markdown
	}
	contents := symbol.Label + "\n\n" + symbol.Documentation + "\n\nCategory: " + symbol.Category
	if symbol.StatementKind != "" {
		contents += "\n\nStatement: " + symbol.StatementKind
	}
	return contents
}

func tqlDocHasContent(doc tqlDocInfo) bool {
	description := strings.TrimSpace(doc.Description)
	return description != "" && !strings.EqualFold(description, "TODO")
}

func tqlDocVariantHasContent(doc tqlDocVariant) bool {
	description := strings.TrimSpace(doc.Description)
	return description != "" && !strings.EqualFold(description, "TODO")
}

func tqlDocVariantForRole(doc tqlDocInfo, role string) (tqlDocVariant, bool) {
	if role == "" || len(doc.Roles) == 0 {
		return tqlDocVariant{}, false
	}
	variant, ok := doc.Roles[role]
	return variant, ok
}

func signatureFromDoc(doc tqlDocInfo) *base.SignatureInfo {
	return signatureFromDocVariant(doc.Description, doc.Signatures, doc.Slots)
}

func signatureFromDocForRole(doc tqlDocInfo, role string) *base.SignatureInfo {
	if variant, ok := tqlDocVariantForRole(doc, role); ok && tqlDocVariantHasContent(variant) {
		return signatureFromDocVariant(variant.Description, variant.Signatures, variant.Slots)
	}
	return signatureFromDoc(doc)
}

func signatureFromDocVariant(description string, signatures []tqlDocSignature, slots []tqlDocSlot) *base.SignatureInfo {
	if len(signatures) == 0 {
		return nil
	}
	signature := signatures[0]
	info := &base.SignatureInfo{Label: signature.Label, Documentation: description}
	for _, label := range signature.Parameters {
		info.Parameters = append(info.Parameters, base.ParameterInfo{Label: label, Documentation: slotDocumentation(slots, label)})
	}
	if len(info.Parameters) == 0 {
		for _, slot := range slots {
			info.Parameters = append(info.Parameters, base.ParameterInfo{Label: slot.Name, Documentation: slotDocumentation(slots, slot.Name)})
		}
	}
	return info
}

func docSlotDocumentation(doc tqlDocInfo, name string) string {
	return slotDocumentation(doc.Slots, name)
}

func slotDocumentation(slots []tqlDocSlot, name string) string {
	trimmedName := strings.TrimSuffix(name, "...")
	for _, slot := range slots {
		if slot.Name != trimmedName {
			continue
		}
		parts := make([]string, 0, 3)
		if slot.Accepts != "" {
			parts = append(parts, "accepts "+slot.Accepts)
		}
		if slot.Required {
			parts = append(parts, "required")
		}
		if slot.Repeat {
			parts = append(parts, "repeatable")
		}
		if len(slot.Suggestions) > 0 {
			parts = append(parts, "suggestions: "+strings.Join(slot.Suggestions, ", "))
		}
		return strings.Join(parts, "; ")
	}
	return ""
}

func tqlFunctionDocumentation(label string, category string) string {
	name := strings.TrimSuffix(label, "()")
	if kind, ok := tqlStatementKind(name); ok {
		switch kind {
		case coretql.StatementSource:
			return "Source statement that starts a TQL flow by producing records."
		case coretql.StatementMap:
			return "Map statement that transforms records between source and sink statements."
		case coretql.StatementSink:
			return "Sink statement that terminates a TQL flow by encoding or writing records."
		case coretql.StatementSourceOrMap:
			return "Statement that can produce records as a source or transform records as a map."
		case coretql.StatementSourceOrSink:
			return "Statement that can read records as a source or encode records as a sink."
		}
	}
	switch category {
	case "context":
		return "Argument function for request, record, and execution context values."
	case "math":
		return "Argument function for numeric expressions."
	case "arrays", "arrays and dictionaries":
		return "Argument function for list and dictionary values."
	case "map time":
		return "Argument function for time and time zone values."
	case "database source":
		return "Argument function for database source statements."
	case "database sink":
		return "Argument function for database sink statements."
	case "generator":
		return "Argument function for generated records used by FAKE()."
	case "conversion":
		return "Argument function for converting strings, numbers, booleans, and time values."
	case "encoder":
		return "Argument function for output encoders and chart statements."
	case "maps stat", "map monad", "maps.group":
		return "Argument function for record transformation and aggregation."
	default:
		return "TQL argument function."
	}
}

func tqlStatementKind(label string) (coretql.StatementKind, bool) {
	name := strings.TrimSuffix(label, "()")
	if name == "" || name != strings.ToUpper(name) {
		return coretql.StatementUnknown, false
	}
	return coretql.StatementKindByFunctionName(name)
}

func tqlSignature(label string) *base.SignatureInfo {
	name := strings.TrimSuffix(label, "()")
	switch name {
	case "SQL":
		return &base.SignatureInfo{Label: "SQL(query, args...)", Documentation: "Source statement that runs a SQL query and produces records.", Parameters: []base.ParameterInfo{{Label: "query", Documentation: "SQL text"}, {Label: "args", Documentation: "Optional query parameters"}}}
	case "SQL_SELECT":
		return &base.SignatureInfo{Label: "SQL_SELECT(fields..., options...)", Documentation: "Source statement that selects columns from a table or tag query.", Parameters: []base.ParameterInfo{{Label: "fields", Documentation: "Column names"}, {Label: "options", Documentation: "from(), limit(), between(), and related options"}}}
	case "FAKE":
		return &base.SignatureInfo{Label: "FAKE(generator)", Documentation: "Source statement that generates artificial records for tests and examples.", Parameters: []base.ParameterInfo{{Label: "generator", Documentation: "Generator such as linspace(), oscillator(), json(), or csv()"}}}
	case "MAPVALUE":
		return &base.SignatureInfo{Label: "MAPVALUE(index, expression, options...)", Documentation: "Map statement that changes or appends a value field.", Parameters: []base.ParameterInfo{{Label: "index", Documentation: "Value field index"}, {Label: "expression", Documentation: "Expression to evaluate for the field"}, {Label: "options", Documentation: "Optional mapping options"}}}
	case "CSV":
		return &base.SignatureInfo{Label: "CSV(options...)", Documentation: "Source or sink statement for reading CSV input or encoding records as CSV.", Parameters: []base.ParameterInfo{{Label: "options", Documentation: "CSV source or encoder options"}}}
	case "JSON":
		return &base.SignatureInfo{Label: "JSON(options...)", Documentation: "Sink statement that encodes records as JSON.", Parameters: []base.ParameterInfo{{Label: "options", Documentation: "JSON encoder options such as transpose(true)"}}}
	case "param":
		return &base.SignatureInfo{Label: "param(name)", Documentation: "Returns a value from HTTP query parameters.", Parameters: []base.ParameterInfo{{Label: "name", Documentation: "Query parameter name"}}}
	case "value":
		return &base.SignatureInfo{Label: "value(index)", Documentation: "Returns a field from the current record value.", Parameters: []base.ParameterInfo{{Label: "index", Documentation: "Value field index"}}}
	case "list":
		return &base.SignatureInfo{Label: "list(values...)", Documentation: "Creates a list value.", Parameters: []base.ParameterInfo{{Label: "values", Documentation: "List elements"}}}
	case "dict":
		return &base.SignatureInfo{Label: "dict(keyValues...)", Documentation: "Creates a dictionary value from key and value pairs.", Parameters: []base.ParameterInfo{{Label: "keyValues", Documentation: "Alternating keys and values"}}}
	}
	return &base.SignatureInfo{
		Label:         name + "(...)",
		Documentation: tqlFunctionDocumentation(name, ""),
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

func (svc *Service) Completion(_ context.Context, doc base.Document, pos base.Position) ([]base.CompletionItem, error) {
	items := make([]base.CompletionItem, len(svc.items))
	copy(items, svc.items)
	if prioritized := svc.completionItemsForArgument(doc.Text, pos); len(prioritized) > 0 {
		return mergeCompletionItems(prioritized, items), nil
	}
	return items, nil
}

func (svc *Service) completionItemsForArgument(text string, pos base.Position) []base.CompletionItem {
	call, ok := callContextAtPosition(text, pos)
	if !ok {
		return nil
	}
	doc, ok := generatedTqlDocs[call.callee]
	if !ok || len(doc.Slots) == 0 {
		return nil
	}
	variant, variantOK := tqlDocVariantForRole(doc, statementRoleAtPosition(text, pos))
	suggestions := suggestionsForCallSlot(doc, variant, variantOK, call)
	items := make([]base.CompletionItem, 0, len(suggestions))
	for _, suggestion := range suggestions {
		if item, ok := svc.completionItemForSuggestion(suggestion); ok {
			items = append(items, item)
		}
	}
	return items
}

func suggestionsForCallSlot(doc tqlDocInfo, variant tqlDocVariant, hasVariant bool, call callContext) []string {
	slots := doc.Slots
	if hasVariant {
		slots = variant.Slots
	}
	slotNames := activeSlotNames(doc, variant, hasVariant, call)
	seen := make(map[string]bool)
	suggestions := make([]string, 0)
	for _, slotName := range slotNames {
		for _, slot := range slots {
			if slot.Name != slotName {
				continue
			}
			for _, suggestion := range slot.Suggestions {
				if !seen[suggestion] {
					seen[suggestion] = true
					suggestions = append(suggestions, suggestion)
				}
			}
		}
	}
	return suggestions
}

func activeSlotNames(doc tqlDocInfo, variant tqlDocVariant, hasVariant bool, call callContext) []string {
	slots := doc.Slots
	if hasVariant {
		slots = variant.Slots
	}
	switch doc.Label {
	case "SQL":
		if call.activeParameter == 0 {
			return []string{"bridge", "sqlText"}
		}
		if call.activeParameter == 1 && len(call.arguments) > 0 && isHelperCall(call.arguments[0], "bridge") {
			return []string{"sqlText"}
		}
		return []string{"params"}
	case "MAPVALUE":
		switch call.activeParameter {
		case 0:
			return []string{"index"}
		case 1:
			return []string{"expression"}
		default:
			return []string{"options"}
		}
	case "CSV":
		if call.activeParameter == 0 {
			return []string{"input", "options"}
		}
		return []string{"options"}
	}
	if call.activeParameter < len(slots) {
		return []string{slots[call.activeParameter].Name}
	}
	for _, slot := range slots {
		if slot.Repeat {
			return []string{slot.Name}
		}
	}
	return nil
}

func statementRoleAtPosition(text string, pos base.Position) string {
	type statementLine struct {
		line int
		name string
	}
	lines := strings.Split(text, "\n")
	statements := make([]statementLine, 0)
	for lineIndex, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		nameEnd := 0
		for nameEnd < len(trimmed) {
			r := rune(trimmed[nameEnd])
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
				break
			}
			nameEnd++
		}
		if nameEnd == 0 || nameEnd >= len(trimmed) || trimmed[nameEnd] != '(' {
			continue
		}
		statements = append(statements, statementLine{line: lineIndex, name: strings.ToUpper(trimmed[:nameEnd])})
	}
	if len(statements) == 0 {
		return ""
	}
	line := pos.Line - 1
	statementIndex := -1
	for idx, statement := range statements {
		if statement.line <= line {
			statementIndex = idx
		}
	}
	if statementIndex < 0 {
		return ""
	}
	if statementIndex == 0 {
		return "source"
	}
	if statementIndex == len(statements)-1 {
		return "sink"
	}
	return "map"
}

func isHelperCall(value string, name string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, name+"(") || strings.HasPrefix(strings.ToUpper(trimmed), strings.ToUpper(name)+"(")
}

func (svc *Service) completionItemForSuggestion(suggestion string) (base.CompletionItem, bool) {
	suggestion = strings.TrimSpace(suggestion)
	if suggestion == "" {
		return base.CompletionItem{}, false
	}
	switch suggestion {
	case "backtick-sql-string":
		return base.CompletionItem{Label: "SQL string", Kind: base.CompletionSnippet, Detail: "SQL text", Documentation: "Backtick SQL text literal.", InsertText: "`SELECT * FROM ${1:table}`"}, true
	}
	if strings.Contains(suggestion, " ") {
		return base.CompletionItem{}, false
	}
	for _, item := range svc.items {
		if item.Label == suggestion {
			return item, true
		}
	}
	if isNumericSuggestion(suggestion) {
		return base.CompletionItem{Label: suggestion, Kind: base.CompletionValue, Detail: "literal", Documentation: "Literal value suggestion.", InsertText: suggestion}, true
	}
	return base.CompletionItem{}, false
}

func isNumericSuggestion(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func mergeCompletionItems(prioritized []base.CompletionItem, fallback []base.CompletionItem) []base.CompletionItem {
	seen := make(map[string]bool, len(prioritized))
	items := make([]base.CompletionItem, 0, len(prioritized)+len(fallback))
	for _, item := range prioritized {
		if seen[item.Label] {
			continue
		}
		seen[item.Label] = true
		items = append(items, item)
	}
	for _, item := range fallback {
		if seen[item.Label] {
			continue
		}
		seen[item.Label] = true
		items = append(items, item)
	}
	return items
}

func (svc *Service) Hover(_ context.Context, doc base.Document, pos base.Position) (*base.Hover, error) {
	word, rng := wordAtPosition(doc.Text, pos)
	if word != "" {
		symbol, ok := svc.symbols[word]
		if !ok {
			symbol, ok = svc.symbols[strings.ToUpper(word)]
		}
		if ok {
			contents := hoverContentsForRole(symbol, statementRoleAtPosition(doc.Text, pos))
			return &base.Hover{Range: rng, Contents: contents}, nil
		}
	}
	symbol, rng := symbolAtPosition(doc.Text, pos)
	if symbol != "" {
		if contents, ok := svc.hovers[symbol]; ok {
			return &base.Hover{Range: rng, Contents: contents}, nil
		}
	}
	return nil, nil
}

func (svc *Service) SignatureHelp(_ context.Context, doc base.Document, pos base.Position) (*base.SignatureHelp, error) {
	callee, activeParameter, ok := callExpressionAtPosition(doc.Text, pos)
	if !ok {
		return nil, nil
	}
	callee = strings.ToUpper(callee)
	if docInfo, ok := generatedTqlDocs[callee]; ok && !docInfo.Draft {
		if signature := signatureFromDocForRole(docInfo, statementRoleAtPosition(doc.Text, pos)); signature != nil {
			return signatureHelp(signature, activeParameter), nil
		}
	}
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
	return r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func symbolAtPosition(text string, pos base.Position) (string, base.Range) {
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
	if idx >= len(line) {
		idx = len(line) - 1
	}
	if idx < 0 || !isOperatorRune(line[idx]) {
		return "", base.Range{}
	}
	start := idx
	for start > 0 && isOperatorRune(line[start-1]) {
		start--
	}
	end := idx + 1
	for end < len(line) && isOperatorRune(line[end]) {
		end++
	}
	return string(line[start:end]), base.Range{Start: base.Position{Line: pos.Line, Column: start + 1}, End: base.Position{Line: pos.Line, Column: end + 1}}
}

func isOperatorRune(r rune) bool {
	return strings.ContainsRune("?=:!<>|&+-*/%^~", r)
}

type callFrame struct {
	open   int
	commas int
}

type callContext struct {
	callee          string
	activeParameter int
	arguments       []string
}

func callContextAtPosition(text string, pos base.Position) (callContext, bool) {
	prefix, ok := textBeforePosition(text, pos)
	if !ok {
		return callContext{}, false
	}
	frame, ok := innermostCallFrame(prefix)
	if !ok {
		return callContext{}, false
	}
	callee := calleeBeforeOpen(prefix, frame.open)
	if callee == "" {
		return callContext{}, false
	}
	return callContext{callee: strings.ToUpper(callee), activeParameter: frame.commas, arguments: splitTopLevelArguments(prefix[frame.open+1:])}, true
}

func callExpressionAtPosition(text string, pos base.Position) (string, int, bool) {
	prefix, ok := textBeforePosition(text, pos)
	if !ok {
		return "", 0, false
	}
	frame, ok := innermostCallFrame(prefix)
	if !ok {
		return "", 0, false
	}
	callee := calleeBeforeOpen(prefix, frame.open)
	if callee == "" {
		return "", 0, false
	}
	return callee, frame.commas, true
}

func innermostCallFrame(prefix string) (callFrame, bool) {
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
		return callFrame{}, false
	}
	return frames[len(frames)-1], true
}

func splitTopLevelArguments(value string) []string {
	args := make([]string, 0)
	start := 0
	depth := 0
	quote := rune(0)
	escaped := false
	for idx, r := range value {
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
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(value[start:idx]))
				start = idx + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(value[start:]))
	return args
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
