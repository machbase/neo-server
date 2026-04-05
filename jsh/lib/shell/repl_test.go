package shell

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

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
	hist := NewHistory(HistoryConfig{Name: "test", Size: 10, Enabled: false})
	sctx := slashCtx{Writer: writeFunc, History: hist, Profile: r.cfg.Profile}
	val, exit := r.dispatchSlashCommand("unknown_cmd", "", sctx)
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
	val, exit = r.dispatchSlashCommand("quit", "", sctx)
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

// TestReplConfigMerge verifies that mergeReplConfigFromMap correctly overlays
// a JS options map onto a base ReplConfig.
func TestReplConfigMerge(t *testing.T) {
	base := defaultReplConfig()

	t.Run("eval sets Eval field", func(t *testing.T) {
		cfg := mergeReplConfigFromMap(base, map[string]interface{}{"eval": "1+1"})
		if cfg.Eval != "1+1" {
			t.Errorf("Eval = %q, want %q", cfg.Eval, "1+1")
		}
	})
	t.Run("print sets PrintEval", func(t *testing.T) {
		cfg := mergeReplConfigFromMap(base, map[string]interface{}{
			"eval": "42", "print": true,
		})
		if !cfg.PrintEval {
			t.Error("PrintEval should be true when print=true")
		}
	})
	t.Run("noHistory disables history", func(t *testing.T) {
		cfg := mergeReplConfigFromMap(base, map[string]interface{}{"noHistory": true})
		if cfg.History.Enabled {
			t.Error("History.Enabled should be false when noHistory=true")
		}
	})
	t.Run("load string appended", func(t *testing.T) {
		cfg := mergeReplConfigFromMap(base, map[string]interface{}{"load": "setup.js"})
		if len(cfg.PreloadFiles) != 1 || cfg.PreloadFiles[0] != "setup.js" {
			t.Errorf("PreloadFiles = %v, want [setup.js]", cfg.PreloadFiles)
		}
	})
	t.Run("load array appended", func(t *testing.T) {
		cfg := mergeReplConfigFromMap(base, map[string]interface{}{
			"load": []interface{}{"a.js", "b.js"},
		})
		if len(cfg.PreloadFiles) != 2 {
			t.Errorf("PreloadFiles len = %d, want 2", len(cfg.PreloadFiles))
		}
	})
	t.Run("require string appended", func(t *testing.T) {
		cfg := mergeReplConfigFromMap(base, map[string]interface{}{"require": "fs"})
		if len(cfg.PreloadModules) != 1 || cfg.PreloadModules[0] != "fs" {
			t.Errorf("PreloadModules = %v, want [fs]", cfg.PreloadModules)
		}
	})
	t.Run("unknown key ignored", func(t *testing.T) {
		cfg := mergeReplConfigFromMap(base, map[string]interface{}{"unknownKey": "value"})
		if cfg.Eval != "" {
			t.Error("unknown key should not affect Eval")
		}
	})
}

// TestRunEvalMode verifies the eval-and-exit path in Loop().
// We call runEval directly to avoid needing a terminal.
func TestRunEvalMode(t *testing.T) {
	rt := goja.New()
	// Register minimal console.println so consoleRenderer does not panic.
	console := rt.NewObject()
	rt.Set("console", console)
	console.Set("println", func(call goja.FunctionCall) goja.Value {
		return goja.Undefined()
	})

	r := &Repl{rt: rt, cfg: defaultReplConfig()}
	r.registerBuiltinCommands()

	var buf strings.Builder
	w := writeAdapter{&buf}
	renderer := WriterRenderer{}

	t.Run("eval returns 0 on success", func(t *testing.T) {
		val := r.runEval("1 + 1", false, w, renderer, 0)
		if val.ToInteger() != 0 {
			t.Errorf("runEval exit = %v, want 0", val)
		}
	})
	t.Run("eval returns 1 on error", func(t *testing.T) {
		val := r.runEval(")(invalid", false, w, renderer, 0)
		if val.ToInteger() != 1 {
			t.Errorf("runEval exit = %v, want 1", val)
		}
	})
	t.Run("print=true writes result", func(t *testing.T) {
		buf.Reset()
		r.runEval("42", true, w, renderer, 0)
		if !strings.Contains(buf.String(), "42") {
			t.Errorf("output %q should contain '42'", buf.String())
		}
	})
	t.Run("print=false does not write result", func(t *testing.T) {
		buf.Reset()
		r.runEval("42", false, w, renderer, 0)
		if strings.Contains(buf.String(), "42") {
			t.Errorf("output %q should not contain '42' when print=false", buf.String())
		}
	})
}

