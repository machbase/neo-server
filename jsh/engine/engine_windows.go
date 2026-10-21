//go:build windows

package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

var ensureProcessExistsFn = ensureProcessExists
var sendInterruptSignalFn = sendInterruptSignal

// ErrnoNotDir is the errno JSH file systems report for a path component that
// is not a directory. syscall.ENOTDIR cannot be used on Windows because it
// aliases ERROR_PATH_NOT_FOUND, which Win32 reports for a missing parent.
const ErrnoNotDir = syscall.Errno(267) // ERROR_DIRECTORY

// platformErrnoCode maps Win32 error codes to Node.js style codes, following
// libuv (src/win/error.c and the readlink special case in src/win/fs.c).
// ERROR_DIRECTORY is not handled here: it is ErrnoNotDir and maps to ENOTDIR,
// whereas libuv reports a raw ERROR_DIRECTORY as ENOENT.
func platformErrnoCode(err error, errno syscall.Errno) string {
	switch errno {
	case syscall.ERROR_PATH_NOT_FOUND:
		// Win32 reports a missing or non-directory parent component with
		// ERROR_PATH_NOT_FOUND. Go's readdir reports a non-directory with
		// syscall.ENOTDIR, which has the same value.
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) && pathErr.Op == "readdir" {
			return "ENOTDIR"
		}
		return "ENOENT"
	case syscall.ERROR_FILE_NOT_FOUND, 123 /* ERROR_INVALID_NAME */, 161, /* ERROR_BAD_PATHNAME */
		15 /* ERROR_INVALID_DRIVE */, 4392 /* ERROR_INVALID_REPARSE_DATA */ :
		return "ENOENT"
	case syscall.ERROR_ACCESS_DENIED, 1314 /* ERROR_PRIVILEGE_NOT_HELD */ :
		return "EPERM"
	case 740 /* ERROR_ELEVATION_REQUIRED */, 1920 /* ERROR_CANT_ACCESS_FILE */ :
		return "EACCES"
	case syscall.ERROR_ALREADY_EXISTS, syscall.ERROR_FILE_EXISTS:
		return "EEXIST"
	case syscall.ERROR_DIR_NOT_EMPTY:
		return "ENOTEMPTY"
	case 1 /* ERROR_INVALID_FUNCTION */ :
		return "EISDIR"
	case 87 /* ERROR_INVALID_PARAMETER */, 13 /* ERROR_INVALID_DATA */, 122, /* ERROR_INSUFFICIENT_BUFFER */
		1464 /* ERROR_SYMLINK_NOT_SUPPORTED */, 4390 /* ERROR_NOT_A_REPARSE_POINT */ :
		return "EINVAL"
	case 206 /* ERROR_FILENAME_EXCED_RANGE */, 111 /* ERROR_BUFFER_OVERFLOW */ :
		return "ENAMETOOLONG"
	case 112 /* ERROR_DISK_FULL */, 39 /* ERROR_HANDLE_DISK_FULL */, 82 /* ERROR_CANNOT_MAKE */ :
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
	case 31 /* ERROR_GEN_FAILURE */, 110 /* ERROR_OPEN_FAILED */, 1117 /* ERROR_IO_DEVICE */ :
		return "EIO"
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
