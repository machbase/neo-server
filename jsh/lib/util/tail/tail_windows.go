//go:build windows

package tail

import (
	"fmt"
	"os"
	"syscall"
)

func getInode(_ os.FileInfo) uint64 {
	return 0
}

func getFileIDFromHandle(f *os.File) (uint64, error) {
	var info syscall.ByHandleFileInformation
	err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &info)
	if err != nil {
		return 0, err
	}
	return uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow), nil
}

func openFileShared(filepath string) (*os.File, error) {
	pathPtr, err := syscall.UTF16PtrFromString(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert path: %w", err)
	}
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
