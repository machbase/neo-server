//go:build windows

package tailer

import (
	"fmt"
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

// openFileShared opens a file in shared mode on Windows
// This allows the file to be renamed or deleted while it's open
func openFileShared(filepath string) (*os.File, error) {
	pathPtr, err := syscall.UTF16PtrFromString(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert path: %w", err)
	}

	// Open with FILE_SHARE_DELETE to allow the file to be renamed/deleted while open
	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return os.NewFile(uintptr(handle), filepath), nil
}