// memHistory is an in-memory SessionHistory used in tests to avoid file I/O.
type memHistory struct{ entries []string }

func (h *memHistory) Len() int        { return len(h.entries) }
func (h *memHistory) At(i int) string { return h.entries[i] }
func (h *memHistory) Add(s string)    { h.entries = append(h.entries, s) }

// TestSlashCtxHandlers verifies new Phase 3 slash commands via dispatchSlashCommand.
func TestSlashCtxHandlers(t *testing.T) {
	rt := goja.New()
	rt.Set("myCustomGlobal", rt.ToValue(123))
	r := &Repl{rt: rt, cfg: defaultReplConfig()}
	r.registerBuiltinCommands()
	hist := &memHistory{}

	var buf strings.Builder
	ctx := slashCtx{
		Writer:  writeAdapter{&buf},
		History: hist,
		Profile: r.cfg.Profile,
	}

	t.Run("modules lists known modules", func(t *testing.T) {
		buf.Reset()
		_, exit := r.dispatchSlashCommand("modules", "", ctx)
		if exit {
			t.Error("\\modules should not request exit")
		}
		if !strings.Contains(buf.String(), "process") {
			t.Errorf("\\modules output %q should mention 'process'", buf.String())
		}
	})
	t.Run("globals lists non-standard globals", func(t *testing.T) {
		buf.Reset()
		_, exit := r.dispatchSlashCommand("globals", "", ctx)
		if exit {
			t.Error("\\globals should not request exit")
		}
		if !strings.Contains(buf.String(), "myCustomGlobal") {
			t.Errorf("\\globals output %q should contain 'myCustomGlobal'", buf.String())
		}
	})
	t.Run("history empty", func(t *testing.T) {
		buf.Reset()
		_, exit := r.dispatchSlashCommand("history", "", ctx)
		if exit {
			t.Error("\\history should not request exit")
		}
		if !strings.Contains(buf.String(), "empty") {
			t.Errorf("\\history empty output %q should say 'empty'", buf.String())
		}
	})
	t.Run("history with entries", func(t *testing.T) {
		hist.Add("let x = 1;")
		hist.Add("x + 2")
		buf.Reset()
		_, exit := r.dispatchSlashCommand("history", "", ctx)
		if exit {
			t.Error("\\history should not request exit")
		}
		if !strings.Contains(buf.String(), "let x = 1;") {
			t.Errorf("\\history output %q should contain history entry", buf.String())
		}
	})
	t.Run("inspect evaluates expression", func(t *testing.T) {
		buf.Reset()
		_, exit := r.dispatchSlashCommand("inspect", "1 + 1", ctx)
		if exit {
			t.Error("\\inspect should not request exit")
		}
		if !strings.Contains(buf.String(), "2") {
			t.Errorf("\\inspect output %q should contain '2'", buf.String())
		}
	})
	t.Run("inspect empty args shows usage", func(t *testing.T) {
		buf.Reset()
		r.dispatchSlashCommand("inspect", "", ctx)
		if !strings.Contains(buf.String(), "Usage") {
			t.Errorf("\\inspect with no args should show usage, got %q", buf.String())
		}
	})
	t.Run("json evaluates and formats", func(t *testing.T) {
		buf.Reset()
		_, exit := r.dispatchSlashCommand("json", "[1,2,3]", ctx)
		if exit {
			t.Error("\\json should not request exit")
		}
		if !strings.Contains(buf.String(), "1") {
			t.Errorf("\\json output %q should contain array content", buf.String())
		}
	})
	t.Run("reset does not exit", func(t *testing.T) {
		buf.Reset()
		_, exit := r.dispatchSlashCommand("reset", "", ctx)
		if exit {
			t.Error("\\reset should not request exit")
		}
	})
}

