package shell

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
	jshrl "github.com/machbase/neo-server/v8/jsh/native/readline"
	"github.com/mattn/go-colorable"
)

type Repl struct {
	rt      *goja.Runtime
	history *jshrl.History
}

func repl(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		repl := &Repl{
			rt:      rt,
			history: jshrl.NewHistory("repl_history", 100),
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
	"  \033[32m\\quit\033[0m, \033[32m\\q\033[0m  - Exit REPL\n" +
	"  \033[32m\\r\033[0m         - Run script (at the last line of script)\n"

func (repl *Repl) Loop(call goja.FunctionCall) goja.Value {
	var ed multiline.Editor
	ed.SetTty(NewTty()) // See TtyWrap comment
	ed.SetPrompt(repl.prompt)
	ed.SetWriter(colorable.NewColorableStdout())
	ed.SubmitOnEnterWhen(repl.submitOnEnterWhen)
	ed.SetHistory(repl.history)
	ed.SetHistoryCycling(true)
	ctx := context.Background()
	repl.println(repl.rt.ToValue(replBanner))
	for {
		var input string
		if lines, err := ed.Read(ctx); err != nil {
			break
		} else {
			last := lines[len(lines)-1]
			switch last {
			case "\\r": // run script
				// remove the last line and
				input = strings.Join(lines, "\n")
				repl.history.Add(input)
				input = strings.TrimSuffix(input, "\\r")
			case "\\exit", "\\q", "\\quit":
				return repl.rt.ToValue(0)
			}
		}
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
		return w.Write([]byte(fmt.Sprintf("%04d  ", lineNo)))
	}
}

// regular expression to match repl command that starts with \{command}
var replCommandRegex = regexp.MustCompile(`^\\([a-zA-Z]+)(\s+.*)?$`)

func (repl *Repl) submitOnEnterWhen(lines []string, lineNo int) bool {
	return replCommandRegex.MatchString(lines[lineNo])
}

func (repl *Repl) println(vals ...goja.Value) {
	console := repl.rt.Get("console").(*goja.Object)
	print, _ := goja.AssertFunction(console.Get("println"))
	print(goja.Undefined(), vals...)
}
