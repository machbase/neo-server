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

type Repl struct {
	rt      *goja.Runtime
	history SessionHistory
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

func repl(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		repl := &Repl{
			rt:      rt,
			history: NewHistory(HistoryConfig{Name: "repl_history", Size: 100, Enabled: true}),
		}
		obj := rt.NewObject()
		obj.Set("loop", repl.Loop)
		return obj
	}
}

const replBanner = "\033[1;36m╔══════════════════════════════════════╗\n" +
	"║     Welcome to \033[1;35mJSH REPL\033[1;36m              ║\n" +
	"╚══════════════════════════════════════╝\033[0m\n" +
	"\033[33mCommands:\033[0m\n" +
	"  \033[32m\\quit\033[0m, \033[32m\\q\033[0m  - Exit REPL\n"

func (repl *Repl) Loop(call goja.FunctionCall) goja.Value {
	ses := NewEditorSession(SessionConfig{
		Writer:               colorable.NewColorableStdout(),
		EnableHistoryCycling: true,
		History: HistoryConfig{
			Name:    "repl_history",
			Size:    100,
			Enabled: true,
		},
		Hooks: SessionHooks{
			Prompt:            repl.prompt,
			SubmitOnEnterWhen: repl.submitOnEnterWhen,
		},
	})
	ed := ses.Editor
	repl.history = ses.History
	ctx := context.Background()
	repl.println(repl.rt.ToValue(replBanner))
	for {
		var input string
		if lines, err := ed.Read(ctx); err != nil {
			break
		} else {
			last := lines[len(lines)-1]
			if strings.HasPrefix(last, "\\") {
				switch last {
				case "\\exit", "\\q", "\\quit":
					return repl.rt.ToValue(0)
				}
			} else {
				if strings.HasSuffix(strings.TrimSpace(last), ";") {
					input = strings.Join(lines, "\n")
					repl.history.Add(input)
				}
			}
		}

		// JavaScript evaluation is Repl-only and is intentionally excluded from
		// shared foundation extraction with Shell.
		val, err := repl.rt.RunString(input)
		if err != nil {
			repl.println(repl.rt.NewGoError(err))
		} else {
			if val != nil && val != goja.Null() && val != goja.Undefined() {
				repl.println(val)
			} else {
				repl.println()
			}
		}
	}
	return repl.rt.ToValue(0)
}

func (repl *Repl) prompt(w io.Writer, lineNo int) (int, error) {
	if lineNo == 0 {
		return w.Write([]byte("repl> "))
	} else {
		return w.Write([]byte(fmt.Sprintf("%04d  ", lineNo+1)))
	}
}

// regular expression to match repl command that starts with \{command}
var replCommandRegex = regexp.MustCompile(`^\\([a-zA-Z]+)(\s+.*)?$`)

// javascript statement
var replStatementRegexp = regexp.MustCompile(`^[\s\S]*;[\s]*$`)

func (repl *Repl) submitOnEnterWhen(lines []string, lineNo int) bool {
	return replCommandRegex.MatchString(lines[lineNo]) || replStatementRegexp.MatchString(lines[lineNo])
}

func (repl *Repl) println(vals ...goja.Value) {
	console := repl.rt.Get("console").(*goja.Object)
	print, _ := goja.AssertFunction(console.Get("println"))
	print(goja.Undefined(), vals...)
}
