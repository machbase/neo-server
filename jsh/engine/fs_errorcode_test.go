package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"testing"
)

// platformMessageError mimics an OS error whose text is not a POSIX message,
// e.g. "The system cannot find the file specified." on Windows.
type platformMessageError struct {
	target error
}

func (e platformMessageError) Error() string        { return "platform specific message." }
func (e platformMessageError) Is(target error) bool { return target == e.target }

func TestFilesystemErrorCode(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{errors.New("unknown"), ""},
		{&fs.PathError{Op: "open", Path: "/x", Err: platformMessageError{fs.ErrNotExist}}, "ENOENT"},
		{&fs.PathError{Op: "open", Path: "/x", Err: platformMessageError{fs.ErrExist}}, "EEXIST"},
		{&fs.PathError{Op: "open", Path: "/x", Err: platformMessageError{fs.ErrPermission}}, "EACCES"},
		{fmt.Errorf("wrapped: %w", errors.ErrUnsupported), "ENOTSUP"},
		{fs.ErrInvalid, "EINVAL"},
		{&fs.PathError{Op: "rmdir", Path: "/x", Err: syscall.ENOTEMPTY}, "ENOTEMPTY"},
		{&fs.PathError{Op: "rmdir", Path: "/x", Err: syscall.ENOTDIR}, "ENOTDIR"},
		{&fs.PathError{Op: "unlink", Path: "/x", Err: syscall.EISDIR}, "EISDIR"},
		{&fs.PathError{Op: "truncate", Path: "/x", Err: syscall.EFBIG}, "EFBIG"},
		{&fs.PathError{Op: "open", Path: "/x", Err: syscall.EPERM}, "EPERM"},
		{&fs.PathError{Op: "open", Path: "/x", Err: syscall.EACCES}, "EACCES"},
	}
	for _, tc := range tests {
		if got := FilesystemErrorCode(tc.err); got != tc.want {
			t.Errorf("FilesystemErrorCode(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}
