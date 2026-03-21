//go:build windows

package engine_test

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func prepareSignalHelperCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func sendTestSignal(cmd *exec.Cmd, signalName string) error {
	switch signalName {
	case "SIGINT":
		return windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))
	default:
		return fmt.Errorf("unsupported external signal delivery on windows: %s", signalName)
	}
}

func requireWindowsSignalIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("JSH_WINDOWS_SIGNAL_INTEGRATION") != "1" {
		t.Skip("windows signal integration is opt-in; set JSH_WINDOWS_SIGNAL_INTEGRATION=1 to run tests that emit console control events")
	}
}
