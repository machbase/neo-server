//go:build windows

package engine

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

var ensureProcessExistsFn = ensureProcessExists
var sendInterruptSignalFn = sendInterruptSignal

// platformErrnoCode maps Win32 error codes to Node.js style codes, following libuv.
func platformErrnoCode(errno syscall.Errno) string {
	switch errno {
	case syscall.ERROR_FILE_NOT_FOUND, syscall.ERROR_PATH_NOT_FOUND, 123 /* ERROR_INVALID_NAME */ :
		return "ENOENT"
	case syscall.ERROR_ACCESS_DENIED:
		return "EPERM"
	case syscall.ERROR_ALREADY_EXISTS, syscall.ERROR_FILE_EXISTS:
		return "EEXIST"
	case syscall.ERROR_DIR_NOT_EMPTY:
		return "ENOTEMPTY"
	case 267 /* ERROR_DIRECTORY */ :
		return "ENOTDIR"
	case 206 /* ERROR_FILENAME_EXCED_RANGE */ :
		return "ENAMETOOLONG"
	case 112 /* ERROR_DISK_FULL */ :
		return "ENOSPC"
	case 19 /* ERROR_WRITE_PROTECT */ :
		return "EROFS"
	case 1921 /* ERROR_CANT_RESOLVE_FILENAME */ :
		return "ELOOP"
	case 50 /* ERROR_NOT_SUPPORTED */ :
		return "ENOTSUP"
	case 32 /* ERROR_SHARING_VIOLATION */, 33 /* ERROR_LOCK_VIOLATION */ :
		return "EBUSY"
	case 17 /* ERROR_NOT_SAME_DEVICE */ :
		return "EXDEV"
	case 6 /* ERROR_INVALID_HANDLE */ :
		return "EBADF"
	}
	return ""
}

func accessOS(path string, mode uint32) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if mode&4 != 0 {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	if mode&2 != 0 {
		if info.IsDir() {
			// Directories cannot be opened for writing on Windows; use the read-only attribute.
			if info.Mode().Perm()&0o200 == 0 {
				return &fs.PathError{Op: "access", Path: path, Err: fs.ErrPermission}
			}
		} else {
			file, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
	}
	// Like Node.js on Windows, X_OK only checks existence because Windows
	// file modes carry no execute bits.
	return nil
}

func unlinkOS(path string) error {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	deleteErr := syscall.DeleteFile(name)
	if deleteErr == nil {
		return nil
	}
	attrs, attrErr := syscall.GetFileAttributes(name)
	if attrErr != nil {
		return deleteErr
	}
	if attrs&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		// Directory symlinks and junctions are removed like links, as Node.js does.
		if attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return syscall.RemoveDirectory(name)
		}
		return syscall.EISDIR
	}
	// Like os.Remove and Node.js, clear the read-only attribute before retrying.
	if attrs&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		if err := syscall.SetFileAttributes(name, attrs&^syscall.FILE_ATTRIBUTE_READONLY); err == nil {
			if err := syscall.DeleteFile(name); err == nil {
				return nil
			}
			_ = syscall.SetFileAttributes(name, attrs)
		}
	}
	return deleteErr
}

func rmdirOS(path string) error {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return syscall.RemoveDirectory(name)
}

func (jr *JSRuntime) exec0(ex *exec.Cmd, opts ExecOptions) (int, error) {
	ex.Stdin = resolveExecReader(jr.Env.Reader(), opts.Stdin)
	ex.Stdout = resolveExecWriter(jr.Env.Writer(), opts.Stdout)
	ex.Stderr = resolveExecWriter(jr.Env.ErrorWriter(), opts.Stderr)

	// Windows doesn't support process groups like Unix
	// Just run the process directly
	if err := ex.Start(); err != nil {
		return -1, err
	}

	var procEntryWarn error
	procEntry, err := jr.createProcessEntry(ex)
	if err != nil {
		// Process entry recording is best-effort.
		// Do not fail command execution when service-controller is overloaded.
		procEntryWarn = err
		procEntry = nil
	}

	// wait for process to finish
	var result int
	if err := ex.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result = exitErr.ExitCode()
		} else {
			result = -1
		}
	} else {
		result = 0
	}

	if procEntry != nil {
		procEntry.finish(result)
	}
	if procEntryWarn != nil {
		fmt.Fprintf(jr.Env.ErrorWriter(), "warning: process entry record failed: %v\n", procEntryWarn)
	}

	return result, nil
}

func killProcess(pid int, signalLabel string, signalNumber int, osSignal os.Signal) error {
	if err := ensureProcessExistsFn(pid); err != nil {
		return err
	}

	if signalNumber == 0 {
		return nil
	}

	switch signalNumber {
	case 2:
		if err := sendInterruptSignalFn(pid); err != nil {
			return fmt.Errorf("SIGINT delivery on windows requires a console process group and may be unavailable: %w", err)
		}
		return nil
	case 3, 9, 15:
		proc, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return proc.Kill()
	default:
		return fmt.Errorf("unsupported signal on windows: %s", signalLabel)
	}
}

func ensureProcessExists(pid int) error {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return err
	}
	return windows.CloseHandle(handle)
}

func sendInterruptSignal(pid int) error {
	return windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(pid))
}
