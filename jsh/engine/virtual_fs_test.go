package engine

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"syscall"
	"testing"
	"testing/fstest"
	"time"
)

func TestVirtualFS_CreateAndReadFile(t *testing.T) {
	vfs := NewVirtualFS()

	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	modified := created.Add(2 * time.Hour)
	prop := VirtualFileProperty{
		CreateTime: created,
		ModTime:    modified,
		Mode:       0600,
	}

	if err := vfs.AddFile("docs/readme.txt", "hello", prop); err != nil {
		t.Fatalf("CreateFile(string) failed: %v", err)
	}
	if err := vfs.AddFile("docs/data.bin", []byte{1, 2, 3}, VirtualFileProperty{}); err != nil {
		t.Fatalf("CreateFile([]byte) failed: %v", err)
	}

	data, err := fs.ReadFile(vfs, "docs/readme.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %q", string(data))
	}

	info, err := fs.Stat(vfs, "docs/readme.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("unexpected mode: %v", info.Mode())
	}
	if !info.ModTime().Equal(modified) {
		t.Fatalf("unexpected mod time: %v", info.ModTime())
	}

	sys, ok := info.Sys().(VirtualFileProperty)
	if !ok {
		t.Fatalf("unexpected sys type: %T", info.Sys())
	}
	if !sys.CreateTime.Equal(created) {
		t.Fatalf("unexpected create time: %v", sys.CreateTime)
	}
}

func TestVirtualFS_ReadDir(t *testing.T) {
	vfs := NewVirtualFS()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	must(vfs.AddFile("app/main.js", "x", VirtualFileProperty{}))
	must(vfs.AddFile("app/lib/util.js", "y", VirtualFileProperty{}))
	must(vfs.AddFile("app/readme.md", "z", VirtualFileProperty{}))

	entries, err := fs.ReadDir(vfs, "app")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	names := make([]string, 0, len(entries))
	dirs := map[string]bool{}
	for _, ent := range entries {
		names = append(names, ent.Name())
		dirs[ent.Name()] = ent.IsDir()
	}
	sort.Strings(names)

	expected := []string{"lib", "main.js", "readme.md"}
	if len(names) != len(expected) {
		t.Fatalf("unexpected entries count: %d", len(names))
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("unexpected entry[%d]: %q", i, names[i])
		}
	}
	if !dirs["lib"] {
		t.Fatalf("lib should be a directory")
	}
}

func TestVirtualFS_RemoveFileAndTree(t *testing.T) {
	vfs := NewVirtualFS()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	must(vfs.AddFile("root/a.txt", "a", VirtualFileProperty{}))
	must(vfs.AddFile("root/sub/b.txt", "b", VirtualFileProperty{}))

	must(vfs.Remove("root/a.txt"))
	if _, err := fs.Stat(vfs, "root/a.txt"); err == nil {
		t.Fatalf("removed file should not exist")
	}

	must(vfs.Remove("root/sub"))
	if _, err := fs.Stat(vfs, "root/sub/b.txt"); err == nil {
		t.Fatalf("removed subtree file should not exist")
	}
}

func TestVirtualFS_UnlinkDoesNotRemoveDirectories(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.AddFile("root/sub/keep.txt", "keep", VirtualFileProperty{}); err != nil {
		t.Fatal(err)
	}

	if err := vfs.Unlink("root/sub"); !errors.Is(err, syscall.EISDIR) {
		t.Fatalf("Unlink(directory) error = %v, want %v", err, syscall.EISDIR)
	}
	data, err := fs.ReadFile(vfs, "root/sub/keep.txt")
	if err != nil {
		t.Fatalf("directory contents should remain after failed unlink: %v", err)
	}
	if string(data) != "keep" {
		t.Fatalf("unexpected file contents: %q", data)
	}

	if err := vfs.Unlink("root/sub/keep.txt"); err != nil {
		t.Fatalf("Unlink(file) failed: %v", err)
	}
	if _, err := fs.Stat(vfs, "root/sub/keep.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unlinked file Stat error = %v, want %v", err, fs.ErrNotExist)
	}
}

