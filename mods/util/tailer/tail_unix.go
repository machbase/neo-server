//go:build darwin || linux || freebsd || openbsd || netbsd

package tailer

import (
	"os"
	"syscall"
)

// getFileIDFromHandle gets the unique file ID from an open file handle
// On Unix systems, this returns the inode number
func getFileIDFromHandle(f *os.File) (uint64, error) {
	stat, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return getInode(stat), nil
}

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
