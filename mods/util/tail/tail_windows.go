//go:build windows

package tail

import (
	"os"
	"syscall"
)

// getInode returns a unique file identifier on Windows
// Windows doesn't have inodes, but we can use the file index which serves a similar purpose
func getInode(stat os.FileInfo) uint64 {
	if sys, ok := stat.Sys().(*syscall.Win32FileAttributeData); ok {
		// On Windows, we can use a combination of VolumeSerialNumber and FileIndex
		// to uniquely identify a file
		return uint64(sys.FileSizeHigh)<<32 | uint64(sys.FileSizeLow)
	}
	return 0
}

// getFileID returns the actual file ID on Windows
func getFileID(f *os.File) (uint64, error) {
	var info syscall.ByHandleFileInformation
	err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &info)
	if err != nil {
		return 0, err
	}

	// Combine FileIndexHigh and FileIndexLow to create a unique identifier
	fileID := uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)
	return fileID, nil
}
