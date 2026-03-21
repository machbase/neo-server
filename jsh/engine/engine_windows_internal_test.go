//go:build windows

package engine

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestKillProcessSigintUsesInterruptSender(t *testing.T) {
	originalEnsure := ensureProcessExistsFn
	originalSend := sendInterruptSignalFn
	t.Cleanup(func() {
		ensureProcessExistsFn = originalEnsure
		sendInterruptSignalFn = originalSend
	})

	var ensuredPID int
	var signaledPID int
	ensureProcessExistsFn = func(pid int) error {
		ensuredPID = pid
		return nil
	}
	sendInterruptSignalFn = func(pid int) error {
		signaledPID = pid
		return nil
	}

	if err := killProcess(4321, "SIGINT", 2, os.Interrupt); err != nil {
		t.Fatalf("killProcess returned error: %v", err)
	}
	if ensuredPID != 4321 {
		t.Fatalf("ensureProcessExists called with %d, want 4321", ensuredPID)
	}
	if signaledPID != 4321 {
		t.Fatalf("sendInterruptSignal called with %d, want 4321", signaledPID)
	}
}

func TestKillProcessSigintUnavailableWrapsError(t *testing.T) {
	originalEnsure := ensureProcessExistsFn
	originalSend := sendInterruptSignalFn
	t.Cleanup(func() {
		ensureProcessExistsFn = originalEnsure
		sendInterruptSignalFn = originalSend
	})

	ensureProcessExistsFn = func(pid int) error {
		return nil
	}
	sendInterruptSignalFn = func(pid int) error {
		return errors.New("GenerateConsoleCtrlEvent failed")
	}

	err := killProcess(4321, "SIGINT", 2, os.Interrupt)
	if err == nil {
		t.Fatal("expected killProcess to return an error")
	}
	if !strings.Contains(err.Error(), "SIGINT delivery on windows requires a console process group and may be unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}
