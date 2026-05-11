package tql

import (
	"context"
	"strings"
	"testing"

	base "github.com/machbase/neo-server/v8/mods/lsp"
)

func TestDiagnosticsParseError(t *testing.T) {
	svc := NewService()
	diags, err := svc.Diagnostics(context.Background(), base.Document{
		URI:      "memory://test.tql",
		Language: base.LanguageTQL,
		Text:     "FAKE(json({[1]}))\nMAPVALUE(0, value(0))3\nCSV()",
	})
	if err != nil {
		t.Fatalf("Diagnostics returned error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Range.Start.Line != 2 {
		t.Fatalf("expected diagnostic on line 2, got %+v", diags[0].Range)
	}
	if diags[0].Severity != base.SeverityError {
		t.Fatalf("expected error severity, got %d", diags[0].Severity)
	}
}

func TestDiagnosticsStructureError(t *testing.T) {
	svc := NewService()
	diags, err := svc.Diagnostics(context.Background(), base.Document{
		URI:      "memory://test.tql",
		Language: base.LanguageTQL,
		Text:     "MAPVALUE(0, 1)\nCSV()",
	})
	if err != nil {
		t.Fatalf("Diagnostics returned error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "invalid_source" {
		t.Fatalf("expected invalid_source, got %q", diags[0].Code)
	}
	if diags[0].Range.Start.Line != 1 {
		t.Fatalf("expected diagnostic on line 1, got %+v", diags[0].Range)
	}
}

func TestCompletionIncludesTqlFunctions(t *testing.T) {
	svc := NewService()
	items, err := svc.Completion(context.Background(), base.Document{Language: base.LanguageTQL}, base.Position{Line: 1, Column: 1})
	if err != nil {
		t.Fatalf("Completion returned error: %v", err)
	}
	if !hasCompletion(items, "FAKE") {
		t.Fatalf("expected FAKE completion")
	}
	if !hasCompletion(items, "MAPVALUE") {
		t.Fatalf("expected MAPVALUE completion")
	}
	if !hasCompletion(items, "CSV") {
		t.Fatalf("expected CSV completion")
	}
}

func TestBuildMetadataIncludesTqlFunctions(t *testing.T) {
	metadata := BuildMetadata()
	if metadata.Language != base.LanguageTQL {
		t.Fatalf("expected tql metadata, got %q", metadata.Language)
	}
	if metadata.Version == "" {
		t.Fatal("expected metadata version")
	}
	if !hasKeyword(metadata.Keywords, "FAKE") {
		t.Fatal("expected FAKE keyword")
	}
	if !hasKeyword(metadata.Keywords, "MAPVALUE") {
		t.Fatal("expected MAPVALUE keyword")
	}
	if !hasSymbolStatementKind(metadata.Symbols, "FAKE", "source") {
		t.Fatal("expected FAKE source symbol")
	}
	if !hasSymbolStatementKind(metadata.Symbols, "MAPVALUE", "map") {
		t.Fatal("expected MAPVALUE map symbol")
	}
	if !hasSymbolStatementKind(metadata.Symbols, "CSV", "source_or_sink") {
		t.Fatal("expected CSV source_or_sink symbol")
	}
	if !hasSymbolStatementKind(metadata.Symbols, "CHART_SURFACE3D", "sink") {
		t.Fatal("expected CHART_SURFACE3D sink symbol")
	}
	if !hasSymbolSignature(metadata.Symbols, "MAPVALUE") {
		t.Fatal("expected MAPVALUE signature")
	}
	if !hasKeyword(metadata.Keywords, "NULL") {
		t.Fatal("expected NULL keyword")
	}
	if !hasKeyword(metadata.Keywords, "log-level") {
		t.Fatal("expected log-level pragma keyword")
	}
}

func TestMetadataReturnsTqlMetadata(t *testing.T) {
	svc := NewService()
	metadata := svc.Metadata()
	if metadata.Language != base.LanguageTQL {
		t.Fatalf("expected tql metadata, got %q", metadata.Language)
	}
	if len(metadata.Symbols) == 0 {
		t.Fatal("expected tql symbols")
	}
}

func TestHoverReturnsFunctionInfo(t *testing.T) {
	svc := NewService()
	hover, err := svc.Hover(context.Background(), base.Document{
		Language: base.LanguageTQL,
		Text:     "FAKE(json({[1]}))\nCSV()",
	}, base.Position{Line: 1, Column: 2})
	if err != nil {
		t.Fatalf("Hover returned error: %v", err)
	}
	if hover == nil {
		t.Fatal("expected hover")
	}
	if hover.Range.Start.Line != 1 || hover.Range.Start.Column != 1 {
		t.Fatalf("unexpected hover range: %+v", hover.Range)
	}
	if hover.Contents == "" {
		t.Fatal("expected hover contents")
	}
}

func TestSignatureHelpReturnsTqlFunctionInfo(t *testing.T) {
	svc := NewService()
	help, err := svc.SignatureHelp(context.Background(), base.Document{Language: base.LanguageTQL, Text: "MAPVALUE(0, value(0))"}, base.Position{Line: 1, Column: 12})
	if err != nil {
		t.Fatalf("SignatureHelp returned error: %v", err)
	}
	if help == nil || len(help.Signatures) == 0 {
		t.Fatalf("expected signature help, got %+v", help)
	}
	if help.Signatures[0].Label != "MAPVALUE(index, expression, options...)" {
		t.Fatalf("expected MAPVALUE signature, got %+v", help.Signatures[0])
	}
	if help.ActiveParameter != 1 {
		t.Fatalf("expected active parameter 1, got %d", help.ActiveParameter)
	}
}

func TestCompletionPrioritizesDocumentSlotSuggestions(t *testing.T) {
	svc := NewService()
	ctx := context.Background()
	cases := []struct {
		name     string
		code     string
		position base.Position
		labels   []string
	}{
		{
			name:     "sql first argument",
			code:     "SQL(",
			position: base.Position{Line: 1, Column: len("SQL(") + 1},
			labels:   []string{"bridge", "SQL string"},
		},
		{
			name:     "sql text after bridge",
			code:     "SQL(bridge('demo'), ",
			position: base.Position{Line: 1, Column: len("SQL(bridge('demo'), ") + 1},
			labels:   []string{"SQL string"},
		},
		{
			name:     "sql params after query",
			code:     "SQL(`select * from example`, ",
			position: base.Position{Line: 1, Column: len("SQL(`select * from example`, ") + 1},
			labels:   []string{"param", "tz", "sqlTimeformat", "ansiTimeformat"},
		},
		{
			name:     "mapvalue expression",
			code:     "MAPVALUE(0, ",
			position: base.Position{Line: 1, Column: len("MAPVALUE(0, ") + 1},
			labels:   []string{"value", "key", "param", "time"},
		},
		{
			name:     "csv source options",
			code:     "CSV(",
			position: base.Position{Line: 1, Column: len("CSV(") + 1},
			labels:   []string{"file", "payload", "field", "charset"},
		},
		{
			name:     "csv sink options",
			code:     "FAKE(linspace(1, 3, 3))\nCSV(",
			position: base.Position{Line: 2, Column: len("CSV(") + 1},
			labels:   []string{"nullValue", "cache", "tz", "sqlTimeformat", "ansiTimeformat"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := svc.Completion(ctx, base.Document{Language: base.LanguageTQL, Text: tc.code}, tc.position)
			if err != nil {
				t.Fatalf("Completion returned error: %v", err)
			}
			assertLeadingCompletionLabels(t, items, tc.labels)
		})
	}
}

func TestWebUIMessageForTqlContexts(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	completionCases := []webUICompletionCase{
		{
			name:     "generator source completion",
			code:     "FA",
			position: base.Position{Line: 1, Column: len("FA") + 1},
			label:    "FAKE",
			expect: expectedCompletionItem{
				kind:          base.CompletionFunction,
				detail:        "generator",
				documentation: "`FAKE()` produces artificial records from a generator helper. It is commonly used for examples, tests, synthetic wave data, and inline CSV or JSON data.",
				insertText:    "FAKE()",
			},
		},
		{
			name:     "encoder sink completion",
			code:     "CSV",
			position: base.Position{Line: 1, Column: len("CSV") + 1},
			label:    "CSV",
			expect: expectedCompletionItem{
				kind:          base.CompletionFunction,
				detail:        "encoder",
				documentation: "As a SRC function, `CSV()` reads CSV data from `file()`, `payload()`, or inline content and yields records. As a SINK function, `CSV()` encodes incoming records as CSV lines. Sink output is terminated by two consecutive newlines.",
				insertText:    "CSV()",
			},
		},
		{
			name:     "map statement completion",
			code:     "FAKE(linspace(1, 3, 3))\nMAP",
			position: base.Position{Line: 2, Column: len("MAP") + 1},
			label:    "MAPVALUE",
			expect: expectedCompletionItem{
				kind:          base.CompletionFunction,
				detail:        "map monad",
				documentation: "`MAPVALUE()` changes or appends a value field in the current record. The first argument selects the value index, and the second argument is evaluated as the new field value. Additional options can name the output field or adjust mapping behavior.",
				insertText:    "MAPVALUE()",
			},
		},
		{
			name:     "query parameter helper completion",
			code:     "param",
			position: base.Position{Line: 1, Column: len("param") + 1},
			label:    "param",
			expect: expectedCompletionItem{
				kind:          base.CompletionFunction,
				detail:        "context",
				documentation: "`param()` returns a requested query parameter when a TQL script is called via HTTP.",
				insertText:    "param()",
			},
		},
		{
			name:     "null literal completion",
			code:     "NU",
			position: base.Position{Line: 1, Column: len("NU") + 1},
			label:    "NULL",
			expect: expectedCompletionItem{
				kind:          base.CompletionValue,
				detail:        "null",
				documentation: "Null literal used in comparisons and default expressions.",
				insertText:    "NULL",
			},
		},
		{
			name:     "pragma completion",
			code:     "//+ log",
			position: base.Position{Line: 1, Column: len("//+ log") + 1},
			label:    "log-level",
			expect: expectedCompletionItem{
				kind:          base.CompletionKeyword,
				detail:        "TRACE | DEBUG | INFO | WARN | ERROR",
				documentation: "Pragma that sets the TQL execution log level.",
				insertText:    "//+ log-level=ERROR",
			},
		},
	}
	for _, tc := range completionCases {
		t.Run("completion/"+tc.name, func(t *testing.T) {
			items, err := svc.Completion(ctx, base.Document{Language: base.LanguageTQL, Text: tc.code}, tc.position)
			if err != nil {
				t.Fatalf("Completion returned error: %v", err)
			}
			item := findCompletion(items, tc.label)
			if item == nil {
				t.Fatalf("expected %q completion", tc.label)
			}
			assertCompletionItem(t, item, tc.expect)
		})
	}

	hoverCases := []webUIHoverCase{
		{
			name:     "generator source hover",
			code:     "FAKE(json({[1]}))\nCSV()",
			position: base.Position{Line: 1, Column: 2},
			expect:   "# FAKE\n\n## Kind\n\nstatement source",
		},
		{
			name:     "encoder source or sink hover",
			code:     "FAKE(json({[1]}))\nCSV()",
			position: base.Position{Line: 2, Column: 2},
			expect:   "# CSV\n\n## Kind\n\nstatement sink",
		},
		{
			name:     "http param hover",
			code:     "SQL(`select * from example where name = ?`, param('name'))\nCSV()",
			position: base.Position{Line: 1, Column: strings.Index("SQL(`select * from example where name = ?`, param('name'))", "param") + 2},
			expect:   "# param\n\n## Kind\n\nhelper",
		},
		{
			name:     "nil coalescing operator hover",
			code:     "SQL_SELECT('time', 'value', from('example', param('name') ?? 'temperature'))\nCSV()",
			position: base.Position{Line: 1, Column: strings.Index("SQL_SELECT('time', 'value', from('example', param('name') ?? 'temperature'))", "??") + 1},
			expect:   "??\n\nReturns the left operand when it is defined, otherwise returns the right operand.\n\nCategory: operator",
		},
		{
			name:     "pragma hover",
			code:     "//+ log-level=TRACE\nSQL('select * from example')\nCSV()",
			position: base.Position{Line: 1, Column: strings.Index("//+ log-level=TRACE", "log-level") + 2},
			expect:   "log-level\n\nPragma that sets the TQL execution log level.\n\nCategory: pragma",
		},
	}
	for _, tc := range hoverCases {
		t.Run("hover/"+tc.name, func(t *testing.T) {
			hover, err := svc.Hover(ctx, base.Document{Language: base.LanguageTQL, Text: tc.code}, tc.position)
			if err != nil {
				t.Fatalf("Hover returned error: %v", err)
			}
			if hover == nil {
				t.Fatal("expected hover")
			}
			if !strings.Contains(hover.Contents, tc.expect) {
				t.Fatalf("unexpected hover contents:\nwant to contain %q\n got %q", tc.expect, hover.Contents)
			}
		})
	}

	signatureCases := []webUISignatureCase{
		{
			name:     "map function after first argument",
			code:     "MAPVALUE(0, value(0))",
			position: base.Position{Line: 1, Column: strings.Index("MAPVALUE(0, value(0))", ",") + 2},
			expect: expectedSignatureHelp{
				label:           "MAPVALUE(index, expression, options...)",
				documentation:   "`MAPVALUE()` changes or appends a value field in the current record. The first argument selects the value index, and the second argument is evaluated as the new field value. Additional options can name the output field or adjust mapping behavior.",
				activeSignature: 0,
				activeParameter: 1,
				parameters: []base.ParameterInfo{
					{Label: "index", Documentation: "accepts literal:number; required; suggestions: 0, 1, 2"},
					{Label: "expression", Documentation: "accepts expression; required; suggestions: value, key, param, time, parseTime, tz, list, dict, in, ??"},
					{Label: "options", Documentation: "accepts literal:string|helper; repeatable; suggestions: nullValue, lazy"},
				},
			},
		},
		{
			name:     "source csv first argument",
			code:     "CSV(",
			position: base.Position{Line: 1, Column: len("CSV(") + 1},
			expect: expectedSignatureHelp{
				label:           "CSV(input, options...)",
				documentation:   "As a SRC function, `CSV()` reads CSV data from `file()`, `payload()`, or inline content and yields records. Use `field()` helpers to declare input column types and names.",
				activeSignature: 0,
				activeParameter: 0,
				parameters: []base.ParameterInfo{
					{Label: "input", Documentation: "accepts stream|string|helper:file|helper:payload; required; suggestions: file, payload"},
					{Label: "options", Documentation: "accepts helper; repeatable; suggestions: field, charset, logProgress"},
				},
			},
		},
		{
			name:     "sink csv options",
			code:     "FAKE(linspace(1, 3, 3))\nCSV(",
			position: base.Position{Line: 2, Column: len("CSV(") + 1},
			expect: expectedSignatureHelp{
				label:           "CSV(options...)",
				documentation:   "As a SINK function, `CSV()` encodes incoming records as CSV lines. Time and null rendering can be adjusted with formatting helpers.",
				activeSignature: 0,
				activeParameter: 0,
				parameters: []base.ParameterInfo{
					{Label: "options", Documentation: "accepts helper; repeatable; suggestions: nullValue, cache, tz, sqlTimeformat, ansiTimeformat"},
				},
			},
		},
		{
			name:     "sql query parameter",
			code:     "SQL(`select * from example where name = ?`, param('name'))",
			position: base.Position{Line: 1, Column: strings.Index("SQL(`select * from example where name = ?`, param('name'))", ",") + 2},
			expect: expectedSignatureHelp{
				label:           "SQL(sqlText, params...)",
				documentation:   "`SQL()` executes a SQL SELECT statement and produces records from the database. When `bridge()` is supplied, the query is executed through that bridge. Use backtick strings for multi-line SQL text and variadic arguments for bind parameters.",
				activeSignature: 0,
				activeParameter: 1,
				parameters: []base.ParameterInfo{
					{Label: "sqlText", Documentation: "accepts literal:string; required; suggestions: backtick-sql-string"},
					{Label: "params", Documentation: "accepts expression; repeatable; suggestions: param, tz, sqlTimeformat, ansiTimeformat"},
				},
			},
		},
		{
			name:     "param helper first argument",
			code:     "param(",
			position: base.Position{Line: 1, Column: len("param(") + 1},
			expect: expectedSignatureHelp{
				label:           "param(name)",
				documentation:   "`param()` returns a requested query parameter when a TQL script is called via HTTP.",
				activeSignature: 0,
				activeParameter: 0,
				parameters:      []base.ParameterInfo{{Label: "name", Documentation: "accepts literal:string; required; suggestions: query parameter name"}},
			},
		},
	}
	for _, tc := range signatureCases {
		t.Run("signature/"+tc.name, func(t *testing.T) {
			help, err := svc.SignatureHelp(ctx, base.Document{Language: base.LanguageTQL, Text: tc.code}, tc.position)
			if err != nil {
				t.Fatalf("SignatureHelp returned error: %v", err)
			}
			assertSignatureHelp(t, help, tc.expect)
		})
	}
}

type webUICompletionCase struct {
	name     string
	code     string
	position base.Position
	label    string
	expect   expectedCompletionItem
}

type expectedCompletionItem struct {
	kind          base.CompletionItemKind
	detail        string
	documentation string
	insertText    string
}

type webUIHoverCase struct {
	name     string
	code     string
	position base.Position
	expect   string
}

type webUISignatureCase struct {
	name     string
	code     string
	position base.Position
	expect   expectedSignatureHelp
}

type expectedSignatureHelp struct {
	label           string
	documentation   string
	activeSignature int
	activeParameter int
	parameters      []base.ParameterInfo
}

func hasCompletion(items []base.CompletionItem, label string) bool {
	return findCompletion(items, label) != nil
}

func findCompletion(items []base.CompletionItem, label string) *base.CompletionItem {
	for _, item := range items {
		if item.Label == label {
			return &item
		}
	}
	return nil
}

func hasKeyword(items []base.KeywordInfo, label string) bool {
	for _, item := range items {
		if item.Label == label {
			return true
		}
	}
	return false
}

func hasSymbolStatementKind(items []base.SymbolInfo, label string, statementKind string) bool {
	for _, item := range items {
		if item.Label == label && item.StatementKind == statementKind {
			return true
		}
	}
	return false
}

func hasSymbolSignature(items []base.SymbolInfo, label string) bool {
	for _, item := range items {
		if item.Label == label && item.Signature != nil && item.Signature.Label != "" {
			return true
		}
	}
	return false
}

func assertCompletionItem(t *testing.T, item *base.CompletionItem, expect expectedCompletionItem) {
	t.Helper()
	if item.Kind != expect.kind {
		t.Fatalf("expected completion kind %d, got %d", expect.kind, item.Kind)
	}
	if item.Detail != expect.detail {
		t.Fatalf("expected completion detail %q, got %q", expect.detail, item.Detail)
	}
	if item.Documentation != expect.documentation {
		t.Fatalf("expected completion documentation %q, got %q", expect.documentation, item.Documentation)
	}
	if item.InsertText != expect.insertText {
		t.Fatalf("expected completion insertText %q, got %q", expect.insertText, item.InsertText)
	}
}

func assertLeadingCompletionLabels(t *testing.T, items []base.CompletionItem, labels []string) {
	t.Helper()
	if len(items) < len(labels) {
		t.Fatalf("expected at least %d completion items, got %d", len(labels), len(items))
	}
	for idx, label := range labels {
		if items[idx].Label != label {
			t.Fatalf("expected completion item %d to be %q, got %q", idx, label, items[idx].Label)
		}
	}
}

func assertSignatureHelp(t *testing.T, help *base.SignatureHelp, expect expectedSignatureHelp) {
	t.Helper()
	if help == nil || len(help.Signatures) != 1 {
		t.Fatalf("expected one signature help item, got %+v", help)
	}
	if help.ActiveSignature != expect.activeSignature {
		t.Fatalf("expected active signature %d, got %d", expect.activeSignature, help.ActiveSignature)
	}
	if help.ActiveParameter != expect.activeParameter {
		t.Fatalf("expected active parameter %d, got %d", expect.activeParameter, help.ActiveParameter)
	}
	signature := help.Signatures[0]
	if signature.Label != expect.label {
		t.Fatalf("expected signature label %q, got %q", expect.label, signature.Label)
	}
	if signature.Documentation != expect.documentation {
		t.Fatalf("expected signature documentation %q, got %q", expect.documentation, signature.Documentation)
	}
	if len(signature.Parameters) != len(expect.parameters) {
		t.Fatalf("expected %d signature parameters, got %d", len(expect.parameters), len(signature.Parameters))
	}
	for idx, expected := range expect.parameters {
		actual := signature.Parameters[idx]
		if actual.Label != expected.Label || actual.Documentation != expected.Documentation {
			t.Fatalf("unexpected signature parameter %d: want %+v got %+v", idx, expected, actual)
		}
	}
}
