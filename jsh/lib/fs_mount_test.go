package lib_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
)

func TestFSRmRecursiveKeepsNestedMountPoints(t *testing.T) {
	rootDir := t.TempDir()
	innerDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "data", "inner"), 0o755); err != nil {
		t.Fatal(err)
	}

	output := &bytes.Buffer{}
	jr, err := engine.New(engine.Config{
		Name: "rm-nested-mount",
		Code: `
			const fs = require('fs');
			fs.mkdirSync('/data/a/b', { recursive: true });
			fs.writeFileSync('/data/a/b/f.txt', 'x');
			fs.writeFileSync('/data/z.txt', 'z');
			fs.writeFileSync('/data/inner/f.txt', 'i');
			try {
				fs.rmSync('/data', { recursive: true, force: true });
				console.println('rm ok');
			} catch (e) {
				console.println('rm', e.code, e.path);
			}
			const names = (dir) => fs.readdirSync(dir).filter((n) => n !== '.' && n !== '..').sort().join(',');
			console.println('data', '[' + names('/data') + ']');
			console.println('inner', '[' + names('/data/inner') + ']');
		`,
		FSTabs: []engine.FSTab{
			{MountPoint: "/", Source: rootDir},
			{MountPoint: "/data/inner", Source: innerDir},
			{MountPoint: "/lib", FS: lib.LibFS()},
		},
		Env:    map[string]any{"PWD": "/", "HOME": "/", "LIBRARY_PATH": "/lib"},
		Reader: &bytes.Buffer{},
		Writer: output,
	})
	if err != nil {
		t.Fatal(err)
	}
	lib.Enable(jr)
	if err := jr.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	want := "rm EBUSY /data/inner\ndata [inner]\ninner []\n"
	if got := output.String(); got != want {
		t.Fatalf("output:\n%s\nwant:\n%s", got, want)
	}
	if entries, err := os.ReadDir(innerDir); err != nil || len(entries) != 0 {
		t.Fatalf("inner mount content = %v, %v; want empty", entries, err)
	}
	if _, err := os.Stat(innerDir); err != nil {
		t.Fatalf("inner mount directory must remain: %v", err)
	}
	if strings.Contains(output.String(), "rm ok") {
		t.Fatal("rmSync must report EBUSY for the nested mount point")
	}
}
