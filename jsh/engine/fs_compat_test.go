package engine

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"testing/fstest"
)

type capabilityReadFS struct {
	fstest.MapFS
	opens int
	stats int
	reads int
}

func (c *capabilityReadFS) Open(name string) (fs.File, error) {
	c.opens++
	return c.MapFS.Open(name)
}

func (c *capabilityReadFS) Stat(name string) (fs.FileInfo, error) {
	c.stats++
	return c.MapFS.Stat(name)
}

func (c *capabilityReadFS) ReadFile(name string) ([]byte, error) {
	c.reads++
	return c.MapFS.ReadFile(name)
}

func TestFS_UsesBackendReadCapabilities(t *testing.T) {
	backend := &capabilityReadFS{MapFS: fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("payload")},
	}}
	mfs := NewFS()
	if err := mfs.Mount("/work", backend); err != nil {
		t.Fatal(err)
	}
	backend.opens, backend.stats, backend.reads = 0, 0, 0
	info, err := mfs.Stat("/work/file.txt")
	if err != nil || info.Size() != 7 {
		t.Fatalf("Stat() = %v, %v", info, err)
	}
	data, err := mfs.ReadFile("/work/file.txt")
	if err != nil || string(data) != "payload" {
		t.Fatalf("ReadFile() = %q, %v", data, err)
	}
	if backend.opens != 0 || backend.stats != 1 || backend.reads != 1 {
		t.Fatalf("backend calls: open=%d stat=%d read=%d", backend.opens, backend.stats, backend.reads)
	}
	backend.stats = 0
	entries, err := mfs.ReadDir("/work")
	if err != nil || len(entries) != 3 {
		t.Fatalf("ReadDir() = %v, %v", entries, err)
	}
	for _, entry := range entries[:2] {
		info, err := entry.Info()
		if err != nil || !info.IsDir() {
			t.Fatalf("dot entry Info() = %v, %v", info, err)
		}
	}
	if backend.opens != 0 || backend.stats != 1 {
		t.Fatalf("root directory metadata calls: open=%d stat=%d", backend.opens, backend.stats)
	}
}

func TestFS_ReadCapabilitiesFallback(t *testing.T) {
	mfs := NewFS()
	backend := struct{ fs.FS }{fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("payload")},
	}}
	if err := mfs.Mount("/work", backend); err != nil {
		t.Fatal(err)
	}
	info, err := mfs.Stat("/work/file.txt")
	if err != nil || info.Size() != 7 {
		t.Fatalf("Stat() = %v, %v", info, err)
	}
	data, err := mfs.ReadFile("/work/file.txt")
	if err != nil || string(data) != "payload" {
		t.Fatalf("ReadFile() = %q, %v", data, err)
	}
	if err := mfs.Mount("/work/implied/child", fstest.MapFS{}); err != nil {
		t.Fatal(err)
	}
	info, err = mfs.Stat("/work/implied")
	if err != nil || !info.IsDir() {
		t.Fatalf("Stat(implied) = %v, %v", info, err)
	}
	if _, err := mfs.ReadFile("/work/implied"); err == nil {
		t.Fatal("ReadFile(implied) must reject a directory")
	}
	for _, path := range []string{"/work/missing.txt", "/unmounted"} {
		if _, err := mfs.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Stat(%q) error=%v, want not exist", path, err)
		}
		if _, err := mfs.ReadFile(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("ReadFile(%q) error=%v, want not exist", path, err)
		}
	}
	invalidPath := "/work/\xff"
	if _, err := mfs.Stat(invalidPath); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("Stat(invalid) error=%v, want invalid", err)
	}
	if _, err := mfs.ReadFile(invalidPath); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("ReadFile(invalid) error=%v, want invalid", err)
	}
}

func TestFS_MountStrictRejectsConflictWithoutMutation(t *testing.T) {
	for _, childFirst := range []bool{false, true} {
		name := "parent-first"
		if childFirst {
			name = "child-first"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "data"), 0o755); err != nil {
				t.Fatal(err)
			}
			mfs := NewFS()
			firstPoint, secondPoint := "/work", "/work/data"
			firstFS, secondFS := os.DirFS(root), os.DirFS(t.TempDir())
			if childFirst {
				firstPoint, secondPoint = secondPoint, firstPoint
				firstFS, secondFS = secondFS, firstFS
			}
			if err := mfs.MountStrict(firstPoint, firstFS); err != nil {
				t.Fatal(err)
			}
			if err := mfs.MountStrict(secondPoint, secondFS); !errors.Is(err, ErrMountConflict) {
				t.Fatalf("MountStrict() error=%v, want mount conflict", err)
			}
			if got := mfs.Mounts(); !reflect.DeepEqual(got, []string{firstPoint}) {
				t.Fatalf("failed MountStrict changed mounts: %v", got)
			}
			if warnings := mfs.MountWarnings(); len(warnings) != 0 {
				t.Fatalf("strict mounts must return errors, not warnings: %v", warnings)
			}
		})
	}
}

func TestFS_ResolveHostMountAndVirtualDirectory(t *testing.T) {
	mfs := NewFS()
	root := t.TempDir()
	if err := mfs.MountStrict("/work", os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	mount, relative, host, err := mfs.ResolveHostMount("/work/missing.txt")
	if err != nil || mount != "/work" || relative != "missing.txt" || host != filepath.Join(root, "missing.txt") {
		t.Fatalf("ResolveHostMount() = %q, %q, %q, %v", mount, relative, host, err)
	}
	if _, err := mfs.Stat("/work/missing.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat(missing) error=%v, want not exist", err)
	}
	if err := mfs.MountStrict("/work/apps/deep", NewVirtualFS()); err != nil {
		t.Fatal(err)
	}
	if virtual, err := mfs.IsVirtualDirectory("/work/apps"); err != nil || !virtual {
		t.Fatalf("IsVirtualDirectory(implied) = %v, %v", virtual, err)
	}
	if virtual, err := mfs.IsVirtualDirectory("/work"); err != nil || virtual {
		t.Fatalf("IsVirtualDirectory(backed) = %v, %v", virtual, err)
	}
}

func TestFS_RenameHostAcrossMounts(t *testing.T) {
	mfs := NewFS()
	source, destination := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mfs.MountStrict("/src", os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	if err := mfs.MountStrict("/dst", os.DirFS(destination)); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Rename("/src/file.txt", "/dst/file.txt"); !errors.Is(err, syscall.EXDEV) {
		t.Fatalf("Rename() error=%v, want EXDEV", err)
	}
	if err := mfs.RenameHost("/src/file.txt", "/dst/file.txt"); err != nil {
		t.Fatalf("RenameHost() error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "file.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("RenameHost left source: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(destination, "file.txt")); err != nil || string(data) != "data" {
		t.Fatalf("RenameHost destination=%q, %v", data, err)
	}
	if err := mfs.RenameHost("/dst", "/src/moved"); !errors.Is(err, syscall.EBUSY) {
		t.Fatalf("RenameHost(mount) error=%v, want EBUSY", err)
	}
	if err := mfs.MountStrict("/mem", fstest.MapFS{}); err != nil {
		t.Fatal(err)
	}
	if err := mfs.RenameHost("/dst/file.txt", "/mem/file.txt"); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("RenameHost(non-host) error=%v, want permission error", err)
	}
}
