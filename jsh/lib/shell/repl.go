package shell

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

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
	}
}

func (repl *Repl) Loop(call goja.FunctionCall) goja.Value {
	cfg := repl.cfg
	writer := cfg.Writer
	if writer == nil {
		writer = colorable.NewColorableStdout()
	}
	renderer := cfg.Renderer
	if renderer == nil {
		renderer = newConsoleRenderer(repl.rt)
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
		if err := ses.Stop(loopErr); err != nil {
			ses.Renderer.RenderError(ses.Writer, err) //nolint:errcheck
		}
	}()
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
			if val, exit := repl.dispatchSlashCommand(cmd, args, ses.Writer); exit {
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

		val, evalErr := repl.rt.RunString(input)
		if evalErr != nil {
			ses.Renderer.RenderError(ses.Writer, evalErr) //nolint:errcheck
		} else if val != nil && val != goja.Null() && val != goja.Undefined() {
			ses.Renderer.RenderValue(ses.Writer, val) //nolint:errcheck
		}
	}
	return repl.rt.ToValue(0)
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
// string literal detection (single-quote, double-quote, backtick). Empty or
// whitespace-only input is never considered complete.
//
// Limitations: comments containing brackets and ${...} inside template literals
// are not parsed precisely; these edge cases are deferred to Phase 3.
func isJSInputComplete(lines []string) bool {
	code := strings.Join(lines, "\n")
	if strings.TrimSpace(code) == "" {
		return false
	}
	var depth int
	var inString rune
	var prev rune
	for _, ch := range code {
		if inString != 0 {
			if ch == inString && prev != '\\' {
				inString = 0
			}
		} else {
			switch ch {
			case '{', '(', '[':
				depth++
			case '}', ')', ']':
				depth--
			case '\'', '"', '`':
				inString = ch
			}
		}
		prev = ch
	}
	return depth <= 0 && inString == 0
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
