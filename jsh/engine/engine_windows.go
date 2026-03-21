//go:build windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/dop251/goja"
	"golang.org/x/sys/windows"
)

var ensureProcessExistsFn = ensureProcessExists
var sendInterruptSignalFn = sendInterruptSignal

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

func killProcess(pid int, signalLabel string, signalNumber int, osSignal os.Signal) error {
	if err := ensureProcessExistsFn(pid); err != nil {
		return err
	}

	if signalNumber == 0 {
		return nil
	}

	switch signalNumber {
	case 2:
		if err := sendInterruptSignalFn(pid); err != nil {
			return fmt.Errorf("SIGINT delivery on windows requires a console process group and may be unavailable: %w", err)
		}
		return nil
	case 3, 9, 15:
		proc, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return proc.Kill()
	default:
		return fmt.Errorf("unsupported signal on windows: %s", signalLabel)
	}
}

func ensureProcessExists(pid int) error {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return err
	}
	return windows.CloseHandle(handle)
}

func sendInterruptSignal(pid int) error {
	return windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(pid))
}
