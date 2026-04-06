package shell

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"strings"

	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
	"github.com/hymkor/go-multiline-ny/completion"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/log"
	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/keys"
)

//go:embed profile_user.js
var userJS []byte

//go:embed profile_agent.js
var agentJS []byte

//go:embed ai_prompt.js
var aiPromptJS []byte

//go:embed ai_executor.js
var aiExecutorJS []byte

// Files returns JavaScript library files embedded in this package.
// They are mounted under /lib inside the virtual file system:
//   - require('repl/profiles/user')  — human operator helpers
//   - require('repl/profiles/agent') — agent/machine-readable helpers
//   - require('ai/prompt')           — LLM system prompt assembler
//   - require('ai/executor')         — jsh code block extractor and executor
func Files() map[string][]byte {
	return map[string][]byte{
		"repl/profiles/user.js":  userJS,
		"repl/profiles/agent.js": agentJS,
		"ai/prompt.js":           aiPromptJS,
		"ai/executor.js":         aiExecutorJS,
	}
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)

	// shell = new Shell()
	o.Set("Shell", shell(rt))
	o.Set("Repl", repl(rt))
	registerAIModule(rt, o)
}

func shell(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		shell := &Shell{
			rt: rt,
		}

		obj := rt.NewObject()
		obj.Set("run", shell.Run)
		return obj
	}
}

type Shell struct {
	rt      *goja.Runtime
	env     *engine.Env
	history SessionHistory
}

