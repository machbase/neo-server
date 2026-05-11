package jsh

import (
	"context"
	"strings"
	"testing"

	base "github.com/machbase/neo-server/v8/mods/lsp"
)

func TestDiagnosticsSyntaxError(t *testing.T) {
	svc := NewService()
	diags, err := svc.Diagnostics(context.Background(), base.Document{
		URI:      "memory://test.js",
		Language: base.LanguageJSH,
		Text:     "function broken( {\nconsole.log('x')",
	})
	if err != nil {
		t.Fatalf("Diagnostics returned error: %v", err)
	}
	if len(diags) == 0 {
		t.Fatal("expected diagnostic")
	}
	if diags[0].Source != "jsh" {
		t.Fatalf("expected jsh source, got %q", diags[0].Source)
	}
	if diags[0].Range.Start.Line <= 0 || diags[0].Range.Start.Column <= 0 {
		t.Fatalf("expected normalized range, got %+v", diags[0].Range)
	}
}

func TestDiagnosticsUnsupportedSyntax(t *testing.T) {
	svc := NewService()
	diags, err := svc.Diagnostics(context.Background(), base.Document{
		URI:      "memory://test.js",
		Language: base.LanguageJSH,
		Text:     "await run()\nimport x from 'y'",
	})
	if err != nil {
		t.Fatalf("Diagnostics returned error: %v", err)
	}
	if !hasDiagnosticCode(diags, "unsupported_await") {
		t.Fatalf("expected unsupported_await diagnostic, got %+v", diags)
	}
	if !hasDiagnosticCode(diags, "unsupported_import") {
		t.Fatalf("expected unsupported_import diagnostic, got %+v", diags)
	}
}

func TestCompletionIncludesJshRuntimeSymbols(t *testing.T) {
	svc := NewService()
	items, err := svc.Completion(context.Background(), base.Document{Language: base.LanguageJSH}, base.Position{Line: 1, Column: 1})
	if err != nil {
		t.Fatalf("Completion returned error: %v", err)
	}
	if !hasCompletion(items, "require") {
		t.Fatal("expected require completion")
	}
	if hasCompletion(items, "@jsh/process") {
		t.Fatal("did not expect @jsh/process completion")
	}
	if !hasCompletion(items, "fs") {
		t.Fatal("expected fs module completion")
	}
}

func TestCompletionIncludesRequireModules(t *testing.T) {
	svc := NewService()
	text := "const fs = require('"
	items, err := svc.Completion(context.Background(), base.Document{Language: base.LanguageJSH, Text: text}, base.Position{Line: 1, Column: len(text) + 1})
	if err != nil {
		t.Fatalf("Completion returned error: %v", err)
	}
	if !hasCompletion(items, "fs") {
		t.Fatal("expected fs module completion")
	}
	if hasCompletion(items, "@jsh/fs") {
		t.Fatal("did not expect @jsh/fs module completion")
	}
}

func TestCompletionIncludesModuleExports(t *testing.T) {
	svc := NewService()
	text := "const fs = require('fs');\nfs."
	items, err := svc.Completion(context.Background(), base.Document{Language: base.LanguageJSH, Text: text}, base.Position{Line: 2, Column: 4})
	if err != nil {
		t.Fatalf("Completion returned error: %v", err)
	}
	if !hasCompletion(items, "readFileSync") {
		t.Fatal("expected readFileSync export completion")
	}
	item := findCompletion(items, "readFileSync")
	if item == nil || !strings.Contains(item.Documentation, "Read file contents synchronously") {
		t.Fatalf("expected readFileSync JSDoc completion, got %+v", item)
	}
}

func TestBuildMetadataIncludesJSDocExports(t *testing.T) {
	metadata := BuildMetadata()
	if metadata.Language != base.LanguageJSH {
		t.Fatalf("expected jsh metadata, got %q", metadata.Language)
	}
	if hasModule(metadata.Modules, "@jsh/fs") {
		t.Fatal("did not expect @jsh/fs module metadata")
	}
	fsModule := findModule(metadata.Modules, "fs")
	if fsModule == nil {
		t.Fatal("expected fs module metadata")
	}
	readFileSync := findSymbol(fsModule.Exports, "readFileSync")
	if readFileSync == nil {
		t.Fatal("expected fs.readFileSync metadata")
	}
	if readFileSync.Signature == nil || readFileSync.Signature.Label == "" {
		t.Fatalf("expected readFileSync signature, got %+v", readFileSync)
	}
	if !strings.Contains(readFileSync.Documentation, "Read file contents synchronously") {
		t.Fatalf("expected JSDoc documentation, got %q", readFileSync.Documentation)
	}
}