// TestDefaultProfileKnownModules checks that the default profile lists modules.
func TestDefaultProfileKnownModules(t *testing.T) {
	p := defaultReplProfile()
	if len(p.KnownModules) == 0 {
		t.Error("defaultReplProfile should list at least one KnownModule")
	}
	found := false
	for _, m := range p.KnownModules {
		if strings.Contains(m, "process") {
			found = true
			break
		}
	}
	if !found {
		t.Error("KnownModules should include 'process'")
	}
}

// ---- Phase 5 tests ----

// TestIsJSInputCompleteComments verifies that comment content does not affect
// bracket balance when determining input completeness.
func TestIsJSInputCompleteComments(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		complete bool
	}{
		// Single-line comments: brackets inside should not count.
		{
			name:     "line comment with open brace",
			lines:    []string{"// this { is a comment"},
			complete: true,
		},
		{
			name:     "code then line comment with brace",
			lines:    []string{"let x = 1; // { comment"},
			complete: true,
		},
		{
			name:     "open brace before line comment",
			lines:    []string{"function foo() { // still open"},
			complete: false,
		},
		// Block comments.
		{
			name:     "block comment with brackets",
			lines:    []string{"/* { } ( ) */"},
			complete: true,
		},
		{
			name:     "unclosed block comment",
			lines:    []string{"/* not closed"},
			complete: false,
		},
		{
			name: "multi-line block comment closed",
			lines: []string{
				"/*",
				"  {",
				"*/",
				"let x = 1;",
			},
			complete: true,
		},
		// Escaped backslash before quote: \\" — the quote IS the string end.
		{
			name:     "double backslash before quote closes string",
			lines:    []string{`"hello\\"`},
			complete: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isJSInputComplete(tc.lines)
			if got != tc.complete {
				t.Errorf("isJSInputComplete(%q) = %v, want %v",
					strings.Join(tc.lines, `\n`), got, tc.complete)
			}
		})
	}
}

// TestInspectValue verifies that inspectValue annotates primitive types and
// handles functions and null/undefined correctly.
func TestInspectValue(t *testing.T) {
	rt := goja.New()

	t.Run("null", func(t *testing.T) {
		got := inspectValue(rt, goja.Null())
		if got != "null" {
			t.Errorf("inspectValue(null) = %q, want %q", got, "null")
		}
	})
	t.Run("undefined", func(t *testing.T) {
		got := inspectValue(rt, goja.Undefined())
		if got != "undefined" {
			t.Errorf("inspectValue(undefined) = %q, want %q", got, "undefined")
		}
	})
	t.Run("string annotated", func(t *testing.T) {
		val, _ := rt.RunString(`"hello"`)
		got := inspectValue(rt, val)
		if !strings.Contains(got, "string") {
			t.Errorf("inspectValue(string) = %q should contain 'string'", got)
		}
		if !strings.Contains(got, "hello") {
			t.Errorf("inspectValue(string) = %q should contain value", got)
		}
	})
	t.Run("number annotated", func(t *testing.T) {
		val, _ := rt.RunString("42")
		got := inspectValue(rt, val)
		if !strings.Contains(got, "number") {
			t.Errorf("inspectValue(number) = %q should contain 'number'", got)
		}
		if !strings.Contains(got, "42") {
			t.Errorf("inspectValue(number) = %q should contain '42'", got)
		}
	})
	t.Run("boolean annotated", func(t *testing.T) {
		val, _ := rt.RunString("true")
		got := inspectValue(rt, val)
		if !strings.Contains(got, "boolean") {
			t.Errorf("inspectValue(bool) = %q should contain 'boolean'", got)
		}
	})
	t.Run("named function", func(t *testing.T) {
		val, _ := rt.RunString("(function myFn() {})")
		got := inspectValue(rt, val)
		if !strings.Contains(got, "myFn") {
			t.Errorf("inspectValue(function) = %q should contain function name", got)
		}
	})
	t.Run("array as JSON", func(t *testing.T) {
		val, _ := rt.RunString("[1,2,3]")
		got := inspectValue(rt, val)
		if !strings.Contains(got, "1") {
			t.Errorf("inspectValue(array) = %q should contain array content", got)
		}
	})
}

