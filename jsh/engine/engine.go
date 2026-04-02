package engine

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/url"
	"github.com/machbase/neo-server/v8/jsh/log"
)

type JSRuntime struct {
	Name   string
	Source string
	Args   []string
	Strict bool
	Env    *Env

	registry      *require.Registry
	eventLoop     *eventloop.EventLoop
	filesystem    *FS
	exitCode      int
	shutdownHooks []func()
	nowFunc       func() time.Time
}

type ExecOptions struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func (jr *JSRuntime) RegisterNativeModule(name string, loader require.ModuleLoader) {
	jr.registry.RegisterNativeModule(name, loader)
}

func (jr *JSRuntime) EventLoop() *eventloop.EventLoop {
	return jr.eventLoop
}

func (jr *JSRuntime) Run() error {
	if jr.Env == nil {
		jr.Env = &Env{}
	}

	defer func() {
		if r := recover(); r != nil {
			if ie, ok := r.(*goja.InterruptedError); ok {
				fmt.Fprintf(jr.Env.Writer(), "interrupted: %v\n", ie.Value())
			}
			fmt.Fprintf(os.Stderr, "panic: %v\n%v\n", r, string(debug.Stack()))
			os.Exit(1)
		}
	}()

	// guarantee shutdown hooks run at the end
	defer func() {
		slices.Reverse(jr.shutdownHooks)
		for _, hook := range jr.shutdownHooks {
			hook()
		}
	}()

	program, err := goja.Compile(jr.Name, jr.Source, jr.Strict)
	if err != nil {
		return err
	}
	var retErr error = nil
	jr.eventLoop.Run(func(vm *goja.Runtime) {
		buffer.Enable(vm)
		url.Enable(vm)
		vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
		vm.Set("console", log.SetConsole(vm, jr.Env.Writer()))
		// goja_nodejs core-util module also uses 'util' name
		// and it is loaded first then '/lib/util' would be ignored
		// so we copy all exports from '/lib/util' into 'util' here
		vm.RunScript("init", `(()=>{
			const u = require('/lib/util');
			const util = require('util');
			for (const k of Object.keys(u)) {
				util[k] = u[k];
			}
		})();`)
		if _, err := vm.RunProgram(program); err != nil {
			retErr = err
			if ie, ok := err.(*goja.InterruptedError); ok {
				if ec, ok := ie.Value().(Exit); ok {
					// process.exit(exit_code) called from javascript
					// it indicates normal termination
					// do not treat as error
					jr.exitCode = ec.Code
					return
				}
				fmt.Fprintf(jr.Env.Writer(), "Interrupted: %s\n", ie.String())
			} else if jsErr, ok := err.(*goja.Exception); ok {
				msg := jsErr.String()
				msg = strings.TrimPrefix(msg, "GoError: ")
				fmt.Fprintf(jr.Env.Writer(), "%s\n", msg)
			} else {
				msg := err.Error()
				msg = strings.TrimPrefix(msg, "GoError: ")
				fmt.Fprintf(jr.Env.Writer(), "%s\n", msg)
			}
			jr.exitCode = -1
		}
	})
	return retErr
}

func (jr *JSRuntime) ExitCode() int {
	return jr.exitCode
}

func (jr *JSRuntime) AddShutdownHook(hook func()) {
	jr.shutdownHooks = append(jr.shutdownHooks, hook)
}

// doExec executes a command by building an exec.Cmd and running it.
//
// syntax) exec(command: string, ...args: string): number
// return) exit code
func (jr *JSRuntime) Exec(path string, args ...string) (int, error) {
	return jr.ExecWithOptions(path, ExecOptions{}, args...)
}

func (jr *JSRuntime) ExecWithOptions(path string, opts ExecOptions, args ...string) (int, error) {
	if path == "" && len(args) == 0 {
		return -1, fmt.Errorf("no command provided")
	}
	if strings.HasPrefix(path, "@") {
		// when path starts with "@",
		// treat it as calling a native executable with args
		cmd := exec.Command(path[1:], args...)
		return jr.exec0(cmd, opts)
	}
	eb := jr.Env.ExecBuilder()
	if eb == nil {
		return -1, fmt.Errorf("no command builder defined")
	}
	env := jr.Env.vars
	argv := make([]string, 0, len(args)+1)
	argv = append(argv, path)
	argv = append(argv, args...)
	cmd, err := eb("", argv, env)
	if err != nil {
		return -1, err
	}
	return jr.exec0(cmd, opts)
}

// doExecString executes a command line string via the exec function.
//
// syntax) execString(source: string): number
// return) exit code
func (jr *JSRuntime) ExecString(source string) (int, error) {
	return jr.ExecStringWithOptions(source, ExecOptions{})
}

func (jr *JSRuntime) ExecStringWithOptions(source string, opts ExecOptions) (int, error) {
	if source == "" {
		return -1, fmt.Errorf("no source provided")
	}
	eb := jr.Env.ExecBuilder()
	if eb == nil {
		return -1, fmt.Errorf("no command builder defined")
	}
	env := jr.Env.vars
	cmd, err := eb(source, nil, env)
	if err != nil {
		return -1, err
	}
	return jr.exec0(cmd, opts)
}

func resolveExecReader(defaultReader io.Reader, override io.Reader) io.Reader {
	if override != nil {
		return override
	}
	return defaultReader
}

func resolveExecWriter(defaultWriter io.Writer, override io.Writer) io.Writer {
	if override != nil {
		return override
	}
	return defaultWriter
}
