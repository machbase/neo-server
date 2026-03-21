//go:build windows

package engine

import (
	"fmt"
	"os"
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

func killProcess(pid int, signalLabel string, signalNumber int, osSignal os.Signal) error {
	if err := ensureProcessExists(pid); err != nil {
		return err
	}

	if signalNumber == 0 {
		return nil
	}

	switch signalNumber {
	case 2, 3, 9, 15:
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
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return err
	}
	return syscall.CloseHandle(handle)
}
