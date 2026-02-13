//go:build windows

package engine

import (
	"os/exec"

	"github.com/dop251/goja"
)

func (jr *JSRuntime) exec0(vm *goja.Runtime, ex *exec.Cmd) goja.Value {
	ex.Stdin = jr.Env.Reader()
	ex.Stdout = jr.Env.Writer()
	ex.Stderr = jr.Env.Writer()

	// Windows doesn't support process groups like Unix
	// Just run the process directly
	if err := ex.Start(); err != nil {
		return vm.NewGoError(err)
	}

	// wait for process to finish
	var result goja.Value
	if err := ex.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result = vm.ToValue(exitErr.ExitCode())
		} else {
			result = vm.NewGoError(err)
		}
	} else {
		result = vm.ToValue(0)
	}

	return result
}
