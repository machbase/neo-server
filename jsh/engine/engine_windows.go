//go:build windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

var ensureProcessExistsFn = ensureProcessExists
var sendInterruptSignalFn = sendInterruptSignal

func (jr *JSRuntime) exec0(ex *exec.Cmd, opts ExecOptions) (int, error) {
	ex.Stdin = resolveExecReader(jr.Env.Reader(), opts.Stdin)
	ex.Stdout = resolveExecWriter(jr.Env.Writer(), opts.Stdout)
	ex.Stderr = resolveExecWriter(jr.Env.ErrorWriter(), opts.Stderr)

	// Windows doesn't support process groups like Unix
	// Just run the process directly
	if err := ex.Start(); err != nil {
		return -1, err
	}

	procEntry, err := jr.createProcessEntry(ex)
	if err != nil {
		// Process entry recording is best-effort.
		// Do not fail command execution when service-controller is overloaded.
		fmt.Fprintf(jr.Env.ErrorWriter(), "warning: process entry record failed: %v\n", err)
		procEntry = nil
	}

	// wait for process to finish
	var result int
	if err := ex.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result = exitErr.ExitCode()
		} else {
			result = -1
		}
	} else {
		result = 0
	}

	if procEntry != nil {
		procEntry.finish(result)
	}

	return result, nil
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