// TestToJSON verifies that toJSON handles functions and null/undefined.
func TestToJSON(t *testing.T) {
	rt := goja.New()

	t.Run("null", func(t *testing.T) {
		if got := toJSON(rt, goja.Null()); got != "null" {
			t.Errorf("toJSON(null) = %q, want %q", got, "null")
		}
	})
	t.Run("undefined", func(t *testing.T) {
		if got := toJSON(rt, goja.Undefined()); got != "undefined" {
			t.Errorf("toJSON(undefined) = %q, want %q", got, "undefined")
		}
	})
	t.Run("function", func(t *testing.T) {
		val, _ := rt.RunString("(function() {})")
		got := toJSON(rt, val)
		if !strings.Contains(got, "Function") {
			t.Errorf("toJSON(function) = %q should indicate function", got)
		}
	})
	t.Run("number", func(t *testing.T) {
		val, _ := rt.RunString("123")
		got := toJSON(rt, val)
		if !strings.Contains(got, "123") {
			t.Errorf("toJSON(number) = %q should contain '123'", got)
		}
	})
}

// TestResetSoftReset verifies that \reset clears user-defined globals and
// preserves initial globals.
func TestResetSoftReset(t *testing.T) {
	rt := goja.New()
	r := &Repl{rt: rt, cfg: defaultReplConfig()}
	r.registerBuiltinCommands()

	// Capture initial globals (simulating what Loop() does).
	initialGlobals := make(map[string]struct{})
	for _, k := range rt.GlobalObject().Keys() {
		initialGlobals[k] = struct{}{}
	}

	// Add a user-defined global after the snapshot.
	rt.Set("userVar", rt.ToValue(42))

	var buf strings.Builder
	ctx := slashCtx{
		Writer:         writeAdapter{&buf},
		History:        &memHistory{},
		Profile:        r.cfg.Profile,
		InitialGlobals: initialGlobals,
	}

	// Verify userVar exists before reset.
	if v := rt.GlobalObject().Get("userVar"); v == nil || v == goja.Undefined() {
		t.Fatal("userVar should exist before reset")
	}

	_, exit := r.dispatchSlashCommand("reset", "", ctx)
	if exit {
		t.Error("\\reset should not request exit")
	}

	// After reset, userVar should be gone.
	if v := rt.GlobalObject().Get("userVar"); v != nil && v != goja.Undefined() {
		t.Errorf("userVar should be cleared after \\reset, got %v", v)
	}

	if !strings.Contains(buf.String(), "Reset complete") {
		t.Errorf("\\reset output %q should confirm reset", buf.String())
	}
}

// TestHistoryNameConfig verifies that mergeReplConfigFromMap sets History.Name.
func TestHistoryNameConfig(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{"historyName": "my_session"})
	if cfg.History.Name != "my_session" {
		t.Errorf("History.Name = %q, want %q", cfg.History.Name, "my_session")
	}
}

// TestUserProfileName verifies the user profile has correct Name field.
func TestUserProfileName(t *testing.T) {
	p := userReplProfile()
	if p.Name != "user" {
		t.Errorf("userReplProfile().Name = %q, want %q", p.Name, "user")
	}
}

// ── Phase A: Agent profile ────────────────────────────────────────────────────

// TestAgentProfileName verifies the agent profile has correct Name field.
func TestAgentProfileName(t *testing.T) {
	p := agentReplProfile()
	if p.Name != "agent" {
		t.Errorf("agentReplProfile().Name = %q, want %q", p.Name, "agent")
	}
}

// TestAgentProfileNoBanner verifies the agent profile banner is nil
// so AgentRenderer emits no banner JSON event by default.
func TestAgentProfileNoBanner(t *testing.T) {
	p := agentReplProfile()
	if p.Banner != nil {
		t.Errorf("agentReplProfile().Banner should be nil for clean JSON output")
	}
}

