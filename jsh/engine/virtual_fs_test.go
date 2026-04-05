package engine

import (
	"io/fs"
	"sort"
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

func TestVirtualFS_WriteAppendMkdirAndRename(t *testing.T) {
	vfs := NewVirtualFS()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	must(vfs.Mkdir("workspace/cache"))
	if _, err := fs.Stat(vfs, "workspace/cache"); err != nil {
		t.Fatalf("mkdir should create explicit directory: %v", err)
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
