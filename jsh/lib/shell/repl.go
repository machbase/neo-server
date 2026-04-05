package shell

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/mattn/go-colorable"
)

// ReplConfig holds the per-instance configuration for a Repl.
// The zero value is valid; defaultReplConfig() returns pre-populated defaults.
type ReplConfig struct {
	Profile              RuntimeProfile
	History              HistoryConfig
	Writer               io.Writer
	Renderer             Renderer
	EnableHistoryCycling bool

	// Phase 2: non-interactive and startup options.
	// When Eval is non-empty, Loop() runs the code and returns without
	// entering the interactive loop. PrintEval=true prints the result.
	Eval           string
	PrintEval      bool
	PreloadModules []string // require(mod) before interactive loop
	PreloadFiles   []string // eval file content before interactive loop

	// Phase C: operational limits and capability controls.
	// These are enforced by the agent profile and AgentRenderer.
	ReadOnly       bool  // deny write operations in agent helpers (agent.db.exec)
	TimeoutMs      int64 // per-evaluation timeout in milliseconds; 0 = no limit
	MaxRows        int   // maximum rows per query; 0 = use profile default (1000)
	MaxOutputBytes int   // maximum serialized output bytes; 0 = no limit

	// Phase D: transcript and audit.
	// When TranscriptPath is non-empty, all inputs and results are recorded
	// as newline-delimited JSON to the specified file.
	TranscriptPath string
}

func defaultReplConfig() ReplConfig {
	return ReplConfig{
		Profile:              defaultReplProfile(),
		History:              HistoryConfig{Name: "repl_history", Size: 100, Enabled: true},
		EnableHistoryCycling: true,
	}
}

// Repl is the JavaScript runtime console product of this package.
//
// Repl-specific responsibilities stay here even when editor/session plumbing
// is extracted into shared foundation files:
//   - JavaScript source accumulation
//   - completeness checks for submitted source
//   - evaluation via goja runtime
//   - expression result rendering
//   - slash command semantics
//
// Shared editor/history/profile/render/bootstrap hooks may move out in later
// phases, but Repl.Loop() evaluation semantics remain intentionally separate
// from Shell command execution.
type Repl struct {
	rt   *goja.Runtime
	cfg  ReplConfig
	cmds []*slashEntry
}

func repl(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		r := &Repl{
			rt:  rt,
			cfg: defaultReplConfig(),
		}
		r.registerBuiltinCommands()
		obj := rt.NewObject()
		obj.Set("loop", r.Loop)
		return obj
	}
}

const replBanner = "\033[1;36m╔══════════════════════════════════════╗\n" +
	"║     Welcome to \033[1;35mJSH REPL\033[1;36m              ║\n" +
	"╚══════════════════════════════════════╝\033[0m\n" +
	"Type \033[32m\\help\033[0m for available commands.\n" +
	"Constraints: no 'await', no 'import'. Use require('module').\n"

func defaultReplProfile() RuntimeProfile {
	return RuntimeProfile{
		Name:        "default",
		Description: "JSH JavaScript REPL",
		Banner: func() string {
			return replBanner
		},
		Metadata: map[string]any{
			"product": "repl",
			"mode":    "javascript",
		},
		KnownModules: []string{
			"process          — environment variables, argv, exit()",
			"fs               — file system (readFileSync, writeFileSync, ...)",
			"path             — path utilities (join, dirname, basename, ...)",
			"os               — OS interfaces (hostname, platform, ...)",
			"events           — EventEmitter",
			"stream           — Readable, Writable, Transform, pipeline",
			"util/parseArgs   — CLI argument parser",
			"@jsh/shell       — shell and REPL constructors (this module)",
			"@jsh/process     — native process utilities",
			"@jsh/session     — session and connection info",
		},
	}
}

// userReplProfile returns a RuntimeProfile enriched with Machbase Neo helpers.
// It preloads repl/profiles/user and exposes it as the global `user` object.
func userReplProfile() RuntimeProfile {
	base := defaultReplProfile()
	return RuntimeProfile{
		Name:        "user",
		Description: "JSH REPL with Machbase Neo helpers",
		Banner:      base.Banner,
		Startup: func(rt *goja.Runtime) error {
			_, err := rt.RunString(`globalThis.user = require('repl/profiles/user');`)
			return err
		},
		Metadata: map[string]any{
			"product": "repl",
			"mode":    "javascript",
			"profile": "user",
		},
		KnownModules: append(base.KnownModules,
			"repl/profiles/user  — user.db.* query helpers (this profile)",
		),
	}
}