// TestMergeProfileAgent verifies mergeReplConfigFromMap("agent") installs
// agentReplProfile and an AgentRenderer.
func TestMergeProfileAgent(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{"profile": "agent"})
	if cfg.Profile.Name != "agent" {
		t.Errorf("profile.Name = %q, want %q", cfg.Profile.Name, "agent")
	}
	if _, ok := cfg.Renderer.(*AgentRenderer); !ok {
		t.Errorf("Renderer should be *AgentRenderer for profile:agent, got %T", cfg.Renderer)
	}
}

// TestMergeJsonFlag verifies mergeReplConfigFromMap(json:true) installs AgentRenderer.
func TestMergeJsonFlag(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{"json": true})
	if _, ok := cfg.Renderer.(*AgentRenderer); !ok {
		t.Errorf("Renderer should be *AgentRenderer for json:true, got %T", cfg.Renderer)
	}
}

// ── Phase B: AgentRenderer output ────────────────────────────────────────────

// TestAgentRendererBannerNoop verifies that a nil-banner profile emits no output.
func TestAgentRendererBannerNoop(t *testing.T) {
	r := &AgentRenderer{}
	var buf bytes.Buffer
	p := agentReplProfile()
	if err := r.RenderBanner(&buf, p); err != nil {
		t.Fatalf("RenderBanner error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil banner, got %q", buf.String())
	}
}

// TestAgentRendererBannerEmit verifies that a non-empty banner emits a JSON event.
func TestAgentRendererBannerEmit(t *testing.T) {
	r := &AgentRenderer{}
	var buf bytes.Buffer
	p := userReplProfile()
	if err := r.RenderBanner(&buf, p); err != nil {
		t.Fatalf("RenderBanner error: %v", err)
	}
	line := strings.TrimSpace(buf.String())
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		t.Fatalf("banner output is not valid JSON: %v\noutput: %q", err, line)
	}
	if obj["event"] != "banner" {
		t.Errorf("event = %q, want %q", obj["event"], "banner")
	}
}

// TestAgentRendererError verifies RenderError emits ok:false JSON with error field.
func TestAgentRendererError(t *testing.T) {
	r := &AgentRenderer{}
	var buf bytes.Buffer
	if err := r.RenderError(&buf, fmt.Errorf("something went wrong")); err != nil {
		t.Fatalf("RenderError error: %v", err)
	}
	line := strings.TrimSpace(buf.String())
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		t.Fatalf("error output is not valid JSON: %v\noutput: %q", err, line)
	}
	if ok, _ := obj["ok"].(bool); ok {
		t.Errorf("ok should be false for error output")
	}
	if _, hasError := obj["error"]; !hasError {
		t.Errorf("error field missing in output: %s", line)
	}
}

// TestAgentRendererValue verifies RenderValue emits ok:true JSON with type and value.
func TestAgentRendererValue(t *testing.T) {
	rt := goja.New()
	r := &AgentRenderer{}
	var buf bytes.Buffer
	val := rt.ToValue(42)
	if err := r.RenderValue(&buf, val); err != nil {
		t.Fatalf("RenderValue error: %v", err)
	}
	line := strings.TrimSpace(buf.String())
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		t.Fatalf("value output is not valid JSON: %v\noutput: %q", err, line)
	}
	if ok, _ := obj["ok"].(bool); !ok {
		t.Errorf("ok should be true for value output")
	}
	if obj["type"] != "number" {
		t.Errorf("type = %q, want %q", obj["type"], "number")
	}
}

// TestAgentRendererEvalResult verifies RenderEvalResult includes elapsedMs.
func TestAgentRendererEvalResult(t *testing.T) {
	rt := goja.New()
	r := &AgentRenderer{}
	var buf bytes.Buffer
	val := rt.ToValue("hello")
	elapsed := 7 * time.Millisecond
	if err := r.RenderEvalResult(&buf, rt, val, nil, elapsed); err != nil {
		t.Fatalf("RenderEvalResult error: %v", err)
	}
	line := strings.TrimSpace(buf.String())
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, line)
	}
	if ms, _ := obj["elapsedMs"].(float64); ms != 7 {
		t.Errorf("elapsedMs = %v, want 7", obj["elapsedMs"])
	}
	if obj["type"] != "string" {
		t.Errorf("type = %q, want %q", obj["type"], "string")
	}
}

