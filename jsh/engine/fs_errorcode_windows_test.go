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
	// Go opens directories with O_DIRECTORY, so Win32 reports a regular file
	// with the same code as a missing path; FS.ReadDir must still say ENOTDIR.
	mfs := NewFS()
	if err := mfs.Mount("/", os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	_, err = mfs.ReadDir("/file.txt")
	if got := FilesystemErrorCode(err); got != "ENOTDIR" {
		t.Fatalf("readdir regular file: FilesystemErrorCode(%v) = %q, want ENOTDIR", err, got)
	}
	_, err = mfs.ReadDir("/missing")
	if got := FilesystemErrorCode(err); got != "ENOENT" {
		t.Fatalf("readdir missing: FilesystemErrorCode(%v) = %q, want ENOENT", err, got)
	}
	_, err = mfs.ReadDir("/missing/child")
	if got := FilesystemErrorCode(err); got != "ENOENT" {
		t.Fatalf("readdir under missing parent: FilesystemErrorCode(%v) = %q, want ENOENT", err, got)
	}
}
