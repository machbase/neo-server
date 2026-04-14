//go:build linux || darwin

package engine

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func waitCmdDone(t *testing.T, cmd *exec.Cmd, timeout time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("process did not exit within %v", timeout)
	case <-done:
	}
}

func TestForwardSignalToChildGroupNilCommand(t *testing.T) {
	forwardSignalToChildGroup(nil, syscall.SIGTERM)
}

func TestForwardSignalToChildGroupProcessGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "while :; do sleep 1; done")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}

	forwardSignalToChildGroup(cmd, syscall.SIGTERM)
	waitCmdDone(t, cmd, 3*time.Second)
}

func TestForwardSignalToChildGroupFallbackToProcessSignal(t *testing.T) {
	cmd := exec.Command("sh", "-c", "while :; do sleep 1; done")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}

	forwardSignalToChildGroup(cmd, syscall.SIGTERM)
	waitCmdDone(t, cmd, 3*time.Second)
}

func TestKillProcessSendsSignal(t *testing.T) {
	cmd := exec.Command("sh", "-c", "while :; do sleep 1; done")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}

	if err := killProcess(cmd.Process.Pid, "SIGTERM", 15, syscall.SIGTERM); err != nil {
		t.Fatalf("killProcess: %v", err)
	}
	waitCmdDone(t, cmd, 3*time.Second)
}
