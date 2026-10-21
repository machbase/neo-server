package service

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

// startSharedFSController starts a controller whose shared filesystem is
// reachable over RPC and returns a ControllerFS client connected to it.
func startSharedFSController(t *testing.T) (*Controller, *engine.ControllerFS) {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/services", 0o755); err != nil {
		t.Fatalf("MkdirAll(services) error: %v", err)
	}
	ctl, err := NewController(&ControllerConfig{
		ConfigDir: "/work/services",
		Mounts:    []engine.FSTab{{MountPoint: "/work", FS: os.DirFS(tmpDir)}},
	})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	if err := ctl.Start(nil); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	t.Cleanup(func() { ctl.Stop(nil) })

	remote, err := engine.NewControllerFS(ctl.Address())
	if err != nil {
		t.Fatalf("NewControllerFS() error: %v", err)
	}
	t.Cleanup(func() { remote.Close() })
	return ctl, remote
}

func TestSharedFSRPCPreservesErrnoCodes(t *testing.T) {
	ctl, remote := startSharedFSController(t)
	if err := ctl.sharedWriteFile("/file.txt", []byte("seed")); err != nil {
		t.Fatalf("sharedWriteFile() error: %v", err)
	}
	if err := ctl.sharedMkdir("/dir", 0o755); err != nil {
		t.Fatalf("sharedMkdir() error: %v", err)
	}
	if err := ctl.sharedWriteFile("/dir/child.txt", []byte("x")); err != nil {
		t.Fatalf("sharedWriteFile(child) error: %v", err)
	}

	tests := []struct {
		name  string
		err   error
		code  string
		errno syscall.Errno
	}{
		{"mkdir existing", remote.MkdirMode("dir", 0o755), "EEXIST", syscall.EEXIST},
		{"rmdir non-empty", remote.Rmdir("dir"), "ENOTEMPTY", syscall.ENOTEMPTY},
		{"rmdir file", remote.Rmdir("file.txt"), "ENOTDIR", engine.ErrnoNotDir},
		{"unlink directory", remote.Unlink("dir"), "EISDIR", syscall.EISDIR},
		{"truncate oversized", remote.Truncate("file.txt", engine.MaxVirtualFileSize+1), "EFBIG", syscall.EFBIG},
		{"mkdir missing parent", remote.MkdirMode("missing/child", 0o755), "ENOENT", syscall.ENOENT},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if code := engine.FilesystemErrorCode(tc.err); code != tc.code {
				t.Fatalf("FilesystemErrorCode(%v) = %q, want %q", tc.err, code, tc.code)
			}
			if !errors.Is(tc.err, tc.errno) {
				t.Fatalf("errors.Is(%v, %v) = false", tc.err, tc.errno)
			}
			var pathErr *fs.PathError
			if !errors.As(tc.err, &pathErr) {
				t.Fatalf("error %T is not *fs.PathError", tc.err)
			}
		})
	}
}

func TestSharedFSZeroLengthWriteDoesNotExtendFile(t *testing.T) {
	ctl, err := NewController(&ControllerConfig{
		ConfigDir: "/work/services",
		Mounts:    []engine.FSTab{{MountPoint: "/work", FS: os.DirFS(t.TempDir())}},
	})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	if err := ctl.sharedWriteFile("/z.txt", []byte("seed")); err != nil {
		t.Fatalf("sharedWriteFile() error: %v", err)
	}
	fd, err := ctl.sharedOpenFD("/z.txt", os.O_RDWR, 0, "")
	if err != nil {
		t.Fatalf("sharedOpenFD() error: %v", err)
	}
	position := int64(100)
	result, err := ctl.sharedWriteFD(fd.FD, []byte{}, &position)
	if err != nil || result.BytesWritten != 0 {
		t.Fatalf("sharedWriteFD(empty, position=100) = %+v, %v", result, err)
	}
	if _, err := ctl.sharedWriteFD(fd.FD, []byte{}); err != nil {
		t.Fatalf("sharedWriteFD(empty) error: %v", err)
	}
	if err := ctl.sharedCloseFD(fd.FD); err != nil {
		t.Fatalf("sharedCloseFD() error: %v", err)
	}
	data, err := ctl.sharedReadFile("/z.txt")
	if err != nil || string(data) != "seed" {
		t.Fatalf("file content = %q, %v; want unchanged %q", data, err, "seed")
	}
}

func TestSharedFSRootIsBusy(t *testing.T) {
	_, sharedFS := startSharedFSController(t)
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"rmdir", sharedFS.Rmdir("/")},
		{"remove", sharedFS.Remove("/")},
		{"unlink", sharedFS.Unlink("/")},
		{"rename", sharedFS.Rename("/", "/moved")},
		{"rename dest", sharedFS.Rename("/missing", "/")},
	} {
		if !errors.Is(tc.err, syscall.EBUSY) || engine.FilesystemErrorCode(tc.err) != "EBUSY" {
			t.Errorf("%s(/) error=%v code=%q, want EBUSY", tc.name, tc.err, engine.FilesystemErrorCode(tc.err))
		}
	}
}
