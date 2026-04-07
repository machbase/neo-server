//go:build windows

package engine

import "os"

func procProcessGroupID(pid int) int {
	return pid
}

func procProcessExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	return ensureProcessExistsFn(pid) == nil
}

func procProcessOSPid() int {
	return os.Getpid()
}
