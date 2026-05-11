package tql

import (
	"strings"
	"testing"

	coretql "github.com/machbase/neo-server/v8/mods/tql"
)

func TestGeneratedDocsMatchFxDefinitions(t *testing.T) {
	defs := 0
	for _, def := range coretql.FxDefinitions {
		if strings.HasPrefix(def.Name, "//") || def.Name == "" {
			continue
		}
		defs++
		if _, ok := generatedTqlDocs[def.Name]; !ok {
			t.Fatalf("missing generated doc for %s", def.Name)
		}
	}
	if len(generatedTqlDocs) != defs {
		t.Fatalf("expected %d generated docs, got %d", defs, len(generatedTqlDocs))
	}
}

func TestGeneratedDocsContainPrimaryMarkdownDrafts(t *testing.T) {
	for _, name := range []string{"SQL", "SQL_SELECT", "CSV", "MAPVALUE", "FAKE", "INSERT", "APPEND", "JSON", "param", "value", "sqlTimeformat"} {
		doc, ok := generatedTqlDocs[name]
		if !ok {
			t.Fatalf("missing generated doc for %s", name)
		}
		if !tqlDocHasContent(doc) {
			t.Fatalf("expected %s to have a non-template description", name)
		}
		if doc.Markdown == "" || !strings.Contains(doc.Markdown, "## Description") || !strings.Contains(doc.Markdown, "## Examples") {
			t.Fatalf("expected %s markdown body to contain required sections", name)
		}
		if len(doc.Signatures) == 0 {
			t.Fatalf("expected %s to have signatures", name)
		}
	}
}