// TestAgentRendererTruncation verifies MaxOutputBytes causes truncated:true.
func TestAgentRendererTruncation(t *testing.T) {
	rt := goja.New()
	r := &AgentRenderer{MaxOutputBytes: 10}
	var buf bytes.Buffer
	// A long string will exceed the 10-byte limit.
	val := rt.ToValue("this is a very long string that exceeds the byte limit")
	if err := r.RenderValue(&buf, val); err != nil {
		t.Fatalf("RenderValue error: %v", err)
	}
	line := strings.TrimSpace(buf.String())
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, line)
	}
	if truncated, _ := obj["truncated"].(bool); !truncated {
		t.Errorf("truncated should be true when value exceeds MaxOutputBytes; output: %s", line)
	}
}

// ── Phase C: Capability and limits ───────────────────────────────────────────

// TestMergeReadOnly verifies mergeReplConfigFromMap sets ReadOnly from opts.
func TestMergeReadOnly(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{"readOnly": true})
	if !cfg.ReadOnly {
		t.Errorf("ReadOnly should be true")
	}
}

// TestMergeTimeoutMs verifies mergeReplConfigFromMap sets TimeoutMs from opts.
func TestMergeTimeoutMs(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{"timeoutMs": float64(5000)})
	if cfg.TimeoutMs != 5000 {
		t.Errorf("TimeoutMs = %d, want 5000", cfg.TimeoutMs)
	}
}

// TestMergeMaxRows verifies mergeReplConfigFromMap sets MaxRows from opts.
func TestMergeMaxRows(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{"maxRows": float64(500)})
	if cfg.MaxRows != 500 {
		t.Errorf("MaxRows = %d, want 500", cfg.MaxRows)
	}
}

// TestMergeMaxOutputBytes verifies mergeReplConfigFromMap sets MaxOutputBytes.
func TestMergeMaxOutputBytes(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{"maxOutputBytes": float64(32768)})
	if cfg.MaxOutputBytes != 32768 {
		t.Errorf("MaxOutputBytes = %d, want 32768", cfg.MaxOutputBytes)
	}
}

// TestMergeAgentProfileWithReadOnly verifies profile:"agent" + readOnly:true
// propagates ReadOnly into the Renderer and profile metadata.
func TestMergeAgentProfileWithReadOnly(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{
		"readOnly": true,
		"profile":  "agent",
	})
	if !cfg.ReadOnly {
		t.Errorf("ReadOnly should be true")
	}
	if cfg.Profile.Name != "agent" {
		t.Errorf("Profile.Name = %q, want agent", cfg.Profile.Name)
	}
	if ro, _ := cfg.Profile.Metadata["readOnly"].(bool); !ro {
		t.Errorf("Profile.Metadata[readOnly] should be true")
	}
}

// TestMergeMaxOutputBytesReconcile verifies that maxOutputBytes is propagated
// to an existing AgentRenderer when set alongside profile:"agent".
func TestMergeMaxOutputBytesReconcile(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{
		"profile":        "agent",
		"maxOutputBytes": float64(8192),
	})
	ar, ok := cfg.Renderer.(*AgentRenderer)
	if !ok {
		t.Fatalf("Renderer should be *AgentRenderer")
	}
	if ar.MaxOutputBytes != 8192 {
		t.Errorf("AgentRenderer.MaxOutputBytes = %d, want 8192", ar.MaxOutputBytes)
	}
}