// agentProfileConfig holds the capability and resource limit parameters
// injected into the JS runtime before the agent module is loaded.
type agentProfileConfig struct {
	ReadOnly       bool
	MaxRows        int
	MaxOutputBytes int
}

// agentReplProfile returns the default agent RuntimeProfile (no limits overrides).
func agentReplProfile() RuntimeProfile {
	return agentReplProfileWith(agentProfileConfig{
		MaxRows:        1000,
		MaxOutputBytes: 65536,
	})
}

// agentReplProfileWith returns a RuntimeProfile configured for LLM agent use
// with the given capability and limit settings. All output is structured JSON
// via AgentRenderer; no ANSI banner is emitted.
func agentReplProfileWith(cfg agentProfileConfig) RuntimeProfile {
	base := defaultReplProfile()
	return RuntimeProfile{
		Name:        "agent",
		Description: "JSH REPL for LLM agent use — structured JSON output",
		Banner:      nil, // no banner; AgentRenderer emits JSON events
		Startup: func(rt *goja.Runtime) error {
			// Inject limits before requiring agent module so it can read them.
			if err := rt.Set("__agentConfig", map[string]any{
				"readOnly":       cfg.ReadOnly,
				"maxRows":        cfg.MaxRows,
				"maxOutputBytes": cfg.MaxOutputBytes,
			}); err != nil {
				return err
			}
			_, err := rt.RunString(`globalThis.agent = require('repl/profiles/agent');`)
			return err
		},
		Metadata: map[string]any{
			"product":        "repl",
			"mode":           "javascript",
			"profile":        "agent",
			"readOnly":       cfg.ReadOnly,
			"maxRows":        cfg.MaxRows,
			"maxOutputBytes": cfg.MaxOutputBytes,
		},
		KnownModules: append(base.KnownModules,
			"repl/profiles/agent  — agent.db.*, agent.schema.*, agent.runtime.*",
		),
	}
}

func (repl *Repl) Loop(call goja.FunctionCall) goja.Value {
	cfg := repl.cfg
	// Read optional JS config object passed from repl.js (Phase 2).
	if len(call.Arguments) > 0 {
		if obj, ok := call.Arguments[0].Export().(map[string]interface{}); ok {
			cfg = mergeReplConfigFromMap(cfg, obj)
		}
	}

	writer := cfg.Writer
	if writer == nil {
		writer = colorable.NewColorableStdout()
	}
	renderer := cfg.Renderer
	if renderer == nil {
		renderer = newConsoleRenderer(repl.rt)
	}

	// Phase D: wrap renderer with transcript recording when a path is configured.
	var tw *TranscriptWriter
	if cfg.TranscriptPath != "" {
		tw = NewTranscriptWriter(renderer, cfg.TranscriptPath, cfg.Profile.Name)
		renderer = tw
	}

	// Eval-and-exit mode: run code without entering interactive loop.
	if cfg.Eval != "" {
		return repl.runEval(cfg.Eval, cfg.PrintEval, writer, renderer, cfg.TimeoutMs)
	}

	ses := NewEditorSession(SessionConfig{
		Writer:               writer,
		EnableHistoryCycling: cfg.EnableHistoryCycling,
		History:              cfg.History,
		Profile:              cfg.Profile,
		Renderer:             renderer,
		Hooks: SessionHooks{
			Prompt:            repl.prompt,
			SubmitOnEnterWhen: repl.submitOnEnterWhen,
		},
	})
	if err := ses.Start(repl.rt); err != nil {
		ses.Renderer.RenderError(ses.Writer, err) //nolint:errcheck
		return repl.rt.ToValue(1)
	}
	var loopErr error
	defer func() {
		if tw != nil {
			tw.WriteSessionEnd(loopErr)
			tw.Close() //nolint:errcheck
		}
		if err := ses.Stop(loopErr); err != nil {
			ses.Renderer.RenderError(ses.Writer, err) //nolint:errcheck
		}
	}()

	// Run preloads after profile startup (ses.Start already ran Profile.Startup).
	if err := repl.runPreloads(cfg, ses.Writer, renderer); err != nil {
		return repl.rt.ToValue(1)
	}

	// Capture the global namespace after profile startup and preloads.
	// This snapshot is used by \reset to identify user-defined globals.
	initialGlobals := make(map[string]struct{})
	for _, k := range repl.rt.GlobalObject().Keys() {
		initialGlobals[k] = struct{}{}
	}
	sctx := slashCtx{
		Writer:         ses.Writer,
		History:        ses.History,
		Profile:        ses.Profile,
		InitialGlobals: initialGlobals,
	}

	ctx := context.Background()
	ses.Renderer.RenderBanner(ses.Writer, ses.Profile) //nolint:errcheck
	for {
		lines, err := ses.Editor.Read(ctx)
		if err != nil {
			loopErr = err
			break
		}

		last := lines[len(lines)-1]

		// Slash command dispatch — handled before JavaScript evaluation.
		if replCommandRegex.MatchString(last) {
			cmd, args := parseSlashInput(last)
			if val, exit := repl.dispatchSlashCommand(cmd, args, sctx); exit {
				return val
			}
			continue
		}

		// JavaScript evaluation is Repl-only and is intentionally excluded from
		// shared foundation extraction with Shell.
		input := strings.Join(lines, "\n")
		if strings.TrimSpace(input) == "" {
			continue
		}
		ses.History.Add(input)

		// Phase D: record input before evaluation.
		if tw != nil {
			tw.WriteInput(input)
		}

		start := time.Now()
		val, evalErr := repl.evalWithTimeout(input, cfg.TimeoutMs)
		elapsed := time.Since(start)
		renderEvalResult(repl.rt, renderer, ses.Writer, val, evalErr, elapsed)
	}
	return repl.rt.ToValue(0)
}

