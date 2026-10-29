package engine

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"
)

func newHostMountedFS(t *testing.T) (*FS, string) {
	t.Helper()
	dir := t.TempDir()
	mfs := NewFS()
	if err := mfs.Mount("/", os.DirFS(dir)); err != nil {
		t.Fatalf("Mount(/) error: %v", err)
	}
	return mfs, dir
}

func requireFSCode(t *testing.T, label string, err error, want string) {
	t.Helper()
	if got := FilesystemErrorCode(err); got != want {
		t.Fatalf("%s: code=%q (err=%v), want %q", label, got, err, want)
	}
}

func TestFS_HostMountDirectoryOperations(t *testing.T) {
	mfs, dir := newHostMountedFS(t)

	if err := mfs.MkdirAll("/a/b/c"); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := mfs.MkdirAll("/a/b/c"); err != nil {
		t.Fatalf("MkdirAll(existing) error: %v", err)
	}
	if err := mfs.MkdirAll("/"); err != nil {
		t.Fatalf("MkdirAll(/) error: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "a", "b", "c")); err != nil || !info.IsDir() {
		t.Fatalf("MkdirAll did not create the directory: %v", err)
	}
	if err := mfs.WriteFile("/a/file.txt", []byte("abcdef")); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	requireFSCode(t, "MkdirAll over a file", mfs.MkdirAll("/a/file.txt/x"), "EEXIST")

	if err := mfs.Truncate("/a/file.txt", 3); err != nil {
		t.Fatalf("Truncate() error: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "a", "file.txt")); err != nil || string(data) != "abc" {
		t.Fatalf("Truncate() content=%q err=%v, want abc", data, err)
	}
	requireFSCode(t, "Truncate missing", mfs.Truncate("/a/missing.txt", 0), "ENOENT")

	requireFSCode(t, "Rmdir file", mfs.Rmdir("/a/file.txt"), "ENOTDIR")
	requireFSCode(t, "Rmdir missing", mfs.Rmdir("/a/missing"), "ENOENT")
	requireFSCode(t, "Unlink directory", mfs.Unlink("/a/b"), "EISDIR")
	requireFSCode(t, "Unlink missing", mfs.Unlink("/a/missing.txt"), "ENOENT")
	if _, err := os.Stat(filepath.Join(dir, "a", "file.txt")); err != nil {
		t.Fatalf("failed Rmdir must keep the file: %v", err)
	}

	if err := mfs.Rmdir("/a/b/c"); err != nil {
		t.Fatalf("Rmdir() error: %v", err)
	}
	if err := mfs.Unlink("/a/file.txt"); err != nil {
		t.Fatalf("Unlink() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "file.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Unlink() left the file: %v", err)
	}
}

func TestFS_RemoveAllHostTree(t *testing.T) {
	mfs, root := newHostMountedFS(t)
	tree := filepath.Join(root, "tree")
	if err := os.MkdirAll(filepath.Join(tree, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tree, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "nested", "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	requireFSCode(t, "Remove(non-empty directory)", mfs.Remove("/tree"), "ENOTEMPTY")
	if _, err := os.Stat(filepath.Join(tree, "nested", "file.txt")); err != nil {
		t.Fatalf("non-recursive Remove changed the tree: %v", err)
	}
	if err := mfs.Rmdir("/tree/empty"); err != nil {
		t.Fatalf("Rmdir(empty directory) error=%v", err)
	}

	if err := mfs.RemoveAll("/tree"); err != nil {
		t.Fatalf("RemoveAll(tree) error=%v", err)
	}
	if _, err := os.Lstat(tree); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("RemoveAll left the tree: %v", err)
	}
	if err := mfs.RemoveAll("/missing"); err != nil {
		t.Fatalf("RemoveAll(missing) error=%v, want nil", err)
	}
}

func TestFS_RemoveAllUnlinksSymlinkWithoutFollowing(t *testing.T) {
	mfs, root := newHostMountedFS(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	if err := mfs.RemoveAll("/link"); err != nil {
		t.Fatalf("RemoveAll(link) error=%v", err)
	}
	if _, err := os.Lstat(link); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("RemoveAll left the symlink: %v", err)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "keep" {
		t.Fatalf("RemoveAll followed the symlink: content=%q err=%v", data, err)
	}
}

func TestFS_RemoveAllRejectsMountAndAncestorBeforeDeleting(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(parent, "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	mfs := NewFS()
	if err := mfs.Mount("/", os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	mountDir := t.TempDir()
	if err := mfs.Mount("/parent/nested", os.DirFS(mountDir)); err != nil {
		t.Fatal(err)
	}

	if err := mfs.RemoveAll("/parent"); !errors.Is(err, syscall.EBUSY) {
		t.Fatalf("RemoveAll(ancestor) error=%v, want EBUSY", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("RemoveAll changed the ancestor before rejecting: %v", err)
	}
	if err := mfs.RemoveAll("/parent/nested"); !errors.Is(err, syscall.EBUSY) {
		t.Fatalf("RemoveAll(mount point) error=%v, want EBUSY", err)
	}
}

type removeAllFailUnlinkFS struct {
	*VirtualFS
}

func (vfs *removeAllFailUnlinkFS) Unlink(name string) error {
	if name == "tree/b.txt" {
		return fs.ErrPermission
	}
	return vfs.VirtualFS.Unlink(name)
}

func TestFS_RemoveAllReturnsPartialFailure(t *testing.T) {
	vfs := &removeAllFailUnlinkFS{VirtualFS: NewVirtualFS()}
	if err := vfs.Mkdir("tree"); err != nil {
		t.Fatal(err)
	}
	if err := vfs.WriteFile("tree/a.txt", []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := vfs.WriteFile("tree/b.txt", []byte("b")); err != nil {
		t.Fatal(err)
	}
	mfs := NewFS()
	if err := mfs.Mount("/mem", vfs); err != nil {
		t.Fatal(err)
	}

	if err := mfs.RemoveAll("/mem/tree"); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("RemoveAll error=%v, want permission error", err)
	}
	if _, err := mfs.Stat("/mem/tree/a.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("entry before the failure should have been removed: %v", err)
	}
	if _, err := mfs.Stat("/mem/tree/b.txt"); err != nil {
		t.Fatalf("entry that failed to remove should remain: %v", err)
	}
}

func TestFS_HostPathConversionRequiresOSBackedMount(t *testing.T) {
	mfs := NewFS()
	if err := mfs.Mount("/mem", fstest.MapFS{"file.txt": {Data: []byte("x")}}); err != nil {
		t.Fatal(err)
	}

	if _, err := mfs.OSPath("/mem/file.txt"); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("OSPath(non-OS mount) error=%v, want permission error", err)
	}
	if _, err := mfs.ResolveHostPath("/mem/file.txt"); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("ResolveHostPath(non-OS mount) error=%v, want permission error", err)
	}
}

func TestFS_LstatAcrossMountKinds(t *testing.T) {
	mfs, _ := newHostMountedFS(t)
	if err := mfs.Mount("/mem", fstest.MapFS{"file.txt": {Data: []byte("x")}}); err != nil {
		t.Fatalf("Mount(/mem) error: %v", err)
	}
	if err := mfs.Mount("/deep/nested/mount", NewVirtualFS()); err != nil {
		t.Fatalf("Mount(nested) error: %v", err)
	}

	info, err := mfs.Lstat("/mem/file.txt")
	if err != nil || info.IsDir() || info.Size() != 1 {
		t.Fatalf("Lstat(non-OS file) = %v, %v", info, err)
	}
	requireFSCode(t, "Lstat non-OS missing", func() error { _, err := mfs.Lstat("/mem/missing"); return err }(), "ENOENT")
	requireFSCode(t, "Lstat host missing", func() error { _, err := mfs.Lstat("/missing"); return err }(), "ENOENT")

	// "/deep/nested" exists only because a mount point is nested below it.
	info, err = mfs.Lstat("/deep/nested")
	if err != nil || !info.IsDir() {
		t.Fatalf("Lstat(virtual mount dir) = %v, %v; want directory", info, err)
	}

	empty := NewFS()
	if err := empty.Mount("/only/here", NewVirtualFS()); err != nil {
		t.Fatalf("Mount(/only/here) error: %v", err)
	}
	if info, err := empty.Lstat("/only"); err != nil || !info.IsDir() {
		t.Fatalf("Lstat(unbacked virtual dir) = %v, %v; want directory", info, err)
	}
	requireFSCode(t, "Lstat without backing mount", func() error { _, err := empty.Lstat("/nowhere"); return err }(), "ENOENT")
}

func TestFS_AccessRejectsInvalidMode(t *testing.T) {
	mfs, _ := newHostMountedFS(t)
	if err := mfs.WriteFile("/f.txt", []byte("x")); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	requireFSCode(t, "Access invalid mode", mfs.Access("/f.txt", 8), "EINVAL")
	if err := mfs.Access("/f.txt", 4); err != nil {
		t.Fatalf("Access(R_OK) error: %v", err)
	}
}

func TestFS_PositionedDescriptorArgumentValidation(t *testing.T) {
	mfs, dir := newHostMountedFS(t)
	if err := mfs.WriteFile("/fd.txt", []byte("0123456789")); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	buf := make([]byte, 4)
	requireFSCode(t, "ReadAtFD unknown fd", func() error { _, err := mfs.ReadAtFD(999, buf, 0, 4, 0); return err }(), "EINVAL")
	requireFSCode(t, "WriteAtFD unknown fd", func() error { _, err := mfs.WriteAtFD(999, buf, 0); return err }(), "EINVAL")

	fd, err := mfs.OpenFD("/fd.txt", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("OpenFD() error: %v", err)
	}
	defer mfs.CloseFD(fd)
	for _, tc := range []struct {
		name           string
		offset, length int
		position       int64
	}{
		{"negative offset", -1, 1, 0},
		{"negative length", 0, -1, 0},
		{"offset past buffer", 5, 0, 0},
		{"length past buffer", 2, 3, 0},
		{"negative position", 0, 1, -1},
	} {
		requireFSCode(t, "ReadAtFD "+tc.name, func() error { _, err := mfs.ReadAtFD(fd, buf, tc.offset, tc.length, tc.position); return err }(), "EINVAL")
	}
	requireFSCode(t, "WriteAtFD negative position", func() error { _, err := mfs.WriteAtFD(fd, buf, -1); return err }(), "EINVAL")

	// Reading past the end returns the bytes available without an error.
	n, err := mfs.ReadAtFD(fd, buf, 0, 4, 8)
	if err != nil || n != 2 || string(buf[:n]) != "89" {
		t.Fatalf("ReadAtFD(near EOF) = %d, %q, %v; want 2, 89, nil", n, buf[:n], err)
	}

	appendFD, err := mfs.OpenFD("/fd.txt", os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("OpenFD(append) error: %v", err)
	}
	// Like Linux pwrite on an O_APPEND descriptor, the position is ignored.
	if n, err := mfs.WriteAtFD(appendFD, []byte("AB"), 0); err != nil || n != 2 {
		t.Fatalf("WriteAtFD(append) = %d, %v", n, err)
	}
	if err := mfs.CloseFD(appendFD); err != nil {
		t.Fatalf("CloseFD(append) error: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "fd.txt")); err != nil || string(data) != "0123456789AB" {
		t.Fatalf("append content=%q err=%v", data, err)
	}
}

type noReaderAtFS struct{}

type noReaderAtFile struct{ io.Reader }

func (noReaderAtFS) Open(name string) (fs.File, error) {
	if name != "plain.txt" {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return noReaderAtFile{Reader: io.LimitReader(zeroReader{}, 4)}, nil
}

func (noReaderAtFile) Stat() (fs.FileInfo, error) {
	return fstest.MapFS{"plain.txt": {Data: make([]byte, 4)}}.Stat("plain.txt")
}

func (noReaderAtFile) Close() error { return nil }

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestFS_ReadOnlyDescriptorHandle(t *testing.T) {
	mfs := NewFS()
	if err := mfs.Mount("/ro", fstest.MapFS{"file.txt": {Data: []byte("hello")}}); err != nil {
		t.Fatalf("Mount(/ro) error: %v", err)
	}
	if err := mfs.Mount("/plain", noReaderAtFS{}); err != nil {
		t.Fatalf("Mount(/plain) error: %v", err)
	}

	if _, err := mfs.OpenFD("/ro/file.txt", os.O_RDWR, 0); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("OpenFD(write on read-only fs) error = %v, want ErrUnsupported", err)
	}

	fd, err := mfs.OpenFD("/ro/file.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFD() error: %v", err)
	}
	defer mfs.CloseFD(fd)

	buf := make([]byte, 3)
	if n, err := mfs.ReadAtFD(fd, buf, 0, 3, 2); err != nil || string(buf[:n]) != "llo" {
		t.Fatalf("ReadAtFD() = %q, %v; want llo", buf[:n], err)
	}
	if info, err := mfs.FstatFD(fd); err != nil || info.Size() != 5 {
		t.Fatalf("FstatFD() = %v, %v", info, err)
	}
	requireFSCode(t, "WriteFD read-only", func() error { _, err := mfs.WriteFD(fd, []byte("x")); return err }(), "EACCES")
	requireFSCode(t, "WriteAtFD read-only", func() error { _, err := mfs.WriteAtFD(fd, []byte("x"), 0); return err }(), "EACCES")
	requireFSCode(t, "FsyncFD read-only", mfs.FsyncFD(fd), "ENOTSUP")
	requireFSCode(t, "FchmodFD read-only", mfs.FchmodFD(fd, 0o600), "EACCES")
	requireFSCode(t, "FchownFD read-only", mfs.FchownFD(fd, -1, -1), "EACCES")

	plainFD, err := mfs.OpenFD("/plain/plain.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFD(plain) error: %v", err)
	}
	defer mfs.CloseFD(plainFD)
	requireFSCode(t, "ReadAtFD without io.ReaderAt", func() error { _, err := mfs.ReadAtFD(plainFD, buf, 0, 1, 0); return err }(), "ENOTSUP")
}

func TestErrnoForCodeRoundTrip(t *testing.T) {
	for _, code := range []string{"ENOENT", "ENOTDIR", "EEXIST", "EISDIR", "EBUSY", "EXDEV", "EFBIG"} {
		errno, ok := ErrnoForCode(code)
		if !ok {
			t.Fatalf("ErrnoForCode(%q) not found", code)
		}
		if got := FilesystemErrorCode(&fs.PathError{Op: "op", Path: "/x", Err: errno}); got != code {
			t.Fatalf("round trip %q -> %v -> %q", code, errno, got)
		}
	}
	if _, ok := ErrnoForCode("EUNKNOWN"); ok {
		t.Fatal("ErrnoForCode(EUNKNOWN) should not be found")
	}
	if _, ok := ErrnoForCode(""); ok {
		t.Fatal("ErrnoForCode(\"\") should not be found")
	}
	if errno, _ := ErrnoForCode("ENOTDIR"); errno != ErrnoNotDir {
		t.Fatalf("ErrnoForCode(ENOTDIR) = %v, want ErrnoNotDir", errno)
	}
	if errno, _ := ErrnoForCode("EBUSY"); errno != syscall.EBUSY {
		t.Fatalf("ErrnoForCode(EBUSY) = %v, want EBUSY", errno)
	}
}

// win32ReadDirFS mimics Go on Windows, where reading a regular file as a
// directory fails with the same code as a missing path.
type win32ReadDirFS struct{ fs.FS }

func (w win32ReadDirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if info, err := fs.Stat(w.FS, name); err == nil && !info.IsDir() {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return fs.ReadDir(w.FS, name)
}

func TestFS_ReadDirRegularFileIsNotDirectory(t *testing.T) {
	for _, tc := range []struct {
		name string
		fsys fs.FS
	}{
		{"host", nil},
		{"win32-like", win32ReadDirFS{fstest.MapFS{"file.txt": {Data: []byte("x")}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mfs := NewFS()
			fsys := tc.fsys
			if fsys == nil {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				fsys = os.DirFS(dir)
			}
			if err := mfs.Mount("/", fsys); err != nil {
				t.Fatal(err)
			}
			_, err := mfs.ReadDir("/file.txt")
			requireFSCode(t, "readdir regular file", err, "ENOTDIR")
			_, err = mfs.ReadDir("/missing")
			requireFSCode(t, "readdir missing", err, "ENOENT")
		})
	}
}
