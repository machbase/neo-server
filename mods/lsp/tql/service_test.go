package tql

import (
	"context"
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

func hasCompletion(items []base.CompletionItem, label string) bool {
	for _, item := range items {
		if item.Label == label {
			return true
		}
	}
	return false
}