// runEval evaluates code non-interactively and returns the exit value.
func (repl *Repl) runEval(code string, printResult bool, w io.Writer, renderer Renderer, timeoutMs int64) goja.Value {
	start := time.Now()
	val, err := repl.evalWithTimeout(code, timeoutMs)
	elapsed := time.Since(start)
	if err != nil {
		renderer.RenderError(w, err) //nolint:errcheck
		return repl.rt.ToValue(1)
	}
	// AgentRenderer always outputs a structured result; human mode respects printResult.
	_, isAgent := renderer.(*AgentRenderer)
	if printResult || isAgent {
		renderEvalResult(repl.rt, renderer, w, val, nil, elapsed)
	}
	return repl.rt.ToValue(0)
}

// evalWithTimeout runs code and, when timeoutMs > 0, schedules a goja
// Interrupt after the limit. The interrupted state is always cleared on return
// so subsequent evaluations are unaffected.
func (repl *Repl) evalWithTimeout(code string, timeoutMs int64) (goja.Value, error) {
	if timeoutMs <= 0 {
		return repl.rt.RunString(code)
	}
	timer := time.AfterFunc(time.Duration(timeoutMs)*time.Millisecond, func() {
		repl.rt.Interrupt("execution timeout")
	})
	val, err := repl.rt.RunString(code)
	timer.Stop()
	repl.rt.ClearInterrupt()
	return val, err
}

// renderEvalResult routes an evaluation result through the appropriate renderer.
// When renderer is *AgentRenderer, elapsed time is forwarded for structured JSON.
// For all other renderers the standard RenderValue/RenderError path is used.
func renderEvalResult(rt *goja.Runtime, renderer Renderer, w io.Writer, val goja.Value, evalErr error, elapsed time.Duration) {
	if ar, ok := renderer.(*AgentRenderer); ok {
		ar.RenderEvalResult(w, rt, val, evalErr, elapsed) //nolint:errcheck
		return
	}
	if evalErr != nil {
		renderer.RenderError(w, evalErr) //nolint:errcheck
		return
	}
	if val != nil && val != goja.Null() && val != goja.Undefined() {
		renderer.RenderValue(w, val) //nolint:errcheck
	}
}

// runPreloads requires modules and loads files before the interactive loop.
func (repl *Repl) runPreloads(cfg ReplConfig, w io.Writer, renderer Renderer) error {
	requireFn, ok := goja.AssertFunction(repl.rt.Get("require"))
	if !ok {
		return nil // require not available; skip silently
	}
	for _, mod := range cfg.PreloadModules {
		if _, err := requireFn(goja.Undefined(), repl.rt.ToValue(mod)); err != nil {
			renderer.RenderError(w, fmt.Errorf("require(%q): %w", mod, err)) //nolint:errcheck
			return err
		}
	}
	for _, file := range cfg.PreloadFiles {
		// Read the file via the JSH fs module to respect the VFS mount points.
		contentVal, err := repl.rt.RunString(
			"require('fs').readFileSync(" + jsQuote(file) + ",'utf8')")
		if err != nil {
			renderer.RenderError(w, fmt.Errorf("load(%q): %w", file, err)) //nolint:errcheck
			return err
		}
		if _, err := repl.rt.RunScript(file, contentVal.String()); err != nil {
			renderer.RenderError(w, fmt.Errorf("load(%q): %w", file, err)) //nolint:errcheck
			return err
		}
	}
	return nil
}

