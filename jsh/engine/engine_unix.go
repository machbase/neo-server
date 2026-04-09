//go:build linux || darwin

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"unsafe"
)

func (jr *JSRuntime) exec0(ex *exec.Cmd, opts ExecOptions) (int, error) {
	ex.Stdin = resolveExecReader(jr.Env.Reader(), opts.Stdin)
	ex.Stdout = resolveExecWriter(jr.Env.Writer(), opts.Stdout)
	ex.Stderr = resolveExecWriter(jr.Env.ErrorWriter(), opts.Stderr)

	// Get terminal file descriptor
	ttyFd := int(os.Stdin.Fd())
	isTTY := isatty(ttyFd)

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

	procEntry, err := jr.createProcessEntry(ex)
	if err != nil {
		return -1, err
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

	// restore this parent process to foreground
	if isTTY {
		_, _, err := syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(os.Stdin.Fd()),
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
