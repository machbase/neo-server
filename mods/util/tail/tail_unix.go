//go:build darwin || linux || freebsd || openbsd || netbsd

package tail

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
