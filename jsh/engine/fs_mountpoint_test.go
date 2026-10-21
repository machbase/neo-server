package engine

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"
)

func TestFS_AccessUsesOwnerPermissionBits(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.WriteFile("owner.txt", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := vfs.Chmod("owner.txt", 0o600); err != nil {
		t.Fatal(err)
	}
	if err := vfs.WriteFile("other.txt", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := vfs.Chmod("other.txt", 0o004); err != nil {
		t.Fatal(err)
	}
	mfs := NewFS()
	if err := mfs.Mount("/v", vfs); err != nil {
		t.Fatal(err)
	}

	if err := mfs.Access("/v/owner.txt", 4|2); err != nil {
		t.Fatalf("Access(0600, R_OK|W_OK) error=%v, want nil", err)
	}
	if err := mfs.Access("/v/owner.txt", 1); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Access(0600, X_OK) error=%v, want %v", err, fs.ErrPermission)
	}
	if err := mfs.Access("/v/other.txt", 4); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Access(0004, R_OK) error=%v, want %v", err, fs.ErrPermission)
	}

	// A read-only filesystem rejects W_OK even when the owner bit is set.
	if err := mfs.Mount("/ro", fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("x"), Mode: 0o644},
	}); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Access("/ro/file.txt", 4); err != nil {
		t.Fatalf("Access(read-only, R_OK) error=%v, want nil", err)
	}
	if err := mfs.Access("/ro/file.txt", 2); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Access(read-only, W_OK) error=%v, want %v", err, fs.ErrPermission)
	}
}

func TestFS_LstatVirtualMountDirectory(t *testing.T) {
	mfs := NewFS()
	if err := mfs.Mount("/", NewVirtualFS()); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Mount("/a/b", NewVirtualFS()); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Mount("/x/y", os.DirFS(t.TempDir())); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"/a", "/x"} {
		stat, err := mfs.Stat(name)
		if err != nil || !stat.IsDir() {
			t.Fatalf("Stat(%s) = %v, %v; want directory", name, stat, err)
		}
		lstat, err := mfs.Lstat(name)
		if err != nil || !lstat.IsDir() {
			t.Fatalf("Lstat(%s) = %v, %v; want directory", name, lstat, err)
		}
	}
	if _, err := mfs.Lstat("/a/missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Lstat(missing) error=%v, want %v", err, fs.ErrNotExist)
	}

	bare := NewFS()
	if err := bare.Mount("/p/q", NewVirtualFS()); err != nil {
		t.Fatal(err)
	}
	if info, err := bare.Lstat("/p"); err != nil || !info.IsDir() {
		t.Fatalf("Lstat(/p) without root mount = %v, %v; want directory", info, err)
	}
	if _, err := bare.Lstat("/none"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Lstat(/none) error=%v, want %v", err, fs.ErrNotExist)
	}
}

func TestFS_MountPointsCannotBeRemovedOrRenamed(t *testing.T) {
	hostDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostDir, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyHost := t.TempDir()

	mfs := NewFS()
	if err := mfs.Mount("/", NewVirtualFS()); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Mount("/host", os.DirFS(hostDir)); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Mount("/empty", os.DirFS(emptyHost)); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Mount("/mem", NewVirtualFS()); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Mount("/nest/deep", NewVirtualFS()); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Mkdir("/dir"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"/", "/host", "/empty", "/mem", "/nest", "/nest/deep"} {
		if err := mfs.Rmdir(name); !errors.Is(err, syscall.EBUSY) {
			t.Errorf("Rmdir(%s) error=%v, want EBUSY", name, err)
		}
		if err := mfs.Unlink(name); !errors.Is(err, syscall.EBUSY) {
			t.Errorf("Unlink(%s) error=%v, want EBUSY", name, err)
		}
		if err := mfs.Remove(name); !errors.Is(err, syscall.EBUSY) {
			t.Errorf("Remove(%s) error=%v, want EBUSY", name, err)
		}
		if code := FilesystemErrorCode(mfs.Rmdir(name)); code != "EBUSY" {
			t.Errorf("Rmdir(%s) code=%q, want EBUSY", name, code)
		}
	}
	if _, err := os.Stat(emptyHost); err != nil {
		t.Fatalf("empty host mount directory was removed: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(hostDir, "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("host mount content changed: %q, %v", data, err)
	}

	renames := [][2]string{
		{"/host", "/dir/moved"},
		{"/mem", "/moved"},
		{"/nest", "/moved"},
		{"/dir", "/mem"},
		{"/dir", "/nest"},
	}
	for _, rename := range renames {
		if err := mfs.Rename(rename[0], rename[1]); !errors.Is(err, syscall.EBUSY) {
			t.Errorf("Rename(%s, %s) error=%v, want EBUSY", rename[0], rename[1], err)
		}
	}

	// Entries inside a mount point stay removable.
	if err := mfs.Unlink("/host/keep.txt"); err != nil {
		t.Fatalf("Unlink inside mount error=%v", err)
	}
	if err := mfs.Rmdir("/dir"); err != nil {
		t.Fatalf("Rmdir regular directory error=%v", err)
	}
}

func TestVirtualFS_RmdirRootIsBusy(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.Rmdir("."); !errors.Is(err, syscall.EBUSY) {
		t.Fatalf("VirtualFS.Rmdir(.) error=%v, want EBUSY", err)
	}
}

func TestFS_AccessVirtualMountDirectoryMatchesStat(t *testing.T) {
	for _, tc := range []struct {
		name string
		root fs.FS
	}{
		{"host root", os.DirFS(t.TempDir())},
		{"virtual root", NewVirtualFS()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mfs := NewFS()
			if err := mfs.Mount("/", tc.root); err != nil {
				t.Fatal(err)
			}
			if err := mfs.Mount("/a/b", os.DirFS(t.TempDir())); err != nil {
				t.Fatal(err)
			}
			info, err := mfs.Stat("/a")
			if err != nil || !info.IsDir() {
				t.Fatalf("Stat(/a) = %v, %v; want a directory", info, err)
			}
			for _, mode := range []uint32{0, 4, 1, 4 | 1} {
				if err := mfs.Access("/a", mode); err != nil {
					t.Fatalf("Access(/a, %d) error=%v, want nil", mode, err)
				}
			}
			if err := mfs.Access("/a/missing", 0); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("Access(/a/missing) error=%v, want ENOENT", err)
			}
		})
	}
}
