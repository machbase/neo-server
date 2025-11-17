//go:build darwin || linux || freebsd || openbsd || netbsd

package tailer

import (
	"os"
	"syscall"
)

// getInode returns the inode number of a file
// This is used to detect when a file has been rotated
func getInode(stat os.FileInfo) uint64 {
	if sys, ok := stat.Sys().(*syscall.Stat_t); ok {
		return sys.Ino
	}
	return 0
}

// openFileShared opens a file on Unix systems
// On Unix, files can be renamed/deleted while open by default
func openFileShared(filepath string) (*os.File, error) {
	return os.Open(filepath)
}
