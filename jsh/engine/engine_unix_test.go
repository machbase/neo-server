//go:build !windows

package engine_test

import (
	"os/exec"
	"testing"
)

func prepareSignalHelperCommand(cmd *exec.Cmd) {
}

func sendTestSignal(cmd *exec.Cmd, signalName string) error {
	return cmd.Process.Signal(testSignalByName(signalName))
}

func requireWindowsSignalIntegration(t *testing.T) {
	// No-op on non-windows platforms since the signal integration is only relevant to windows.
}
