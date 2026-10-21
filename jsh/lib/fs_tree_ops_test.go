package lib_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
)

func runFSTreeScript(t *testing.T, rootDir string, code string) string {
	t.Helper()
	output := &bytes.Buffer{}
	jr, err := engine.New(engine.Config{
		Name: "fs-tree-ops",
		Code: code,
		FSTabs: []engine.FSTab{
			{MountPoint: "/", Source: rootDir},
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
		t.Fatalf("Run() error: %v\noutput:\n%s", err, output.String())
	}
	return output.String()
}

func TestFSRmdirRecursiveDoesNotFollowSymlink(t *testing.T) {
	rootDir := t.TempDir()
	out := runFSTreeScript(t, rootDir, `
		const fs = require('fs');
		fs.mkdirSync('/target/sub', { recursive: true });
		fs.writeFileSync('/target/sub/keep.txt', 'k');
		fs.symlinkSync('/target', '/link');
		try {
			fs.rmdirSync('/link', { recursive: true });
			console.println('rmdir ok');
		} catch (e) {
			console.println('rmdir', e.code, e.path);
		}
		console.println('kept', fs.existsSync('/target/sub/keep.txt'));
		console.println('link', fs.lstatSync('/link').isSymbolicLink());
	`)
	want := "rmdir ENOTDIR /link\nkept true\nlink true\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "target", "sub", "keep.txt")); err != nil {
		t.Fatalf("symlink target content must be preserved: %v", err)
	}
}

func TestFSRenameDirectoryIntoItselfIsInvalid(t *testing.T) {
	rootDir := t.TempDir()
	out := runFSTreeScript(t, rootDir, `
		const fs = require('fs');
		const tree = (dir) => fs.readdirSync(dir, { recursive: true }).filter((n) => n !== '.' && n !== '..').sort().join(',');
		for (const base of ['/host', '/lib/zz-rename']) {
			fs.mkdirSync(base + '/a/b', { recursive: true });
			try {
				fs.renameSync(base + '/a', base + '/a/b/c');
				console.println(base, 'rename ok');
			} catch (e) {
				console.println(base, 'rename', e.code);
			}
			console.println(base, '[' + tree(base) + ']');
		}
		fs.rmSync('/lib/zz-rename', { recursive: true });
	`)
	want := "/host rename EINVAL\n/host [a,a/b]\n/lib/zz-rename rename EINVAL\n/lib/zz-rename [a,a/b]\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}

func TestFSCpRejectsSelfAndSubdirectoryTargets(t *testing.T) {
	rootDir := t.TempDir()
	out := runFSTreeScript(t, rootDir, `
		const fs = require('fs');
		const tree = (dir) => fs.readdirSync(dir, { recursive: true }).filter((n) => n !== '.' && n !== '..').sort().join(',');
		const tryCp = (label, src, dest, opts) => {
			try {
				fs.cpSync(src, dest, opts);
				console.println(label, 'ok');
			} catch (e) {
				console.println(label, e.code);
			}
		};
		fs.mkdirSync('/cpd', { recursive: true });
		fs.writeFileSync('/cpd/a', 'a');
		fs.writeFileSync('/f.txt', 'abc');
		fs.symlinkSync('/cpd', '/cpd-link');
		tryCp('subdir', '/cpd', '/cpd/inner', { recursive: true });
		tryCp('subdir-via-link', '/cpd-link', '/cpd/inner', { recursive: true });
		tryCp('same-dir', '/cpd', '/cpd/', { recursive: true });
		tryCp('same-file', '/f.txt', '/f.txt');
		tryCp('sibling', '/cpd', '/cpd2', { recursive: true });
		console.println('cpd', '[' + tree('/cpd') + ']');
		console.println('cpd2', '[' + tree('/cpd2') + ']');
		console.println('f', fs.readFileSync('/f.txt', 'utf8'));
	`)
	want := "subdir ERR_FS_CP_EINVAL\nsubdir-via-link ERR_FS_CP_EINVAL\nsame-dir ERR_FS_CP_EINVAL\nsame-file ERR_FS_CP_EINVAL\nsibling ok\ncpd [a]\ncpd2 [a]\nf abc\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}

func TestFSVirtualMountErrorsMatchHostMount(t *testing.T) {
	out := runFSTreeScript(t, t.TempDir(), `
		const fs = require('fs');
		const code = (fn) => { try { fn(); return 'ok'; } catch (e) { return e.code; } };
		for (const b of ['/host', '/lib/zz-err']) {
			fs.mkdirSync(b + '/d/sub', { recursive: true });
			fs.writeFileSync(b + '/f.txt', 'x');
			const results = [
				code(() => fs.renameSync(b + '/missing', b + '/missing/x')),
				code(() => fs.renameSync(b + '/f.txt', b + '/f.txt/y')),
				code(() => fs.renameSync(b + '/d', b + '/f.txt/d')),
				code(() => fs.renameSync(b + '/d', b + '/d/sub/x')),
				code(() => fs.renameSync(b + '/d', b + '/d/nope/x')),
				code(() => fs.writeFileSync(b + '/f.txt/x', 'z')),
				code(() => fs.writeFileSync(b + '/d', 'z')),
				code(() => fs.appendFileSync(b + '/f.txt/x', 'z')),
				code(() => fs.appendFileSync(b + '/d', 'z')),
			];
			console.println(b.startsWith('/lib') ? 'lib' : 'host', results.join(','));
		}
		fs.rmSync('/lib/zz-err', { recursive: true });
	`)
	row := "ENOENT,ENOTDIR,ENOTDIR,EINVAL,ENOENT,ENOTDIR,EISDIR,ENOTDIR,EISDIR"
	want := "host " + row + "\nlib " + row + "\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}

func TestFSCpFollowsNestedSymlinksSafely(t *testing.T) {
	out := runFSTreeScript(t, t.TempDir(), `
		const fs = require('fs');
		const tree = (dir) => fs.readdirSync(dir, { recursive: true }).filter((n) => n !== '.' && n !== '..').sort().join(',');
		const tryCp = (label, src, dest) => {
			try {
				fs.cpSync(src, dest, { recursive: true });
				console.println(label, 'ok', '[' + tree(dest) + ']');
			} catch (e) {
				console.println(label, e.code, e.path);
			}
		};
		fs.mkdirSync('/src/sub', { recursive: true });
		fs.writeFileSync('/src/sub/a', 'a');
		fs.mkdirSync('/other', { recursive: true });
		fs.writeFileSync('/other/o', 'o');
		fs.symlinkSync('/other', '/src/dirlink');
		fs.symlinkSync('/src/sub/a', '/src/filelink');
		tryCp('links', '/src', '/dst');
		console.println('dirlink copied as dir', fs.lstatSync('/dst/dirlink').isDirectory());
		console.println('content', fs.readFileSync('/dst/dirlink/o', 'utf8'), fs.readFileSync('/dst/filelink', 'utf8'));

		fs.mkdirSync('/loop/in', { recursive: true });
		fs.symlinkSync('/loop', '/loop/in/up');
		tryCp('cycle', '/loop', '/loopcopy');

		fs.mkdirSync('/s2', { recursive: true });
		fs.mkdirSync('/d2', { recursive: true });
		fs.symlinkSync('/d2', '/s2/todest');
		tryCp('into-dest', '/s2', '/d2/copy');
		console.println('d2', '[' + tree('/d2') + ']');

		fs.mkdirSync('/s3', { recursive: true });
		fs.symlinkSync('/missing', '/s3/broken');
		tryCp('broken', '/s3', '/d3');
	`)
	want := "links ok [dirlink,dirlink/o,filelink,sub,sub/a]\n" +
		"dirlink copied as dir true\n" +
		"content o a\n" +
		"cycle ELOOP /loop/in/up\n" +
		"into-dest ERR_FS_CP_EINVAL /s2/todest\n" +
		"d2 [copy]\n" +
		"broken ENOENT /s3/broken\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}