// TestEvalWithTimeoutNormal verifies evalWithTimeout works without timeout.
func TestEvalWithTimeoutNormal(t *testing.T) {
	rt := goja.New()
	r := &Repl{rt: rt, cfg: defaultReplConfig()}
	val, err := r.evalWithTimeout("1 + 2", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.ToInteger() != 3 {
		t.Errorf("result = %v, want 3", val)
	}
}

// TestEvalWithTimeoutExpired verifies evalWithTimeout interrupts long execution.
func TestEvalWithTimeoutExpired(t *testing.T) {
	rt := goja.New()
	r := &Repl{rt: rt, cfg: defaultReplConfig()}
	// Infinite loop with a very small timeout should be interrupted.
	_, err := r.evalWithTimeout("var i=0; while(true){ i++; }", 50)
	if err == nil {
		t.Fatal("expected interrupt error, got nil")
	}
	var ie *goja.InterruptedError
	if !errors.As(err, &ie) {
		t.Errorf("expected *goja.InterruptedError, got %T: %v", err, err)
	}
}

// TestEvalWithTimeoutCleared verifies the runtime is usable after a timeout.
func TestEvalWithTimeoutCleared(t *testing.T) {
	rt := goja.New()
	r := &Repl{rt: rt, cfg: defaultReplConfig()}
	// First call: timeout
	r.evalWithTimeout("while(true){}", 50) //nolint:errcheck
	// Second call: should succeed
	val, err := r.evalWithTimeout("42", 0)
	if err != nil {
		t.Fatalf("runtime should be clear after timeout; err: %v", err)
	}
	if val.ToInteger() != 42 {
		t.Errorf("result = %v, want 42", val)
	}
}

// ── Phase D: Transcript and audit ────────────────────────────────────────────

// TestTranscriptWriterInput verifies WriteInput records an "input" event.
func TestTranscriptWriterInput(t *testing.T) {
	f, err := os.CreateTemp("", "transcript-*.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	tw := NewTranscriptWriter(WriterRenderer{}, f.Name(), "agent")
	tw.WriteInput("1 + 1")
	tw.WriteSessionEnd(nil)
	tw.Close() //nolint:errcheck

	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 1 {
		t.Fatalf("expected at least 1 line in transcript, got %d", len(lines))
	}
	var ev map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("transcript line is not valid JSON: %v\nline: %q", err, lines[0])
	}
	if ev["event"] != "input" {
		t.Errorf("event = %q, want 'input'", ev["event"])
	}
	if ev["input"] != "1 + 1" {
		t.Errorf("input = %q, want '1 + 1'", ev["input"])
	}
}

// TestSecretRedaction verifies that sensitive keys are masked in transcript output.
func TestSecretRedaction(t *testing.T) {
	r := &secretRedactor{}
	original := map[string]any{
		"host":     "localhost",
		"password": "supersecret",
		"token":    "abc123",
		"port":     5656,
		"nested": map[string]any{
			"key":   "private",
			"value": "ok",
		},
	}
	result := r.redact(original).(map[string]any)
	if result["host"] != "localhost" {
		t.Errorf("host should not be redacted")
	}
	if result["port"] != 5656 {
		t.Errorf("port should not be redacted")
	}
	if result["password"] != "[REDACTED]" {
		t.Errorf("password should be redacted, got %v", result["password"])
	}
	if result["token"] != "[REDACTED]" {
		t.Errorf("token should be redacted, got %v", result["token"])
	}
	nested := result["nested"].(map[string]any)
	if nested["key"] != "[REDACTED]" {
		t.Errorf("nested key should be redacted, got %v", nested["key"])
	}
	if nested["value"] != "ok" {
		t.Errorf("nested value should not be redacted")
	}
}

// TestTranscriptWriterNoopOnEmptyPath verifies no-op behavior without a path.
func TestTranscriptWriterNoopOnEmptyPath(t *testing.T) {
	inner := WriterRenderer{}
	tw := NewTranscriptWriter(inner, "", "default")
	// Should not panic; all write operations are no-ops.
	tw.WriteInput("test")
	tw.WriteSessionEnd(nil)
	if err := tw.Close(); err != nil {
		t.Errorf("Close() should return nil for no-op writer: %v", err)
	}
}

// TestMergeTranscriptPath verifies mergeReplConfigFromMap sets TranscriptPath.
func TestMergeTranscriptPath(t *testing.T) {
	base := defaultReplConfig()
	cfg := mergeReplConfigFromMap(base, map[string]interface{}{"transcript": "/tmp/test.ndjson"})
	if cfg.TranscriptPath != "/tmp/test.ndjson" {
		t.Errorf("TranscriptPath = %q, want %q", cfg.TranscriptPath, "/tmp/test.ndjson")
	}
}