func TestMetadataReturnsJshMetadata(t *testing.T) {
	svc := NewService()
	metadata := svc.Metadata()
	if metadata.Language != base.LanguageJSH {
		t.Fatalf("expected jsh metadata, got %q", metadata.Language)
	}
	if len(metadata.Modules) == 0 {
		t.Fatal("expected module metadata")
	}
}

func TestHoverReturnsJshRuntimeInfo(t *testing.T) {
	svc := NewService()
	hover, err := svc.Hover(context.Background(), base.Document{
		Language: base.LanguageJSH,
		Text:     "require('@jsh/process')",
	}, base.Position{Line: 1, Column: 2})
	if err != nil {
		t.Fatalf("Hover returned error: %v", err)
	}
	if hover == nil || hover.Contents == "" {
		t.Fatalf("expected hover, got %+v", hover)
	}
}

func TestHoverReturnsJSDocExportInfo(t *testing.T) {
	svc := NewService()
	text := "const fs = require('fs');\nfs.readFileSync('/tmp/a')"
	hover, err := svc.Hover(context.Background(), base.Document{Language: base.LanguageJSH, Text: text}, base.Position{Line: 2, Column: 5})
	if err != nil {
		t.Fatalf("Hover returned error: %v", err)
	}
	if hover == nil {
		t.Fatal("expected hover")
	}
	if !strings.Contains(hover.Contents, "Read file contents synchronously") {
		t.Fatalf("expected JSDoc hover, got %q", hover.Contents)
	}
	if !strings.Contains(hover.Contents, "readFileSync") {
		t.Fatalf("expected readFileSync hover, got %q", hover.Contents)
	}
}

func TestSignatureHelpReturnsJSDocExportInfo(t *testing.T) {
	svc := NewService()
	text := "const fs = require('fs');\nfs.readFileSync('/tmp/a', "
	help, err := svc.SignatureHelp(context.Background(), base.Document{Language: base.LanguageJSH, Text: text}, base.Position{Line: 2, Column: len("fs.readFileSync('/tmp/a', ") + 1})
	if err != nil {
		t.Fatalf("SignatureHelp returned error: %v", err)
	}
	if help == nil || len(help.Signatures) == 0 {
		t.Fatalf("expected signature help, got %+v", help)
	}
	if !strings.Contains(help.Signatures[0].Label, "readFileSync") {
		t.Fatalf("expected readFileSync signature, got %+v", help.Signatures[0])
	}
	if help.ActiveParameter != 1 {
		t.Fatalf("expected active parameter 1, got %d", help.ActiveParameter)
	}
}

func TestSignatureHelpReturnsRequireInfo(t *testing.T) {
	svc := NewService()
	text := "const fs = require("
	help, err := svc.SignatureHelp(context.Background(), base.Document{Language: base.LanguageJSH, Text: text}, base.Position{Line: 1, Column: len(text) + 1})
	if err != nil {
		t.Fatalf("SignatureHelp returned error: %v", err)
	}
	if help == nil || len(help.Signatures) == 0 {
		t.Fatalf("expected signature help, got %+v", help)
	}
	if help.Signatures[0].Label != "require(module)" {
		t.Fatalf("expected require signature, got %+v", help.Signatures[0])
	}
}

