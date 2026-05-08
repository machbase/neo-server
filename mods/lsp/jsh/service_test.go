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
