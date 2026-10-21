//go:build windows

package engine

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFilesystemErrorCodeWindowsPathNotFound(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{&fs.PathError{Op: "lstat", Path: `C:\x\y`, Err: syscall.ERROR_PATH_NOT_FOUND}, "ENOENT"},
		{&fs.PathError{Op: "open", Path: `C:\x\y`, Err: syscall.ERROR_PATH_NOT_FOUND}, "ENOENT"},
		{&fs.PathError{Op: "readdir", Path: `C:\x`, Err: syscall.ENOTDIR}, "ENOTDIR"},
		{&fs.PathError{Op: "rmdir", Path: "/x", Err: ErrnoNotDir}, "ENOTDIR"},
	}
	for _, tc := range tests {
		if got := FilesystemErrorCode(tc.err); got != tc.want {
			t.Errorf("FilesystemErrorCode(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}

	dir := t.TempDir()
	_, err := os.Lstat(filepath.Join(dir, "missing", "child"))
	if got := FilesystemErrorCode(err); got != "ENOENT" {
		t.Fatalf("lstat under a missing parent: FilesystemErrorCode(%v) = %q, want ENOENT", err, got)
	}

	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = os.Readlink(file)
	if got := FilesystemErrorCode(err); got != "EINVAL" {
		t.Fatalf("readlink regular file: FilesystemErrorCode(%v) = %q, want EINVAL", err, got)
	}
	_, err = os.ReadDir(file)
	if got := FilesystemErrorCode(err); got != "ENOTDIR" {
		t.Fatalf("readdir regular file: FilesystemErrorCode(%v) = %q, want ENOTDIR", err, got)
	}
}
