//go:build !windows

package engine_test

import "os/exec"

func prepareSignalHelperCommand(cmd *exec.Cmd) {
}

func sendTestSignal(cmd *exec.Cmd, signalName string) error {
	return cmd.Process.Signal(testSignalByName(signalName))
}
