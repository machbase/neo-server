//go:build darwin || linux || freebsd || openbsd || netbsd

package tail

import (
	"os"
	"syscall"
)

func getFileIDFromHandle(f *os.File) (uint64, error) {
	stat, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return getInode(stat), nil
}

func getInode(stat os.FileInfo) uint64 {
	if sys, ok := stat.Sys().(*syscall.Stat_t); ok {
		return sys.Ino
	}
	return 0
}

func openFileShared(filepath string) (*os.File, error) {
	return os.Open(filepath)
}
