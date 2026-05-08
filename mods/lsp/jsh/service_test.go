package jsh

import (
	"context"
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
	if !hasCompletion(items, "@jsh/process") {
		t.Fatal("expected @jsh/process completion")
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

func hasDiagnosticCode(diags []base.Diagnostic, code string) bool {
	for _, diag := range diags {
		if diag.Code == code {
			return true
		}
	}
	return false
}

func hasCompletion(items []base.CompletionItem, label string) bool {
	for _, item := range items {
		if item.Label == label {
			return true
		}
	}
	return false
}
