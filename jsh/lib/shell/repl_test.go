package shell

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
)

// TestIsJSInputComplete verifies the bracket/string balance logic used to
// decide when to submit accumulated input for JavaScript evaluation.
func TestIsJSInputComplete(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		complete bool
	}{
		// Empty/whitespace → not complete
		{name: "empty", lines: []string{""}, complete: false},
		{name: "whitespace only", lines: []string{"   ", "\t"}, complete: false},

		// Simple expressions → complete immediately (no closing bracket needed)
		{name: "number literal", lines: []string{"42"}, complete: true},
		{name: "string literal", lines: []string{`"hello"`}, complete: true},
		{name: "arithmetic", lines: []string{"1 + 2"}, complete: true},
		{name: "identifier", lines: []string{"undefined"}, complete: true},

		// Semicolon-terminated statements → complete
		{name: "var with semicolon", lines: []string{"let x = 1;"}, complete: true},
		{name: "expression with semicolon", lines: []string{"console.log(42);"}, complete: true},

		// Balanced brackets → complete
		{name: "object literal", lines: []string{"{a: 1}"}, complete: true},
		{name: "array literal", lines: []string{"[1, 2, 3]"}, complete: true},
		{name: "function call", lines: []string{"Math.max(1, 2)"}, complete: true},
		{name: "function decl", lines: []string{"function foo() { return 1; }"}, complete: true},

		// Unbalanced open brackets → not complete (continue collecting)
		{name: "open brace", lines: []string{"function foo() {"}, complete: false},
		{name: "open paren", lines: []string{"Math.max("}, complete: false},
		{name: "open bracket", lines: []string{"["}, complete: false},

		// Multi-line complete
		{
			name: "multi-line function complete",
			lines: []string{
				"function foo() {",
				"  return 1;",
				"}",
			},
			complete: true,
		},
		{
			name: "multi-line function incomplete",
			lines: []string{
				"function foo() {",
				"  return 1;",
			},
			complete: false,
		},
		{
			name:     "multi-line array complete",
			lines:    []string{"[", "1,", "2", "]"},
			complete: true,
		},

		// Open string literal → not complete
		{name: "unclosed single quote", lines: []string{`'hello`}, complete: false},
		{name: "unclosed double quote", lines: []string{`"hello`}, complete: false},
		{name: "unclosed backtick", lines: []string{"`hello"}, complete: false},

		// Closed string literal → complete (balance still 0)
		{name: "closed single quote", lines: []string{`'hello'`}, complete: true},
		{name: "closed double quote", lines: []string{`"hello"`}, complete: true},
		{name: "closed backtick", lines: []string{"`hello`"}, complete: true},

		// Escaped quote inside string → not treated as end of string
		{name: "escaped quote in string", lines: []string{`'it\'s`}, complete: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isJSInputComplete(tc.lines)
			if got != tc.complete {
				t.Errorf("isJSInputComplete(%q) = %v, want %v", strings.Join(tc.lines, "\\n"), got, tc.complete)
			}
		})
	}
}

// TestParseSlashInput verifies command/argument splitting for slash commands.
func TestParseSlashInput(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs string
	}{
		{`\quit`, "quit", ""},
		{`\q`, "q", ""},
		{`\help`, "help", ""},
		{`\load foo.js`, "load", "foo.js"},
		{`\use neo`, "use", "neo"},
		{`\load  spaced.js `, "load", "spaced.js"},
	}
	for _, tc := range tests {
		cmd, args := parseSlashInput(tc.input)
		if cmd != tc.wantCmd || args != tc.wantArgs {
			t.Errorf("parseSlashInput(%q) = (%q, %q), want (%q, %q)",
				tc.input, cmd, args, tc.wantCmd, tc.wantArgs)
		}
	}
}

// TestSlashRegistry verifies that built-in commands are registered and
// dispatchSlashCommand routes correctly.
func TestSlashRegistry(t *testing.T) {
	rt := goja.New()
	r := &Repl{rt: rt, cfg: defaultReplConfig()}
	r.registerBuiltinCommands()

	// All registered aliases must be findable.
	aliases := []string{"quit", "q", "exit", "help", "clear"}
	for _, alias := range aliases {
		found := false
		for _, e := range r.cmds {
			for _, a := range e.aliases {
				if a == alias {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("alias %q not found in registry after registerBuiltinCommands", alias)
		}
	}

	// Unknown command → no exit, no panic.
	var buf strings.Builder
	writeFunc := writeAdapter{&buf}
	val, exit := r.dispatchSlashCommand("unknown_cmd", "", writeFunc)
	if exit {
		t.Error("unknown command should not request exit")
	}
	if val != nil {
		t.Error("unknown command should return nil value")
	}
	if !strings.Contains(buf.String(), "Unknown command") {
		t.Errorf("unknown command output %q should contain 'Unknown command'", buf.String())
	}

	// \quit → exit==true, val is goja.Value(0).
	val, exit = r.dispatchSlashCommand("quit", "", writeFunc)
	if !exit {
		t.Error("\\quit should request exit")
	}
	if val == nil {
		t.Error("\\quit should return non-nil value")
	}
}

// writeAdapter adapts strings.Builder to io.Writer for test use.
type writeAdapter struct{ b *strings.Builder }

func (w writeAdapter) Write(p []byte) (int, error) { return w.b.Write(p) }

// TestReplConfigDefaults verifies that defaultReplConfig returns valid defaults.
func TestReplConfigDefaults(t *testing.T) {
	cfg := defaultReplConfig()
	if cfg.Profile.Name == "" {
		t.Error("ReplConfig.Profile.Name should not be empty")
	}
	if cfg.History.Name == "" {
		t.Error("ReplConfig.History.Name should not be empty")
	}
	if cfg.History.Size <= 0 {
		t.Error("ReplConfig.History.Size should be positive")
	}
	if !cfg.EnableHistoryCycling {
		t.Error("ReplConfig.EnableHistoryCycling should default to true")
	}
}