// mergeReplConfigFromMap merges a JS options map into a ReplConfig.
// Unknown keys are silently ignored.
// Processing order: scalar limits first, then profile (which reads limits), then json flag.
func mergeReplConfigFromMap(base ReplConfig, opts map[string]interface{}) ReplConfig {
	cfg := base
	if v, ok := opts["eval"]; ok {
		if s, ok := v.(string); ok {
			cfg.Eval = s
		}
	}
	if v, ok := opts["print"]; ok {
		if b, ok := v.(bool); ok {
			cfg.PrintEval = b
		}
	}
	if v, ok := opts["noHistory"]; ok {
		if b, ok := v.(bool); ok && b {
			cfg.History.Enabled = false
		}
	}
	if v, ok := opts["load"]; ok {
		cfg.PreloadFiles = appendStrings(cfg.PreloadFiles, v)
	}
	if v, ok := opts["require"]; ok {
		cfg.PreloadModules = appendStrings(cfg.PreloadModules, v)
	}
	// Phase C: process operational limits before profile so they are available
	// when building agentProfileConfig inside the "agent" profile case.
	if v, ok := opts["readOnly"]; ok {
		if b, ok := v.(bool); ok {
			cfg.ReadOnly = b
		}
	}
	if v, ok := opts["timeoutMs"]; ok {
		cfg.TimeoutMs = toInt64(v)
	}
	if v, ok := opts["maxRows"]; ok {
		if n := toInt64(v); n > 0 {
			cfg.MaxRows = int(n)
		}
	}
	if v, ok := opts["maxOutputBytes"]; ok {
		if n := toInt64(v); n > 0 {
			cfg.MaxOutputBytes = int(n)
		}
	}
	if v, ok := opts["transcript"]; ok {
		if s, ok := v.(string); ok && s != "" {
			cfg.TranscriptPath = s
		}
	}
	if v, ok := opts["profile"]; ok {
		if s, ok := v.(string); ok {
			switch s {
			case "user":
				cfg.Profile = userReplProfile()
			case "agent":
				agentCfg := agentProfileConfig{
					ReadOnly:       cfg.ReadOnly,
					MaxRows:        cfg.MaxRows,
					MaxOutputBytes: cfg.MaxOutputBytes,
				}
				if agentCfg.MaxRows == 0 {
					agentCfg.MaxRows = 1000
				}
				if agentCfg.MaxOutputBytes == 0 {
					agentCfg.MaxOutputBytes = 65536
				}
				cfg.Profile = agentReplProfileWith(agentCfg)
				cfg.Renderer = &AgentRenderer{MaxOutputBytes: agentCfg.MaxOutputBytes}
			}
		}
	}
	if v, ok := opts["json"]; ok {
		if b, ok := v.(bool); ok && b {
			maxOut := cfg.MaxOutputBytes
			if maxOut == 0 {
				maxOut = 65536
			}
			cfg.Renderer = &AgentRenderer{MaxOutputBytes: maxOut}
		}
	}
	// Reconcile: if Renderer is AgentRenderer, keep MaxOutputBytes in sync.
	if ar, ok := cfg.Renderer.(*AgentRenderer); ok && cfg.MaxOutputBytes > 0 {
		ar.MaxOutputBytes = cfg.MaxOutputBytes
	}
	if v, ok := opts["historyName"]; ok {
		if s, ok := v.(string); ok && s != "" {
			cfg.History.Name = s
		}
	}
	return cfg
}

// toInt64 converts a numeric interface value to int64.
// Handles float64 (JS numbers from goja) and int64.
func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}

// appendStrings appends string(s) from v (string or []interface{}) to dst.
func appendStrings(dst []string, v interface{}) []string {
	switch t := v.(type) {
	case string:
		if t != "" {
			dst = append(dst, t)
		}
	case []interface{}:
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				dst = append(dst, s)
			}
		}
	}
	return dst
}

