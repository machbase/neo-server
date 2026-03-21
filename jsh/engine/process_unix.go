//go:build !windows

package engine

import (
	"os"
)

func killProcess(pid int, signalLabel string, signalNumber int, osSignal os.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(osSignal)
}
