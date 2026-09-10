//go:build windows

package service

import "os"

func terminateServiceProcess(process *os.Process) error {
	return process.Kill()
}
