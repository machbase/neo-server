package engine

import (
	_ "embed"
	"fmt"
	"os"
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

func (jr *JSRuntime) Exec(vm *goja.Runtime, source string, args []string) goja.Value {
	eb := jr.Env.ExecBuilder()
	if eb == nil {
		return vm.NewGoError(fmt.Errorf("no command builder defined"))
	}
	env := jr.Env.vars
	cmd, err := eb(source, args, env)
	if err != nil {
		return vm.NewGoError(err)
	}
	return jr.exec0(vm, cmd)
}