func TestVirtualFS_RmdirRequiresEmptyDirectory(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.Mkdir("root"); err != nil {
		t.Fatal(err)
	}
	if err := vfs.Mkdir("root/empty"); err != nil {
		t.Fatal(err)
	}
	if err := vfs.Rmdir("root/empty"); err != nil {
		t.Fatalf("Rmdir(empty directory) failed: %v", err)
	}
	if _, err := vfs.Stat("root/empty"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("removed directory Stat error=%v, want %v", err, fs.ErrNotExist)
	}

	if err := vfs.AddFile("root/nonempty/keep.txt", "keep", VirtualFileProperty{}); err != nil {
		t.Fatal(err)
	}
	if err := vfs.Rmdir("root/nonempty"); !errors.Is(err, syscall.ENOTEMPTY) {
		t.Fatalf("Rmdir(non-empty directory) error=%v, want %v", err, syscall.ENOTEMPTY)
	}
	if data, err := fs.ReadFile(vfs, "root/nonempty/keep.txt"); err != nil || string(data) != "keep" {
		t.Fatalf("non-empty directory contents should remain: data=%q error=%v", data, err)
	}
}

func TestVirtualFS_MkdirRequiresExistingParent(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.MkdirMode("missing/child", 0); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("MkdirMode(missing parent) error=%v, want %v", err, fs.ErrNotExist)
	}
	if _, err := vfs.Stat("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing parent Stat error=%v, want %v", err, fs.ErrNotExist)
	}

	if err := vfs.Mkdir("parent"); err != nil {
		t.Fatal(err)
	}
	if err := vfs.MkdirMode("parent/private", 0); err != nil {
		t.Fatalf("MkdirMode(mode zero) failed: %v", err)
	}
	info, err := vfs.Stat("parent/private")
	if err != nil {
		t.Fatalf("Stat(private directory) failed: %v", err)
	}
	if info.Mode().Perm() != 0 {
		t.Fatalf("MkdirMode(mode zero) mode=%o, want 0", info.Mode().Perm())
	}
	if err := vfs.Mkdir("parent/private"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("Mkdir(existing directory) error=%v, want %v", err, fs.ErrExist)
	}
}

func TestVirtualFS_Truncate(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.AddFile("file.txt", "abcdef", VirtualFileProperty{}); err != nil {
		t.Fatal(err)
	}
	if err := vfs.Truncate("file.txt", 3); err != nil {
		t.Fatalf("Truncate(shrink) failed: %v", err)
	}
	data, err := vfs.ReadFile("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "abc" {
		t.Fatalf("truncated content=%q, want %q", data, "abc")
	}

	if err := vfs.Truncate("file.txt", 5); err != nil {
		t.Fatalf("Truncate(expand) failed: %v", err)
	}
	data, err = vfs.ReadFile("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 5 || data[3] != 0 || data[4] != 0 {
		t.Fatalf("expanded data=%v, want zero-filled length 5", data)
	}
	if err := vfs.Truncate("file.txt", -1); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("Truncate(negative) error=%v, want %v", err, fs.ErrInvalid)
	}
}

