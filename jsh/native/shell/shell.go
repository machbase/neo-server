package shell

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
	"github.com/hymkor/go-multiline-ny/completion"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/log"
	jshrl "github.com/machbase/neo-server/v8/jsh/native/readline"
	"github.com/machbase/neo-server/v8/jsh/native/shell/internal"
	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/keys"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)

	// shell = new Shell()
	o.Set("Shell", shell(rt))
	o.Set("Repl", repl(rt))
}

func shell(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		shell := &Shell{
			rt:      rt,
			history: jshrl.NewHistory("history", 100),
		}

		obj := rt.NewObject()
		obj.Set("run", shell.Run)
		return obj
	}
}

type Shell struct {
	rt      *goja.Runtime
	history *jshrl.History
}

var banner = "\n" +
	"\x1B[93m     ██╗ ███████╗ ██╗  ██╗" + "\n" +
	"\x1B[92m     ██║ ██╔════╝ ██║  ██║" + "\n" +
	"\x1B[96m     ██║ ███████╗ ███████║" + "\n" +
	"\x1B[94m██   ██║ ╚════██║ ██╔══██║" + "\n" +
	"\x1B[95m╚█████╔╝ ███████║ ██║  ██║" + "\n" +
	"\x1B[91m ╚════╝  ╚══════╝ ╚═╝  ╚═╝" + "\n" +
	"\x1B[0m" + "\n"

var betaWarn = "" +
	"    This is a JSH command-line runtime in BETA stage.\n" +
	"    Commands and features are subject to change without notice.\n" +
	"    Enter 'exit' to quit the shell.\n"

func (sh *Shell) Run(env *engine.Env) int {
	var ed multiline.Editor
	ed.SetTty(NewTty()) // See TtyWrap comment
	ed.SetPrompt(sh.prompt(env))
	ed.SubmitOnEnterWhen(sh.submitOnEnterWhen)
	ed.SetWriter(colorable.NewColorableStdout())
	ed.SetHistory(sh.history)
	ed.SetHistoryCycling(true)
	ed.SetPredictColor([...]string{"\x1B[3;22;30m", "\x1B[23;39m"}) // dark gray, italic
	ed.ResetColor = "\x1B[0m"
	ed.DefaultColor = "\x1B[37;49;1m"

	// enable completion
	ed.BindKey(keys.CtrlI, &completion.CmdCompletionOrList{
		Delimiter:  "&|><",
		Enclosure:  `"'`,
		Postfix:    " ",
		Candidates: sh.getCompletionCandidates,
	})
	ctx := context.Background()
	log.Println(banner)
	log.Println(betaWarn)
	for {
		var line string
		var forHistory string
		if input, err := ed.Read(ctx); err != nil {
			if err == readline.CtrlC || err == io.EOF {
				log.Println(err.Error())
				continue
			}
			log.Printf("Error input: %v\n", err)
			return 1
		} else {
			forHistory = strings.Join(input, "\n")
			for i, ln := range input {
				input[i] = strings.TrimSuffix(ln, `\`)
			}
			line = strings.Join(input, "")
		}

		// expand environment variables in the line
		line = env.Expand(line)
		if _, alive := sh.process(line); !alive {
			return 0
		}
		// this makes to prevent adding 'exit' command to history
		sh.history.Add(forHistory)
	}
}

func (sh *Shell) prompt(env *engine.Env) func(w io.Writer, lineNo int) (int, error) {
	return func(w io.Writer, lineNo int) (int, error) {
		dir := ""
		if v := env.Get("PWD"); v != nil {
			if s, ok := v.(string); ok {
				dir = s
			}
		}
		if lineNo == 0 {
			return w.Write([]byte(fmt.Sprintf("\x1b[34m%s\x1B[31m >\x1B[0m ", dir)))
		} else {
			return w.Write([]byte(fmt.Sprintf("%s   ", strings.Repeat(" ", len(dir)))))
		}
	}
}

func (sh *Shell) submitOnEnterWhen(lines []string, _ int) bool {
	return !strings.HasSuffix(lines[len(lines)-1], `\`)
}

func (sh *Shell) getCompletionCandidates(fields []string) (forCompletion []string, forListing []string) {
	return
}

// if return false, exit shell
func (sh *Shell) process(line string) (int, bool) {
	// Parse the command
	cmd := parseCommand(line)

	for _, stmt := range cmd.Statements {
		var stopOnError bool
		if stmt.Operator == "&&" {
			stopOnError = true
		}
		for _, pipe := range stmt.Pipelines {
			if pipe.Command == "exit" || pipe.Command == "quit" {
				return 0, false
			}

			// internal commands that execute in the SAME runtime instance
			// others are executed via exec function on the separate runtime process.
			var returnValue goja.Value
			if v, ok := internal.Run(sh.rt, pipe.Command, pipe.Args...); ok {
				returnValue = v
			} else {
				cmd := pipe.Command
				if !strings.HasSuffix(cmd, ".js") {
					cmd += ".js"
				}
				returnValue = sh.exec(cmd, pipe.Args)
			}

			exitCode := -1
			if returnValue != nil {
				switch v := returnValue.Export().(type) {
				default:
					returnStr := returnValue.String()
					returnStr = strings.TrimPrefix(returnStr, "Error: ")
					log.Println(returnStr)
				case int64:
					exitCode = int(v)
				}
			}
			if exitCode != 0 && stopOnError {
				return exitCode, true
			}
		}
	}
	return 0, true
}

func (sh *Shell) exec(command string, args []string) goja.Value {
	parts := []string{}
	for _, arg := range args {
		parts = append(parts, fmt.Sprintf("%q", arg))
	}
	str := strings.Join(parts, ", ")

	val, err := sh.rt.RunString(fmt.Sprintf(`(()=>{
		const {exec, which} = require("process");
		const path = which('%s');
		if (!path || path === "") {
			throw new Error("command not found: " + %q);
		}
		return exec(path, %s);
	})()`, command, command, str))

	if err != nil {
		if jsErr, ok := err.(*goja.Exception); ok {
			return jsErr.Value()
		}
		panic(err)
	}
	return val
}
