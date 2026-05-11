package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	coretql "github.com/machbase/neo-server/v8/mods/tql"
)

const docRootName = "docsrc"

type docTarget struct {
	Name          string
	Category      string
	Directory     string
	Kind          string
	StatementKind string
}

func main() {
	root, err := packageRoot()
	if err != nil {
		fatal(err)
	}

	targets := collectTargets()
	created, warnings, err := syncDocs(root, targets)
	if err != nil {
		fatal(err)
	}
	generated, err := generateDocs(root, targets)
	if err != nil {
		fatal(err)
	}

	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	fmt.Printf("tql docgen: created %d missing markdown template(s), generated %d markdown metadata item(s), %d warning(s)\n", created, generated, len(warnings))
}

func packageRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("unable to resolve docgen source path")
	}
	return filepath.Dir(filepath.Dir(file)), nil
}

func collectTargets() map[string]docTarget {
	targets := make(map[string]docTarget)
	category := "general"
	for _, def := range coretql.FxDefinitions {
		if strings.HasPrefix(def.Name, "//") {
			category = strings.TrimSpace(strings.TrimPrefix(def.Name, "//"))
			continue
		}
		if def.Name == "" {
			continue
		}

		target := docTarget{
			Name:      def.Name,
			Category:  category,
			Directory: "helpers",
			Kind:      "helper",
		}
		if isStatementName(def.Name) {
			target.Directory = "statements"
			target.Kind = "statement"
			if kind, ok := coretql.StatementKindByFunctionName(def.Name); ok {
				target.StatementKind = statementKindString(kind)
			}
		}
		targets[def.Name] = target
	}
	return targets
}

func isStatementName(name string) bool {
	return name != "" && name == strings.ToUpper(name)
}

func syncDocs(root string, targets map[string]docTarget) (int, []string, error) {
	docRoot := filepath.Join(root, docRootName)
	for _, dir := range []string{"statements", "helpers"} {
		if err := os.MkdirAll(filepath.Join(docRoot, dir), 0o755); err != nil {
			return 0, nil, err
		}
	}

	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)

	created := 0
	for _, name := range names {
		target := targets[name]
		path := filepath.Join(docRoot, target.Directory, target.Name+".md")
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return created, nil, err
		}
		if err := os.WriteFile(path, []byte(templateFor(target)), 0o644); err != nil {
			return created, nil, err
		}
		created++
	}

	warnings, err := warningsForUnknownDocs(docRoot, targets)
	if err != nil {
		return created, nil, err
	}
	return created, warnings, nil
}

func warningsForUnknownDocs(docRoot string, targets map[string]docTarget) ([]string, error) {
	warnings := make([]string, 0)
	for _, dir := range []string{"statements", "helpers"} {
		entries, err := os.ReadDir(filepath.Join(docRoot, dir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".md")
			if _, ok := targets[name]; !ok {
				warnings = append(warnings, fmt.Sprintf("%s/%s does not match any FxDefinitions entry", dir, entry.Name()))
			}
		}
	}
	sort.Strings(warnings)
	return warnings, nil
}

func templateFor(target docTarget) string {
	kindLine := target.Kind
	if target.StatementKind != "" {
		kindLine += " " + target.StatementKind
	}

	return fmt.Sprintf(`# %s

## Kind

%s

## Category

%s

## Signatures

%s

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | no | yes | expression | TODO |

## Description

TODO

## Examples

### Basic

%s

## Related

TODO
`, target.Name, kindLine, target.Category, fenced("text", target.Name+"(...)"), fenced("js", target.Name+"()"))
}

func fenced(language string, body string) string {
	return "```" + language + "\n" + body + "\n```"
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

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "tql docgen: %v\n", err)
	os.Exit(1)
}
