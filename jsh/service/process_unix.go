//go:build !windows

package service

import (
	"os"
	"syscall"
)

func terminateServiceProcess(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}