// Shell is the command interpreter product of this package.
//
// Shell-specific responsibilities stay here even when editor/session plumbing
// is extracted into shared foundation files:
//   - command parsing
//   - statement and operator handling
//   - alias expansion
//   - process execution
//   - shell command completion
//
// Shared editor/history/profile/render/bootstrap hooks may move out in later
// phases, but Shell.process(), Shell.exec(), and the statement model remain
// intentionally separate from the JavaScript REPL product.

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
	sh.env = env
	ses := NewEditorSession(SessionConfig{
		Writer:               colorable.NewColorableStdout(),
		EnableHistoryCycling: true,
		History: HistoryConfig{
			Name:    "history",
			Size:    100,
			Enabled: true,
		},
		Profile: defaultShellProfile(),
		Hooks: SessionHooks{
			Prompt:            sh.prompt(env),
			SubmitOnEnterWhen: sh.submitOnEnterWhen,
			ConfigureEditor:   sh.configureEditorSession,
		},
	})
	ed := ses.Editor
	sh.history = ses.History
	if err := ses.Start(sh.rt); err != nil {
		log.Printf("Error starting session: %v\n", err)
		return 1
	}
	var loopErr error
	defer func() {
		if err := ses.Stop(loopErr); err != nil {
			log.Printf("Error stopping session: %v\n", err)
		}
	}()
	ctx := context.Background()
	if msg := ses.Banner(); msg != "" {
		log.Print(msg)
	}
	for {
		var line string
		var forHistory string
		if input, err := ed.Read(ctx); err != nil {
			if err == readline.CtrlC || err == io.EOF {
				log.Println(err.Error())
				continue
			}
			loopErr = err
			log.Printf("Error input: %v\n", err)
			return 1
		} else {
			forHistory = strings.Join(input, "\n")
			for i, ln := range input {
				input[i] = strings.TrimSuffix(ln, `\`)
			}
			line = strings.Join(input, "")
		}

		// Command parsing and execution are Shell-only responsibilities.
		if _, alive := sh.process(line); !alive {
			return 0
		}
		// this makes to prevent adding 'exit' command to history
		sh.history.Add(forHistory)
	}
}

func defaultShellProfile() RuntimeProfile {
	return RuntimeProfile{
		Name:        "shell",
		Description: "JSH command interpreter",
		Banner: func() string {
			return banner + betaWarn
		},
		Metadata: map[string]any{
			"product": "shell",
			"mode":    "command",
		},
	}
}

func (sh *Shell) configureEditorSession(ed *multiline.Editor) {
	ed.SetPredictColor([...]string{"\x1B[3;22;30m", "\x1B[23;39m"}) // dark gray, italic
	ed.LineEditor.Predictor = sh.predictHistory
	ed.ResetColor = "\x1B[0m"
	ed.DefaultColor = "\x1B[37;49;1m"

	// enable completion
	ed.BindKey(keys.CtrlI, &completion.CmdCompletionOrList{
		Delimiter:  "&|><",
		Enclosure:  `"'`,
		Postfix:    " ",
		Candidates: sh.getCompletionCandidates,
	})
	sh.bindPredictionKeys(ed)
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

func (sh *Shell) predictHistory(buf *readline.Buffer) string {
	return predictShellHistory(buf.String(), buf.History)
}

func (sh *Shell) bindPredictionKeys(ed *multiline.Editor) {
	acceptOrForward := &readline.GoCommand{
		Name: "SHELL_ACCEPT_PREDICT_OR_FORWARD",
		Func: func(ctx context.Context, buf *readline.Buffer) readline.Result {
			if shouldAcceptPrediction(buf.Cursor, len(buf.Buffer), ed.CursorLine(), len(ed.Lines())) {
				return readline.CmdAcceptPredict.Call(ctx, buf)
			}
			return ed.CmdForwardChar(ctx, buf)
		},
	}
	ed.BindKey(keys.Right, acceptOrForward)
	ed.BindKey(keys.CtrlF, acceptOrForward)
}

func predictShellHistory(current string, history readline.IHistory) string {
	if history == nil || strings.TrimSpace(current) == "" || strings.HasSuffix(current, `\`) {
		return ""
	}
	for i := history.Len() - 1; i >= 0; i-- {
		for _, line := range strings.Split(history.At(i), "\n") {
			candidate := strings.TrimSuffix(line, `\`)
			if strings.HasPrefix(candidate, current) {
				return current + candidate[len(current):]
			}
		}
	}
	return ""
}

func shouldAcceptPrediction(cursor int, bufferLen int, cursorLine int, lineCount int) bool {
	if cursor < bufferLen {
		return false
	}
	if lineCount <= 0 {
		return true
	}
	return cursorLine >= lineCount-1
}

func (sh *Shell) getCompletionCandidates(fields []string) (forCompletion []string, forListing []string) {
	return
}

// process evaluates shell statements and operators.
// This stays Shell-only and is intentionally excluded from shared foundation
// extraction with Repl.
func (sh *Shell) process(line string) (int, bool) {
	// Parse the command
	cmd := parseCommand(line)
	lastExitCode := 0

	for _, stmt := range cmd.Statements {
		var stopOnError bool
		if stmt.Operator == "&&" {
			stopOnError = true
		}
		exitCode, alive := sh.runStatement(stmt)
		lastExitCode = exitCode
		if !alive {
			return exitCode, false
		}
		if exitCode != 0 && stopOnError {
			return exitCode, true
		}
	}
	return lastExitCode, true
}

// exec resolves and executes shell commands in the runtime.
// This is intentionally not shared with Repl because it belongs to the command
// interpreter execution path.
func (sh *Shell) exec(command string, args []string) goja.Value {
	command, args = sh.expandCommandAlias(command, args)
	parts := []string{}
	for _, arg := range args {
		parts = append(parts, fmt.Sprintf("%q", arg))
	}
	str := strings.Join(parts, ", ")

	val, err := sh.rt.RunString(fmt.Sprintf(`(()=>{
		const {exec, which} = require("process");
		let command = %q;
		let args = [%s];
		let path = which(command);
		if (!path || path === "") {
			// try command as directory contains index.js
			const pathAsDir = !command.endsWith(".js") ? which(command + "/index.js") : "";
			if (!pathAsDir || pathAsDir === "") {
				throw new Error("command not found: " + command);
			} else {
				path = pathAsDir;
			}
		}
		return exec(path, ...args);
	})()`, command, str))

	if err != nil {
		if jsErr, ok := err.(*goja.Exception); ok {
			return jsErr.Value()
		}
		panic(err)
	}
	return val
}
