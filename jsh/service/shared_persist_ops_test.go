package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

func newPersistTestController(t *testing.T, withBackend bool) (*Controller, string) {
	t.Helper()
	tmpDir := t.TempDir()
	backendDir := filepath.Join(tmpDir, "shared-backend")
	for _, dir := range []string{filepath.Join(tmpDir, "services"), backendDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error: %v", dir, err)
		}
	}
	conf := &ControllerConfig{
		ConfigDir: "/work/services",
		Mounts:    []engine.FSTab{{MountPoint: "/work", FS: os.DirFS(tmpDir)}},
	}
	if withBackend {
		conf.SharedFS.BackendDir = "/work/shared-backend"
	}
	ctl, err := NewController(conf)
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	return ctl, backendDir
}

func requireSharedCode(t *testing.T, label string, err error, want string) {
	t.Helper()
	if got := engine.FilesystemErrorCode(err); got != want {
		t.Fatalf("%s: code=%q (err=%v), want %q", label, got, err, want)
	}
}

func TestSharedFSMutationsWithoutBackend(t *testing.T) {
	ctl, _ := newPersistTestController(t, false)
	if err := ctl.sharedMkdir("/dir", 0o755); err != nil {
		t.Fatalf("sharedMkdir error: %v", err)
	}
	if err := ctl.sharedWriteFile("/dir/file.txt", []byte("hello")); err != nil {
		t.Fatalf("sharedWriteFile error: %v", err)
	}
	if err := ctl.sharedTruncate("/dir/file.txt", 2); err != nil {
		t.Fatalf("sharedTruncate error: %v", err)
	}
	if data, err := ctl.sharedReadFile("/dir/file.txt"); err != nil || string(data) != "he" {
		t.Fatalf("truncated content=%q err=%v, want %q", data, err, "he")
	}
	if err := ctl.sharedUnlink("/dir/file.txt"); err != nil {
		t.Fatalf("sharedUnlink error: %v", err)
	}
	if err := ctl.sharedRmdir("/dir"); err != nil {
		t.Fatalf("sharedRmdir error: %v", err)
	}
	requireSharedCode(t, "rmdir root", ctl.sharedRmdir("/"), "EBUSY")
	requireSharedCode(t, "remove root", ctl.sharedRemove("/"), "EBUSY")
	requireSharedCode(t, "unlink root", ctl.sharedUnlink("/"), "EBUSY")
	requireSharedCode(t, "rename root", ctl.sharedRename("/", "/x"), "EBUSY")
}

func TestSharedFSPersistRollsBackOnBackendMismatch(t *testing.T) {
	ctl, backendDir := newPersistTestController(t, true)
	if err := ctl.sharedMkdir("/dir", 0o755); err != nil {
		t.Fatalf("sharedMkdir error: %v", err)
	}
	if err := ctl.sharedWriteFile("/file.txt", []byte("data")); err != nil {
		t.Fatalf("sharedWriteFile error: %v", err)
	}

	// Swap the backend kinds so persistence disagrees with the virtual tree.
	if err := os.Remove(filepath.Join(backendDir, "dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backendDir, "dir"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(backendDir, "file.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(backendDir, "file.txt"), 0o755); err != nil {
		t.Fatal(err)
	}

	requireSharedCode(t, "rmdir over backend file", ctl.sharedRmdir("/dir"), "EINVAL")
	if info, err := ctl.sharedStat("/dir"); err != nil || !info.IsDir {
		t.Fatalf("rmdir should roll back: info=%+v err=%v", info, err)
	}
	requireSharedCode(t, "unlink over backend dir", ctl.sharedUnlink("/file.txt"), "EINVAL")
	if _, err := ctl.sharedStat("/file.txt"); err != nil {
		t.Fatalf("unlink should roll back: %v", err)
	}
	if err := ctl.sharedTruncate("/file.txt", 1); err == nil {
		t.Fatal("truncate over backend dir should fail")
	}
	if data, err := ctl.sharedReadFile("/file.txt"); err != nil || string(data) != "data" {
		t.Fatalf("truncate should roll back: data=%q err=%v", data, err)
	}
}

func TestSharedFSPersistToleratesMissingBackendEntries(t *testing.T) {
	ctl, backendDir := newPersistTestController(t, true)
	if err := ctl.sharedMkdir("/empty", 0o755); err != nil {
		t.Fatalf("sharedMkdir error: %v", err)
	}
	if err := ctl.sharedMkdir("/tree", 0o755); err != nil {
		t.Fatalf("sharedMkdir error: %v", err)
	}
	for _, name := range []string{"/tree/a.txt", "/gone.txt", "/gone-remove.txt"} {
		if err := ctl.sharedWriteFile(name, []byte("x")); err != nil {
			t.Fatalf("sharedWriteFile(%s) error: %v", name, err)
		}
	}
	for _, name := range []string{"empty", "gone.txt", "gone-remove.txt"} {
		if err := os.Remove(filepath.Join(backendDir, name)); err != nil {
			t.Fatal(err)
		}
	}

	if err := ctl.sharedRmdir("/empty"); err != nil {
		t.Fatalf("rmdir with missing backend dir: %v", err)
	}
	if err := ctl.sharedUnlink("/gone.txt"); err != nil {
		t.Fatalf("unlink with missing backend file: %v", err)
	}
	if err := ctl.sharedRemove("/gone-remove.txt"); err != nil {
		t.Fatalf("remove with missing backend file: %v", err)
	}
	if err := ctl.sharedRemove("/tree"); err != nil {
		t.Fatalf("remove backend directory tree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backendDir, "tree")); !os.IsNotExist(err) {
		t.Fatalf("backend tree should be removed, stat err=%v", err)
	}
}

func TestSharedFSDescriptorArgumentErrors(t *testing.T) {
	ctl, _ := newPersistTestController(t, false)
	if err := ctl.sharedWriteFile("/fd.txt", []byte("abc")); err != nil {
		t.Fatalf("sharedWriteFile error: %v", err)
	}
	ro, err := ctl.sharedOpenFD("/fd.txt", os.O_RDONLY, 0, "")
	if err != nil {
		t.Fatalf("open ro: %v", err)
	}
	wo, err := ctl.sharedOpenFD("/fd.txt", os.O_WRONLY, 0, "")
	if err != nil {
		t.Fatalf("open wo: %v", err)
	}
	negative := int64(-1)
	requireSharedCode(t, "read negative length", func() error { _, err := ctl.sharedReadFD(ro.FD, -1); return err }(), "EINVAL")
	requireSharedCode(t, "read negative position", func() error { _, err := ctl.sharedReadFD(ro.FD, 1, &negative); return err }(), "EINVAL")
	requireSharedCode(t, "read write-only", func() error { _, err := ctl.sharedReadFD(wo.FD, 1); return err }(), "EACCES")
	requireSharedCode(t, "write negative position", func() error { _, err := ctl.sharedWriteFD(wo.FD, []byte("z"), &negative); return err }(), "EINVAL")
	requireSharedCode(t, "write read-only", func() error { _, err := ctl.sharedWriteFD(ro.FD, []byte("z")); return err }(), "EACCES")
	if res, err := ctl.sharedReadFD(ro.FD, 0); err != nil || res.BytesRead != 0 {
		t.Fatalf("zero-length read: res=%+v err=%v", res, err)
	}
}