// jsQuote returns a JSON-safe double-quoted string for embedding in JS code.
func jsQuote(s string) string {
	// Use Go's %q which produces a Go-quoted string. Convert to JS-safe by
	// replacing only the delimiters — both use the same escape sequences for
	// printable characters and common escapes (\n, \t, \\, \").
	return fmt.Sprintf("%q", s)
}

func (repl *Repl) prompt(w io.Writer, lineNo int) (int, error) {
	if lineNo == 0 {
		return w.Write([]byte("repl> "))
	}
	return w.Write([]byte(fmt.Sprintf("%04d  ", lineNo+1)))
}

// replCommandRegex matches a slash command line: \command or \command args.
var replCommandRegex = regexp.MustCompile(`^\\([a-zA-Z]+)(\s+.*)?$`)

// submitOnEnterWhen returns true when the accumulated input is ready to be
// submitted for evaluation. A slash command on the current line submits
// immediately. Otherwise the full JavaScript input is checked for syntactic
// completeness via bracket/string balance.
func (repl *Repl) submitOnEnterWhen(lines []string, lineNo int) bool {
	if replCommandRegex.MatchString(lines[lineNo]) {
		return true
	}
	return isJSInputComplete(lines[:lineNo+1])
}

// isJSInputComplete reports whether the accumulated JavaScript lines form a
// syntactically complete input that can be submitted for evaluation.
//
// Completeness is determined by bracket balance ({}, (), []) and unclosed
// string literal detection (single-quote, double-quote, backtick). Single-line
// (//) and block (/* */) comments are skipped so brackets inside them do not
// affect the depth count. Empty or whitespace-only input is never complete.
//
// Limitation: ${...} expressions inside template literals (backtick strings)
// are not recursively parsed; nested brackets there may give false results.
func isJSInputComplete(lines []string) bool {
	code := strings.Join(lines, "\n")
	if strings.TrimSpace(code) == "" {
		return false
	}
	runes := []rune(code)
	n := len(runes)
	var depth int
	var inString rune
	inLineComment := false
	inBlockComment := false

	for i := 0; i < n; i++ {
		ch := runes[i]

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}

		if inBlockComment {
			if ch == '*' && i+1 < n && runes[i+1] == '/' {
				inBlockComment = false
				i++ // consume '/'
			}
			continue
		}

		if inString != 0 {
			if ch == inString {
				// Count preceding backslashes to detect escaping (\\" is not escaped).
				escCount := 0
				for j := i - 1; j >= 0 && runes[j] == '\\'; j-- {
					escCount++
				}
				if escCount%2 == 0 {
					inString = 0
				}
			}
			continue
		}

		// Detect comment starts before bracket/string handling.
		if ch == '/' && i+1 < n {
			if runes[i+1] == '/' {
				inLineComment = true
				i++
				continue
			}
			if runes[i+1] == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		switch ch {
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
		case '\'', '"', '`':
			inString = ch
		}
	}
	return depth <= 0 && inString == 0 && !inBlockComment
}

// consoleRenderer is the Repl-specific Renderer implementation. It delegates
// to the JSH console module (console.println) for value formatting, preserving
// existing output behavior while wiring output through the shared Renderer hook.
//
// The io.Writer parameter in each method is intentionally not used: the console
// module manages its own output destination. The parameter is part of the
// shared Renderer interface contract and may be used by future renderers.
type consoleRenderer struct {
	rt *goja.Runtime
}

func newConsoleRenderer(rt *goja.Runtime) Renderer {
	return &consoleRenderer{rt: rt}
}

func (r *consoleRenderer) RenderBanner(w io.Writer, profile RuntimeProfile) error {
	msg := profile.ResolveBanner()
	if msg == "" {
		return nil
	}
	r.consolePrintln(r.rt.ToValue(msg))
	return nil
}

func (r *consoleRenderer) RenderValue(w io.Writer, value any) error {
	if value == nil {
		return nil
	}
	if v, ok := value.(goja.Value); ok {
		r.consolePrintln(v)
	} else {
		r.consolePrintln(r.rt.ToValue(value))
	}
	return nil
}

func (r *consoleRenderer) RenderError(w io.Writer, err error) error {
	if err == nil {
		return nil
	}
	r.consolePrintln(r.rt.NewGoError(err))
	return nil
}

func (r *consoleRenderer) consolePrintln(vals ...goja.Value) {
	console := r.rt.Get("console").(*goja.Object)
	print, _ := goja.AssertFunction(console.Get("println"))
	print(goja.Undefined(), vals...)
}