func TestVirtualFS_WriteAppendMkdirAndRename(t *testing.T) {
	vfs := NewVirtualFS()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	must(vfs.Mkdir("workspace"))
	must(vfs.MkdirMode("workspace/cache", 0700))
	if _, err := fs.Stat(vfs, "workspace/cache"); err != nil {
		t.Fatalf("mkdir should create explicit directory: %v", err)
	}
	info, err := fs.Stat(vfs, "workspace/cache")
	if err != nil {
		t.Fatalf("Stat(cache) failed: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("MkdirMode() mode=%o, want 0700", info.Mode().Perm())
	}

	must(vfs.WriteFile("workspace/cache/data.txt", []byte("neo")))
	must(vfs.AppendFile("workspace/cache/data.txt", []byte("-server")))

	data, err := fs.ReadFile(vfs, "workspace/cache/data.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "neo-server" {
		t.Fatalf("unexpected content: %q", string(data))
	}

	must(vfs.Mkdir("workspace/archive"))
	must(vfs.Rename("workspace/cache", "workspace/archive/cache"))

	if _, err := fs.Stat(vfs, "workspace/cache/data.txt"); err == nil {
		t.Fatalf("old path should not exist after rename")
	}
	renamed, err := fs.ReadFile(vfs, "workspace/archive/cache/data.txt")
	if err != nil {
		t.Fatalf("renamed file should exist: %v", err)
	}
	if string(renamed) != "neo-server" {
		t.Fatalf("unexpected renamed content: %q", string(renamed))
	}

	entries, err := fs.ReadDir(vfs, "workspace/archive")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "cache" || !entries[0].IsDir() {
		t.Fatalf("unexpected archive entries: %#v", entries)
	}
}

func TestVirtualFS_EmptyDirPersists(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.Mkdir("empty"); err != nil {
		t.Fatalf("Mkdir parent failed: %v", err)
	}
	if err := vfs.Mkdir("empty/child"); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	entries, err := fs.ReadDir(vfs, "empty")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "child" || !entries[0].IsDir() {
		t.Fatalf("unexpected entries for explicit empty directory: %#v", entries)
	}
}

func TestVirtualFS_InvalidCreate(t *testing.T) {
	vfs := NewVirtualFS()

	if err := vfs.AddFile(".", "x", VirtualFileProperty{}); err == nil {
		t.Fatalf("creating root as file should fail")
	}
	if err := vfs.AddFile("a.txt", 123, VirtualFileProperty{}); err == nil {
		t.Fatalf("unsupported content type should fail")
	}
}

func TestVirtualFS_ConflictingPaths(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.AddFile("a/b.txt", "x", VirtualFileProperty{}); err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	if err := vfs.AddFile("a", "y", VirtualFileProperty{}); err == nil {
		t.Fatalf("file cannot replace existing virtual directory")
	}
}

func TestVirtualFS_MountedOnNewFS(t *testing.T) {
	mfs := NewFS()

	root := fstest.MapFS{
		"etc/hosts": &fstest.MapFile{Data: []byte("127.0.0.1 localhost\n")},
	}
	if err := mfs.Mount("/", root); err != nil {
		t.Fatalf("mount root failed: %v", err)
	}

	usr := NewVirtualFS()
	if err := usr.AddFile("bin/neo", "#!/bin/sh\necho neo\n", VirtualFileProperty{Mode: 0755}); err != nil {
		t.Fatalf("virtual create failed: %v", err)
	}
	if err := usr.AddFile("share/doc/readme.txt", "neo docs", VirtualFileProperty{}); err != nil {
		t.Fatalf("virtual create failed: %v", err)
	}

	if err := mfs.Mount("/usr", usr); err != nil {
		t.Fatalf("mount /usr failed: %v", err)
	}

	b, err := fs.ReadFile(mfs, "/usr/bin/neo")
	if err != nil {
		t.Fatalf("read mounted virtual file failed: %v", err)
	}
	if string(b) != "#!/bin/sh\necho neo\n" {
		t.Fatalf("unexpected content: %q", string(b))
	}

	if _, err := fs.Stat(mfs, "/usr/share/doc"); err != nil {
		t.Fatalf("stat mounted virtual dir failed: %v", err)
	}

	usrEntries, err := fs.ReadDir(mfs, "/usr")
	if err != nil {
		t.Fatalf("readdir /usr failed: %v", err)
	}
	usrNames := map[string]bool{}
	for _, ent := range usrEntries {
		usrNames[ent.Name()] = true
	}
	if !usrNames["bin"] || !usrNames["share"] {
		t.Fatalf("/usr should include bin and share, got: %#v", usrNames)
	}

	rootEntries, err := fs.ReadDir(mfs, "/")
	if err != nil {
		t.Fatalf("readdir / failed: %v", err)
	}
	rootNames := map[string]bool{}
	for _, ent := range rootEntries {
		rootNames[ent.Name()] = true
	}
	if !rootNames["etc"] || !rootNames["usr"] {
		t.Fatalf("/ should include etc and usr, got: %#v", rootNames)
	}

	rootBytes, err := fs.ReadFile(mfs, "/etc/hosts")
	if err != nil {
		t.Fatalf("read root file failed: %v", err)
	}
	if len(rootBytes) == 0 {
		t.Fatalf("root file is unexpectedly empty")
	}
}

func TestVirtualFS_LazyLoaderFile(t *testing.T) {
	vfs := NewVirtualFS()

	counter := 0
	loader := func() []byte {
		counter++
		return []byte(fmt.Sprintf("call-%d", counter))
	}

	if err := vfs.AddFile("dynamic.txt", loader, VirtualFileProperty{}); err != nil {
		t.Fatalf("AddFile(loader) failed: %v", err)
	}

	// Each ReadFile call should invoke the loader freshly.
	data1, err := fs.ReadFile(vfs, "dynamic.txt")
	if err != nil {
		t.Fatalf("ReadFile #1 failed: %v", err)
	}
	if string(data1) != "call-1" {
		t.Fatalf("unexpected #1: %q", string(data1))
	}

	data2, err := fs.ReadFile(vfs, "dynamic.txt")
	if err != nil {
		t.Fatalf("ReadFile #2 failed: %v", err)
	}
	if string(data2) != "call-2" {
		t.Fatalf("unexpected #2: %q", string(data2))
	}

	// Open + Read should also invoke the loader.
	f, err := vfs.Open("dynamic.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()
	buf, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(buf) != "call-3" {
		t.Fatalf("unexpected open-read: %q", string(buf))
	}

	// Stat size reflects each invocation's returned length.
	info, err := fs.Stat(vfs, "dynamic.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	// "call-4" is 6 bytes
	if info.Size() != 6 {
		t.Fatalf("unexpected stat size: %d", info.Size())
	}

	// WriteFile on a loader file must be rejected.
	if err := vfs.WriteFile("dynamic.txt", []byte("x")); err == nil {
		t.Fatal("WriteFile on loader file should fail")
	}

	// AppendFile on a loader file must be rejected.
	if err := vfs.AppendFile("dynamic.txt", []byte("x")); err == nil {
		t.Fatal("AppendFile on loader file should fail")
	}

	// AddFile duplicate must still fail.
	if err := vfs.AddFile("dynamic.txt", loader, VirtualFileProperty{}); err == nil {
		t.Fatal("duplicate AddFile should fail")
	}
}

func TestVirtualFS_RemovalPreservesParentDirectories(t *testing.T) {
	vfs := NewVirtualFS()
	for _, dir := range []string{"rmdir", "rmdir/child", "unlink", "remove", "remove/child", "rename", "dest"} {
		if err := vfs.Mkdir(dir); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for _, file := range []string{"unlink/file", "rename/file"} {
		if err := vfs.WriteFile(file, []byte("x")); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}

	if err := vfs.Rmdir("rmdir/child"); err != nil {
		t.Fatalf("rmdir: %v", err)
	}
	if err := vfs.Unlink("unlink/file"); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if err := vfs.Remove("remove/child"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := vfs.Rename("rename/file", "dest/file"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	for _, dir := range []string{"rmdir", "unlink", "remove", "rename"} {
		info, err := vfs.Stat(dir)
		if err != nil {
			t.Fatalf("parent %s should be preserved: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("parent %s should remain a directory", dir)
		}
	}
}

func TestVirtualFS_PosixErrorCodes(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.WriteFile("file", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := vfs.Mkdir("dir"); err != nil {
		t.Fatal(err)
	}
	if err := vfs.WriteFile("dir/child", []byte("x")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		err  error
		want error
	}{
		{"rmdir file", vfs.Rmdir("file"), syscall.ENOTDIR},
		{"rmdir non-empty", vfs.Rmdir("dir"), syscall.ENOTEMPTY},
		{"unlink directory", vfs.Unlink("dir"), syscall.EISDIR},
		{"mkdir under file", vfs.Mkdir("file/child"), syscall.ENOTDIR},
	}
	for _, tc := range tests {
		if !errors.Is(tc.err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, tc.err, tc.want)
		}
	}
}

func TestVirtualFS_TruncateRejectsOversizedFile(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.WriteFile("file", []byte("x")); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int64{MaxVirtualFileSize + 1, 1 << 62} {
		err := vfs.Truncate("file", size)
		if !errors.Is(err, syscall.EFBIG) {
			t.Fatalf("truncate %d: got %v, want EFBIG", size, err)
		}
	}
	data, err := vfs.ReadFile("file")
	if err != nil || string(data) != "x" {
		t.Fatalf("file must be unchanged: %q, %v", data, err)
	}
}

func TestVirtualFS_RenameDirectoryIntoItselfIsInvalid(t *testing.T) {
	vfs := NewVirtualFS()
	if err := vfs.WriteFile("a/b/f.txt", []byte("x")); err != nil {
		t.Fatal(err)
	}
	err := vfs.Rename("a", "a/b/c")
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("Rename into own subtree error = %v, want EINVAL", err)
	}
	if _, err := vfs.Stat("a/b/f.txt"); err != nil {
		t.Fatalf("source tree must remain: %v", err)
	}
	if _, err := vfs.Stat("a/b/c"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("destination must not be created, stat err = %v", err)
	}
	if err := vfs.Rename("a/b", "a/bc"); err != nil {
		t.Fatalf("Rename to sibling with shared prefix error = %v", err)
	}
}

func TestVirtualFS_ErrorPrecedenceFollowsLinux(t *testing.T) {
	newFS := func(t *testing.T) *VirtualFS {
		t.Helper()
		vfs := NewVirtualFS()
		if err := vfs.WriteFile("d/sub/f.txt", []byte("x")); err != nil {
			t.Fatal(err)
		}
		if err := vfs.WriteFile("file.txt", []byte("y")); err != nil {
			t.Fatal(err)
		}
		return vfs
	}
	tests := []struct {
		name string
		op   func(vfs *VirtualFS) error
		want string
	}{
		{"rename missing source into its own subtree", func(v *VirtualFS) error { return v.Rename("missing", "missing/x") }, "ENOENT"},
		{"rename missing source to missing parent", func(v *VirtualFS) error { return v.Rename("missing", "nope/x") }, "ENOENT"},
		{"rename file under itself", func(v *VirtualFS) error { return v.Rename("file.txt", "file.txt/y") }, "ENOTDIR"},
		{"rename into a file parent", func(v *VirtualFS) error { return v.Rename("d", "file.txt/d") }, "ENOTDIR"},
		{"rename directory into its own subtree", func(v *VirtualFS) error { return v.Rename("d", "d/sub/x") }, "EINVAL"},
		{"rename directory into missing subtree parent", func(v *VirtualFS) error { return v.Rename("d", "d/nope/x") }, "ENOENT"},
		{"writefile under a file", func(v *VirtualFS) error { return v.WriteFile("file.txt/x", []byte("z")) }, "ENOTDIR"},
		{"writefile on a directory", func(v *VirtualFS) error { return v.WriteFile("d", []byte("z")) }, "EISDIR"},
		{"appendfile under a file", func(v *VirtualFS) error { return v.AppendFile("file.txt/x", []byte("z")) }, "ENOTDIR"},
		{"appendfile on a directory", func(v *VirtualFS) error { return v.AppendFile("d", []byte("z")) }, "EISDIR"},
		{"addfile under a file", func(v *VirtualFS) error { return v.AddFile("file.txt/x", "z", VirtualFileProperty{}) }, "ENOTDIR"},
		{"addfile on a directory", func(v *VirtualFS) error { return v.AddFile("d", "z", VirtualFileProperty{}) }, "EEXIST"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vfs := newFS(t)
			err := tc.op(vfs)
			if got := FilesystemErrorCode(err); got != tc.want {
				t.Fatalf("error = %v (%s), want %s", err, got, tc.want)
			}
			if _, err := vfs.Stat("d/sub/f.txt"); err != nil {
				t.Fatalf("existing tree must remain: %v", err)
			}
			if data, err := vfs.ReadFile("file.txt"); err != nil || string(data) != "y" {
				t.Fatalf("existing file must remain: %q, %v", data, err)
			}
		})
	}
}
