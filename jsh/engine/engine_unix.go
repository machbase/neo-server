//go:build linux || darwin

package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
	"unsafe"
)

func forwardSignalToChildGroup(ex *exec.Cmd, sig os.Signal) {
	if ex == nil || ex.Process == nil {
		return
	}

	childPID := ex.Process.Pid
	sysSig, ok := sig.(syscall.Signal)
	if !ok {
		_ = ex.Process.Signal(sig)
		return
	}

	// Prefer delivering to the child's process group so descendants also receive it.
	if err := syscall.Kill(-childPID, sysSig); err == nil {
		return
	}
	_ = ex.Process.Signal(sig)
}

func (jr *JSRuntime) exec0(ex *exec.Cmd, opts ExecOptions) (int, error) {
	ex.Stdin = resolveExecReader(jr.Env.Reader(), opts.Stdin)
	ex.Stdout = resolveExecWriter(jr.Env.Writer(), opts.Stdout)
	ex.Stderr = resolveExecWriter(jr.Env.ErrorWriter(), opts.Stderr)

	// Determine interactive mode from the actual stdin wired into the child.
	// CGI/background executions should not capture process-wide SIGINT handlers.
	ttyFd := -1
	isTTY := false
	if stdinFile, ok := ex.Stdin.(*os.File); ok {
		ttyFd = int(stdinFile.Fd())
		isTTY = isatty(ttyFd)
	}

	// Get shell's process group ID
	var shellPgid int
	if isTTY {
		shellPgid = syscall.Getpgrp()
	}

	// child process group, false: not separate
	ex.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // new process group
		Pgid:    0,    // use child's PID as pgid
	}

	if isTTY {
		// Ignore TTY-related signals during job control operations
		signal.Ignore(syscall.SIGTTOU)
		signal.Ignore(syscall.SIGTTIN)
		signal.Ignore(syscall.SIGTSTP)
		defer func() {
			signal.Reset(syscall.SIGTTOU)
			signal.Reset(syscall.SIGTTIN)
			signal.Reset(syscall.SIGTSTP)
		}()
	}

	// child process start
	if err := ex.Start(); err != nil {
		return -1, err
	}

	var parentSignalCh chan os.Signal
	var forwardDone chan struct{}
	if isTTY {
		// In interactive/TTY mode, SIGINT can be delivered to the parent shell
		// process directly (instead of the foreground child). While waiting for the
		// child, forward interrupt/quit to the child process group.
		parentSignalCh = make(chan os.Signal, 2)
		signal.Notify(parentSignalCh, os.Interrupt, syscall.SIGQUIT)
		defer signal.Stop(parentSignalCh)

		forwardDone = make(chan struct{})
		defer close(forwardDone)
		go func() {
			for {
				select {
				case <-forwardDone:
					return
				case sig := <-parentSignalCh:
					forwardSignalToChildGroup(ex, sig)
				}
			}
		}()
	}

	var procEntryWarn error
	procEntry, err := jr.createProcessEntry(ex)
	if err != nil {
		// Process entry recording is best-effort.
		// Do not fail command execution when service-controller is overloaded.
		procEntryWarn = err
		procEntry = nil
	}

	if isTTY {

		childPgid := ex.Process.Pid
		// give terminal control to child process
		_, _, err := syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(ttyFd),
			syscall.TIOCSPGRP,
			uintptr(unsafe.Pointer(&childPgid)))
		if err != 0 {
			fmt.Printf("failed to set foreground: %v\n", err)
		}
	}

	var cancelDone chan struct{}
	if jr.ctx != nil {
		cancelDone = make(chan struct{})
		go func(pid int) {
			select {
			case <-cancelDone:
				return
			case <-jr.ctx.Done():
				// Try graceful termination first for the whole process group.
				if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
					if proc := ex.Process; proc != nil {
						_ = proc.Signal(syscall.SIGTERM)
					}
				}

				timer := time.NewTimer(2 * time.Second)
				defer timer.Stop()
				select {
				case <-cancelDone:
					return
				case <-timer.C:
					// Ensure no zombie child remains after request cancellation.
					if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
						if proc := ex.Process; proc != nil {
							_ = proc.Signal(syscall.SIGKILL)
						}
					}
				}
			}
		}(ex.Process.Pid)
		defer close(cancelDone)
	}

	// wait for process to finish
	var result int
	for {
		err := ex.Wait()
		if err == nil {
			result = 0
			break
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result = exitErr.ExitCode()
		} else {
			result = -1
		}
		break
	}

	if procEntry != nil {
		procEntry.finish(result)
	}
	if procEntryWarn != nil {
		fmt.Fprintf(jr.Env.ErrorWriter(), "warning: process entry record failed: %v\n", procEntryWarn)
	}

	// restore this parent process to foreground
	if isTTY {
		_, _, err := syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(ttyFd),
			syscall.TIOCSPGRP,
			uintptr(unsafe.Pointer(&shellPgid)))
		if err != 0 {
			fmt.Printf("failed to restore foreground: %v\n", err)
		}
	}
	return result, nil
}

// isatty checks if fd is a terminal
func isatty(fd int) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		ioctlReadTermios,
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0)
	return err == 0
}

func killProcess(pid int, signalLabel string, signalNumber int, osSignal os.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(osSignal)
}