func TestWebUIMessageForJshContexts(t *testing.T) {
	svc := NewService()
	ctx := context.Background()
	expectedDocumentation := "```js\nreadFileSync(path: string, options: object): string|Array File contents as string or byte array\n```\n\nRead file contents synchronously\n\nReturns: string|Array File contents as string or byte array"
	readFileSyncSignature := "readFileSync(path: string, options: object): string|Array File contents as string or byte array"

	completionCases := []webUICompletionCase{
		{
			name:     "require string module completion",
			code:     "const fs = require('",
			position: base.Position{Line: 1, Column: len("const fs = require('") + 1},
			label:    "fs",
			expect: expectedCompletionItem{
				kind:          base.CompletionModule,
				detail:        "JSH module",
				documentation: "JSH fs module",
				insertText:    "fs",
			},
		},
		{
			name:     "required module member completion",
			code:     "const fs = require('fs');\nfs.",
			position: base.Position{Line: 2, Column: len("fs.") + 1},
			label:    "readFileSync",
			expect: expectedCompletionItem{
				kind:          base.CompletionFunction,
				detail:        readFileSyncSignature,
				documentation: expectedDocumentation,
				insertText:    "readFileSync",
			},
		},
		{
			name:     "top level runtime completion",
			code:     "con",
			position: base.Position{Line: 1, Column: len("con") + 1},
			label:    "console",
			expect: expectedCompletionItem{
				kind:          base.CompletionVariable,
				detail:        "JSH console",
				documentation: "JSH console object with log, print, println, printf, debug, info, warn, and error methods.",
				insertText:    "console",
			},
		},
	}
	for _, tc := range completionCases {
		t.Run("completion/"+tc.name, func(t *testing.T) {
			items, err := svc.Completion(ctx, base.Document{Language: base.LanguageJSH, Text: tc.code}, tc.position)
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
			name:     "module member jsdoc hover",
			code:     "const fs = require('fs');\nfs.readFileSync('/tmp/a', ",
			position: base.Position{Line: 2, Column: len("fs.readFileSync")},
			expect:   "fs.readFileSync\n\n" + expectedDocumentation,
		},
		{
			name:     "runtime function hover",
			code:     "require('fs')",
			position: base.Position{Line: 1, Column: 2},
			expect:   "require\n\nLoads JSH JavaScript modules and native modules.",
		},
	}
	for _, tc := range hoverCases {
		t.Run("hover/"+tc.name, func(t *testing.T) {
			hover, err := svc.Hover(ctx, base.Document{Language: base.LanguageJSH, Text: tc.code}, tc.position)
			if err != nil {
				t.Fatalf("Hover returned error: %v", err)
			}
			if hover == nil {
				t.Fatal("expected hover")
			}
			if hover.Contents != tc.expect {
				t.Fatalf("unexpected hover contents:\nwant %q\n got %q", tc.expect, hover.Contents)
			}
		})
	}

	signatureCases := []webUISignatureCase{
		{
			name:     "module member second parameter",
			code:     "const fs = require('fs');\nfs.readFileSync('/tmp/a', ",
			position: base.Position{Line: 2, Column: len("fs.readFileSync('/tmp/a', ") + 1},
			expect: expectedSignatureHelp{
				label:           readFileSyncSignature,
				documentation:   "Read file contents synchronously",
				activeSignature: 0,
				activeParameter: 1,
				parameters: []base.ParameterInfo{
					{Label: "path: string", Documentation: "File path"},
					{Label: "options: object", Documentation: "Options (encoding: 'utf8' or null for buffer)"},
				},
			},
		},
		{
			name:     "require first parameter",
			code:     "const fs = require(",
			position: base.Position{Line: 1, Column: len("const fs = require(") + 1},
			expect: expectedSignatureHelp{
				label:           "require(module)",
				documentation:   "Loads a JSH JavaScript module.",
				activeSignature: 0,
				activeParameter: 0,
				parameters:      []base.ParameterInfo{{Label: "module", Documentation: "User-facing module id"}},
			},
		},
	}
	for _, tc := range signatureCases {
		t.Run("signature/"+tc.name, func(t *testing.T) {
			help, err := svc.SignatureHelp(ctx, base.Document{Language: base.LanguageJSH, Text: tc.code}, tc.position)
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

func hasDiagnosticCode(diags []base.Diagnostic, code string) bool {
	for _, diag := range diags {
		if diag.Code == code {
			return true
		}
	}
	return false
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

func hasModule(items []base.ModuleInfo, id string) bool {
	return findModule(items, id) != nil
}

func findModule(items []base.ModuleInfo, id string) *base.ModuleInfo {
	for idx := range items {
		if items[idx].ID == id {
			return &items[idx]
		}
	}
	return nil
}

func findSymbol(items []base.SymbolInfo, label string) *base.SymbolInfo {
	for idx := range items {
		if items[idx].Label == label {
			return &items[idx]
		}
	}
	return nil
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
