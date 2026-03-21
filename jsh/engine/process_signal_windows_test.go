//go:build windows

package engine_test

import (
	"fmt"
	"os/exec"
	"syscall"

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
