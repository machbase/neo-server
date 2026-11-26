//go:build !windows
// +build !windows

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const daemonChildEnv = "_MACHBASE_NEO_DAEMON_CHILD"
const daemonStartupTimeout = 2 * time.Minute

type daemonStatus struct {
	Error string
}

func daemonizeWithError(bootlog, pidPath string, proc func(func() error, *os.File) error) (retErr error) {
	if os.Getenv(daemonChildEnv) == "1" {
		if err := os.Unsetenv(daemonChildEnv); err != nil {
			return fmt.Errorf("clear daemon environment: %w", err)
		}
		syscall.Umask(027)
		status := os.NewFile(3, "daemon status")
		if status == nil {
			return errors.New("daemon status pipe is unavailable")
		}
		syscall.CloseOnExec(3)
		sent := false
		send := func(err error) error {
			if sent {
				return nil
			}
			sent = true
			response := daemonStatus{}
			if err != nil {
				response.Error = err.Error()
			}
			writeErr := json.NewEncoder(status).Encode(response)
			return errors.Join(writeErr, status.Close())
		}
		var pidFile *os.File
		if pidPath != "" {
			pidFile = os.NewFile(4, "daemon pid")
			syscall.CloseOnExec(4)
		}
		err := proc(func() error { return send(nil) }, pidFile)
		if !sent {
			if err == nil {
				err = errors.New("daemon exited before reporting readiness")
			}
			err = errors.Join(err, send(err))
		}
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve daemon executable: %w", err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create daemon status pipe: %w", err)
	}
	defer reader.Close()
	defer writer.Close()

	logPath := bootlog
	if logPath == "" {
		logPath = os.DevNull
	}
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open daemon boot log %q: %w", logPath, err)
	}
	defer logFile.Close()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	var pidFile *os.File
	var pidInfo os.FileInfo
	pidOwned := false
	// Reserve the PID file until readiness transfers ownership to the child.
	defer func() {
		if pidFile != nil {
			retErr = errors.Join(retErr, pidFile.Close())
		}
		if pidOwned && pidInfo != nil {
			retErr = errors.Join(retErr, cleanupFailedDaemonPID(pidPath, pidInfo))
		}
	}()
	if pidPath != "" {
		pidFile, err = os.OpenFile(pidPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
		if err != nil {
			return fmt.Errorf("create pid file %q: %w", pidPath, err)
		}
		pidOwned = true
		pidInfo, err = pidFile.Stat()
		if err != nil {
			return fmt.Errorf("stat pid file %q: %w", pidPath, err)
		}
	}

	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Env = append(os.Environ(), daemonChildEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.ExtraFiles = []*os.File{writer}
	if pidFile != nil {
		cmd.ExtraFiles = append(cmd.ExtraFiles, pidFile)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	abort := func(reason error) error {
		stopped, err := stopFailedDaemon(cmd)
		if !stopped {
			pidOwned = false
		}
		return errors.Join(reason, err)
	}
	if err := writer.Close(); err != nil {
		return abort(fmt.Errorf("close daemon status pipe: %w", err))
	}

	type result struct {
		status daemonStatus
		err    error
	}
	resultCh := make(chan result, 1)
	go func() {
		var status daemonStatus
		err := json.NewDecoder(reader).Decode(&status)
		resultCh <- result{status: status, err: err}
	}()
	timer := time.NewTimer(daemonStartupTimeout)
	defer timer.Stop()
	select {
	case result := <-resultCh:
		if result.err == nil && result.status.Error == "" {
			if pidFile != nil {
				err := pidFile.Close()
				pidFile = nil
				if err != nil {
					return abort(fmt.Errorf("close pid file: %w", err))
				}
			}
			pidOwned = false
			return cmd.Process.Release()
		}
		if result.err != nil {
			return abort(fmt.Errorf("daemon did not report startup: %w", result.err))
		}
		return abort(fmt.Errorf("daemon startup failed: %s", result.status.Error))
	case <-timer.C:
		return abort(fmt.Errorf("daemon startup timed out after %s", daemonStartupTimeout))
	case sig := <-signals:
		return abort(fmt.Errorf("daemon startup interrupted by %s", sig))
	}
}

func stopFailedDaemon(cmd *exec.Cmd) (bool, error) {
	signalErr := cmd.Process.Signal(syscall.SIGTERM)
	if errors.Is(signalErr, os.ErrProcessDone) || errors.Is(signalErr, syscall.ESRCH) {
		signalErr = nil
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	waitResult := func(err error) (bool, error) {
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			return false, errors.Join(signalErr, fmt.Errorf("wait for daemon: %w", err))
		}
		return true, signalErr
	}
	select {
	case err := <-waited:
		return waitResult(err)
	case <-time.After(5 * time.Second):
		killErr := cmd.Process.Kill()
		if errors.Is(killErr, os.ErrProcessDone) || errors.Is(killErr, syscall.ESRCH) {
			killErr = nil
		}
		if killErr != nil {
			return false, errors.Join(signalErr, fmt.Errorf("kill daemon: %w", killErr))
		}
		return waitResult(<-waited)
	}
}

func cleanupFailedDaemonPID(path string, info os.FileInfo) error {
	if info == nil {
		return nil
	}
	current, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat pid file %q: %w", path, err)
	}
	if !os.SameFile(info, current) {
		return nil
	}
	return removeOwnedPID(path, info)
}
