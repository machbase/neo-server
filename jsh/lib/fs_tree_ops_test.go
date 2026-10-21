package lib_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	hostCode := "EINVAL"
	if runtime.GOOS == "windows" {
		// MoveFileEx fails with ERROR_ACCESS_DENIED, which Node.js reports as EPERM.
		hostCode = "EPERM"
	}
	want := "/host rename " + hostCode + "\n/host [a,a/b]\n/lib/zz-rename rename EINVAL\n/lib/zz-rename [a,a/b]\n"
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
	if runtime.GOOS == "windows" {
		// Win32 codes for these host operations differ from Linux; the virtual
		// mount must still report the Linux codes and every host call must fail.
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(lines) != 2 || !strings.HasPrefix(lines[0], "host ") || lines[1] != "lib "+row {
			t.Fatalf("output:\n%s\nwant lib row:\n%s", out, row)
		}
		for _, code := range strings.Split(strings.TrimPrefix(lines[0], "host "), ",") {
			if code == "ok" || code == "" {
				t.Fatalf("host operation unexpectedly succeeded:\n%s", out)
			}
		}
		return
	}
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

func TestFSReadWriteRequireNodeBufferTypes(t *testing.T) {
	out := runFSTreeScript(t, t.TempDir(), `
		const fs = require('fs');
		fs.writeFileSync('/r.txt', 'abcdefgh');
		const fd = fs.openSync('/r.txt', 'r');
		const run = (label, fn) => {
			try {
				console.println(label, JSON.stringify(fn()));
			} catch (e) {
				console.println(label, e.name, e.code);
			}
		};
		run('array', () => fs.readSync(fd, [0, 0, 0, 0], 0, 4, 0));
		run('array-pos', () => fs.readSync(fd, [0, 0], 0, 2));
		run('string', () => fs.readSync(fd, 'abcd', 0, 4, 0));
		run('buffer', () => { const b = Buffer.alloc(3); fs.readSync(fd, b, 0, 3, 1); return b.toString(); });
		run('subarray', () => { const p = new Uint8Array(5); fs.readSync(fd, p.subarray(2), 0, 3, 0); return Array.from(p); });
		run('int16', () => { const v = new Int16Array(2); const n = fs.readSync(fd, v, 0, 4, 0); return [n, Array.from(new Uint8Array(v.buffer))]; });
		run('dataview', () => { const v = new DataView(new ArrayBuffer(3)); fs.readSync(fd, v, 1, 2, 0); return [v.getUint8(0), v.getUint8(1), v.getUint8(2)]; });
		run('int16-range', () => fs.readSync(fd, new Int16Array(2), 0, 5, 0));
		run('arraybuffer', () => { const ab = new ArrayBuffer(4); const n = fs.readSync(fd, ab, 1, 3, 2); return [n, Array.from(new Uint8Array(ab))]; });
		run('arraybuffer-range', () => fs.readSync(fd, new ArrayBuffer(2), 0, 3, 0));
		fs.closeSync(fd);

		const wfd = fs.openSync('/w.txt', 'w+');
		run('w-array', () => fs.writeSync(wfd, [1, 75, 76], 1, 2, 9));
		run('w-array-opts', () => fs.writeSync(wfd, [77], { position: 11 }));
		run('w-number', () => fs.writeSync(wfd, 65));
		run('w-object', () => fs.writeSync(wfd, { toString: () => 'x' }));
		run('w-arraybuffer', () => fs.writeSync(wfd, Uint8Array.from([1, 72, 73]).buffer, 1, 2, 6));
		run('w-arraybuffer-opts', () => fs.writeSync(wfd, Uint8Array.from([74]).buffer, { position: 8 }));
		run('w-null', () => fs.writeSync(wfd, null));
		run('w-int16', () => fs.writeSync(wfd, Int16Array.from([0x4241]), 0, 2, 0));
		run('w-dataview', () => fs.writeSync(wfd, new DataView(Uint8Array.from([67, 68, 69]).buffer, 1), 0, 2, 2));
		run('w-subarray', () => fs.writeSync(wfd, Uint8Array.from([1, 70, 71]).subarray(1), 0, 2, 4));
		fs.closeSync(wfd);
		run('w-content', () => fs.readFileSync('/w.txt', 'utf8'));
	`)
	want := "array TypeError ERR_INVALID_ARG_TYPE\n" +
		"array-pos TypeError ERR_INVALID_ARG_TYPE\n" +
		"string TypeError ERR_INVALID_ARG_TYPE\n" +
		"buffer \"bcd\"\n" +
		"subarray [0,0,97,98,99]\n" +
		"int16 [4,[97,98,99,100]]\n" +
		"dataview [0,97,98]\n" +
		"int16-range RangeError ERR_OUT_OF_RANGE\n" +
		"arraybuffer [3,[0,99,100,101]]\n" +
		"arraybuffer-range RangeError ERR_OUT_OF_RANGE\n" +
		"w-array 2\n" +
		"w-array-opts 1\n" +
		"w-number TypeError ERR_INVALID_ARG_TYPE\n" +
		"w-object TypeError ERR_INVALID_ARG_TYPE\n" +
		"w-arraybuffer 2\n" +
		"w-arraybuffer-opts 1\n" +
		"w-null TypeError ERR_INVALID_ARG_TYPE\n" +
		"w-int16 2\n" +
		"w-dataview 2\n" +
		"w-subarray 2\n" +
		"w-content \"ABDEFGHIJKLM\"\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}

func TestFSWriteFileRequiresNodeDataTypes(t *testing.T) {
	out := runFSTreeScript(t, t.TempDir(), `
		const fs = require('fs');
		const run = (label, fn) => {
			try {
				console.println(label, JSON.stringify(fn()));
			} catch (e) {
				console.println(label, e.name, e.code);
			}
		};
		const hex = (p) => Array.from(fs.readFileSync(p, 'buffer')).map((b) => b.toString(16).padStart(2, '0')).join('');
		for (const op of ['writeFileSync', 'appendFileSync']) {
			run(op + '-number', () => fs[op]('/t.bin', 65));
			run(op + '-object', () => fs[op]('/t.bin', { toString: () => 'x' }));
			run(op + '-null', () => fs[op]('/t.bin', null));
			run(op + '-undefined', () => fs[op]('/t.bin', undefined));
		}
		run('untouched', () => fs.existsSync('/t.bin'));

		// JSH extension: ArrayBuffer is accepted because native modules return it.
		run('ab-write', () => { fs.writeFileSync('/ab.bin', Uint8Array.from([65, 66]).buffer); return fs.readFileSync('/ab.bin', 'utf8'); });
		run('ab-append', () => { fs.appendFileSync('/ab.bin', Uint8Array.from([67]).buffer, 'buffer'); return fs.readFileSync('/ab.bin', 'utf8'); });
		run('ab-empty', () => { fs.writeFileSync('/ab0.bin', new ArrayBuffer(0)); return fs.readFileSync('/ab0.bin', 'buffer').length; });
		// JSH extension: plain byte arrays, e.g. http readBodyBuffer().
		run('arr-write', () => { fs.writeFileSync('/arr.bin', [65, 66], 'buffer'); return fs.readFileSync('/arr.bin', 'utf8'); });
		run('arr-append', () => { fs.appendFileSync('/arr.bin', [67, 324]); return fs.readFileSync('/arr.bin', 'utf8'); });
		run('ab-native', () => {
			const zlib = require('zlib');
			fs.writeFileSync('/z.gz', zlib.gzipSync('hello'));
			return Buffer.from(zlib.gunzipSync(fs.readFileSync('/z.gz', 'buffer'))).toString();
		});

		fs.writeFileSync('/v.bin', Int16Array.from([0x4241]));
		fs.appendFileSync('/v.bin', new DataView(Uint8Array.from([1, 67, 68]).buffer, 1));
		fs.appendFileSync('/v.bin', Buffer.from('EF'));
		fs.appendFileSync('/v.bin', Uint8Array.from([0, 71, 72]).subarray(1));
		run('views', () => fs.readFileSync('/v.bin', 'utf8'));

		run('enc-hex', () => { fs.writeFileSync('/e.bin', '4a4b', 'hex'); return hex('/e.bin'); });
		run('enc-base64', () => { fs.writeFileSync('/e.bin', 'SGk=', { encoding: 'base64' }); return hex('/e.bin'); });
		run('enc-base64url', () => { fs.writeFileSync('/e.bin', '-_8', 'base64url'); return hex('/e.bin'); });
		run('enc-latin1', () => { fs.writeFileSync('/e.bin', '\u00e9A', 'latin1'); return hex('/e.bin'); });
		run('enc-ucs2', () => { fs.writeFileSync('/e.bin', 'Ab', 'utf16le'); return hex('/e.bin'); });
		run('enc-upper', () => { fs.writeFileSync('/e.bin', '4C', 'HEX'); return hex('/e.bin'); });
		run('enc-buffer', () => { fs.writeFileSync('/e.bin', '\u00e9', 'buffer'); return hex('/e.bin'); });
		run('enc-null', () => { fs.writeFileSync('/e.bin', 'z', { encoding: null }); return hex('/e.bin'); });
		run('enc-append-hex', () => { fs.appendFileSync('/e.bin', '7a', 'hex'); return hex('/e.bin'); });
		run('enc-unknown', () => fs.writeFileSync('/e.bin', 'x', 'bogus'));

		const raw = fs.readFileSync('/v.bin', 'buffer');
		run('read-buffer', () => [Buffer.isBuffer(raw), ArrayBuffer.isView(raw), Array.isArray(raw), raw.length]);
		run('read-null', () => Buffer.isBuffer(fs.readFileSync('/v.bin', { encoding: null })));
		run('roundtrip', () => { fs.writeFileSync('/copy.bin', fs.readFileSync('/v.bin', 'buffer'), 'buffer'); return fs.readFileSync('/copy.bin', 'utf8'); });
	`)
	want := "writeFileSync-number TypeError ERR_INVALID_ARG_TYPE\n" +
		"writeFileSync-object TypeError ERR_INVALID_ARG_TYPE\n" +
		"writeFileSync-null TypeError ERR_INVALID_ARG_TYPE\n" +
		"writeFileSync-undefined TypeError ERR_INVALID_ARG_TYPE\n" +
		"appendFileSync-number TypeError ERR_INVALID_ARG_TYPE\n" +
		"appendFileSync-object TypeError ERR_INVALID_ARG_TYPE\n" +
		"appendFileSync-null TypeError ERR_INVALID_ARG_TYPE\n" +
		"appendFileSync-undefined TypeError ERR_INVALID_ARG_TYPE\n" +
		"untouched false\n" +
		"ab-write \"AB\"\n" +
		"ab-append \"ABC\"\n" +
		"ab-empty 0\n" +
		"arr-write \"AB\"\n" +
		"arr-append \"ABCD\"\n" +
		"ab-native \"hello\"\n" +
		"views \"ABCDEFGH\"\n" +
		"enc-hex \"4a4b\"\n" +
		"enc-base64 \"4869\"\n" +
		"enc-base64url \"fbff\"\n" +
		"enc-latin1 \"e941\"\n" +
		"enc-ucs2 \"41006200\"\n" +
		"enc-upper \"4c\"\n" +
		"enc-buffer \"c3a9\"\n" +
		"enc-null \"7a\"\n" +
		"enc-append-hex \"7a7a\"\n" +
		"enc-unknown TypeError ERR_UNKNOWN_ENCODING\n" +
		"read-buffer [true,true,false,8]\n" +
		"read-null true\n" +
		"roundtrip \"ABCDEFGH\"\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}

func TestFSWriteSyncNodeSignatures(t *testing.T) {
	out := runFSTreeScript(t, t.TempDir(), `
		const fs = require('fs');
		const run = (label, fn) => {
			try {
				console.println(label, JSON.stringify(fn()));
			} catch (e) {
				console.println(label, e.name, e.code);
			}
		};
		const content = (p) => fs.readFileSync(p, 'utf8');
		fs.writeFileSync('/s.txt', 'abcdefgh');
		const fd = fs.openSync('/s.txt', 'r+');
		run('str-pos', () => [fs.writeSync(fd, 'XY', 2), content('/s.txt')]);
		run('str-cur', () => [fs.writeSync(fd, 'Z'), content('/s.txt')]);
		run('str-null', () => [fs.writeSync(fd, 'W', null), content('/s.txt')]);
		run('str-nonnum', () => [fs.writeSync(fd, 'V', '7'), content('/s.txt')]);
		run('str-enc', () => [fs.writeSync(fd, '3132', 6, 'hex'), content('/s.txt')]);
		run('str-enc-cur', () => [fs.writeSync(fd, 'UQ==', null, 'base64'), content('/s.txt')]);
		run('str-enc-unknown', () => [fs.writeSync(fd, 'k', 0, 'bogus'), content('/s.txt')]);
		run('str-hex-odd', () => fs.writeSync(fd, 'abc', 0, 'hex'));
		run('str-neg', () => fs.writeSync(fd, 'x', -1));

		run('buf-opts', () => [fs.writeSync(fd, Buffer.from('0123'), { offset: 1, length: 2, position: 4 }), content('/s.txt')]);
		run('buf-opts-default', () => [fs.writeSync(fd, Buffer.from('mn'), { position: 0 }), content('/s.txt')]);
		run('buf-opts-null', () => [fs.writeSync(fd, Buffer.from('!'), null), content('/s.txt')]);
		run('buf-opts-range', () => fs.writeSync(fd, Buffer.from('ab'), { offset: 1, length: 2 }));
		run('buf-args', () => [fs.writeSync(fd, Buffer.from('xyz'), 1, 1, 7), content('/s.txt')]);
		run('buf-nonnum-offset', () => [fs.writeSync(fd, Buffer.from('pq'), 'a', undefined, 0), content('/s.txt')]);
		run('buf-range', () => fs.writeSync(fd, Buffer.from('ab'), 3));
		fs.closeSync(fd);

		const afd = fs.openSync('/s.txt', 'a');
		run('append-str-pos', () => [fs.writeSync(afd, '+', 0), content('/s.txt')]);
		fs.closeSync(afd);
	`)
	want := "str-pos [2,\"abXYefgh\"]\n" +
		"str-cur [1,\"ZbXYefgh\"]\n" +
		"str-null [1,\"ZWXYefgh\"]\n" +
		"str-nonnum [1,\"ZWVYefgh\"]\n" +
		"str-enc [2,\"ZWVYef12\"]\n" +
		"str-enc-cur [1,\"ZWVQef12\"]\n" +
		"str-enc-unknown [1,\"kWVQef12\"]\n" +
		"str-hex-odd TypeError ERR_INVALID_ARG_VALUE\n" +
		"str-neg RangeError ERR_OUT_OF_RANGE\n" +
		"buf-opts [2,\"kWVQ1212\"]\n" +
		"buf-opts-default [2,\"mnVQ1212\"]\n" +
		"buf-opts-null [1,\"mnVQ!212\"]\n" +
		"buf-opts-range RangeError ERR_OUT_OF_RANGE\n" +
		"buf-args [1,\"mnVQ!21y\"]\n" +
		"buf-nonnum-offset [2,\"pqVQ!21y\"]\n" +
		"buf-range RangeError ERR_OUT_OF_RANGE\n" +
		"append-str-pos [1,\"pqVQ!21y+\"]\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}

// Legacy JSH string form writeSync(fd, string, offset, length[, position]) is kept
// as an extension when the fourth argument is a number; offset/length are UTF-8 byte units.
func TestFSWriteSyncLegacyStringForm(t *testing.T) {
	out := runFSTreeScript(t, t.TempDir(), `
		const fs = require('fs');
		const run = (label, fn) => {
			try {
				console.println(label, JSON.stringify(fn()));
			} catch (e) {
				console.println(label, e.name, e.code);
			}
		};
		const content = (p) => fs.readFileSync(p, 'utf8');
		fs.writeFileSync('/l.txt', 'abcdefgh');
		const fd = fs.openSync('/l.txt', 'r+');
		run('legacy-full', () => [fs.writeSync(fd, 'XY', 0, 2), content('/l.txt')]);
		run('legacy-cur', () => [fs.writeSync(fd, 'PQR', 1, 2), content('/l.txt')]);
		run('legacy-pos', () => [fs.writeSync(fd, 'mn', 0, 2, 6), content('/l.txt')]);
		run('legacy-null-offset', () => [fs.writeSync(fd, 'k', null, 1), content('/l.txt')]);
		run('legacy-nonnum-offset', () => [fs.writeSync(fd, 'a', 'x', 1), content('/l.txt')]);
		run('legacy-utf8', () => [fs.writeSync(fd, '\u00e9!', 2, 1, 0), content('/l.txt')]);
		run('legacy-zero', () => [fs.writeSync(fd, 'zz', 0, 0), content('/l.txt')]);
		run('legacy-range', () => fs.writeSync(fd, 'ab', 1, 2));
		run('legacy-neg-pos', () => fs.writeSync(fd, 'a', 0, 1, -1));
		run('legacy-cursor', () => [fs.writeSync(fd, 'Z'), content('/l.txt')]);
		fs.closeSync(fd);
	`)
	want := "legacy-full [2,\"XYcdefgh\"]\n" +
		"legacy-cur [2,\"XYQRefgh\"]\n" +
		"legacy-pos [2,\"XYQRefmn\"]\n" +
		"legacy-null-offset [1,\"XYQRkfmn\"]\n" +
		"legacy-nonnum-offset [1,\"XYQRkamn\"]\n" +
		"legacy-utf8 [1,\"!YQRkamn\"]\n" +
		"legacy-zero [0,\"!YQRkamn\"]\n" +
		"legacy-range RangeError ERR_OUT_OF_RANGE\n" +
		"legacy-neg-pos RangeError ERR_OUT_OF_RANGE\n" +
		"legacy-cursor [1,\"!YQRkaZn\"]\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}

// Buffer methods that create a new instance through Symbol.species (slice, map, filter)
// must work on Buffers, including those returned by fs.readFileSync(path, 'buffer').
func TestFSBufferResultSupportsSpeciesMethods(t *testing.T) {
	out := runFSTreeScript(t, t.TempDir(), `
		const fs = require('fs');
		const run = (label, fn) => {
			try {
				console.println(label, JSON.stringify(fn()));
			} catch (e) {
				console.println(label, e.name, e.message);
			}
		};
		fs.writeFileSync('/b.bin', 'abcdef');
		const b = fs.readFileSync('/b.bin', 'buffer');
		run('slice', () => { const s = b.slice(1, 3); return [Buffer.isBuffer(s), s.toString()]; });
		run('slice-neg', () => b.slice(-2).toString());
		run('slice-shares', () => { const c = Buffer.from('xyz'); c.slice(0, 1)[0] = 65; return c.toString(); });
		run('map', () => { const m = b.map((x) => x - 32); return [Buffer.isBuffer(m), m.toString()]; });
		run('filter', () => b.filter((x) => x > 100).toString());
		run('proto-slice', () => Uint8Array.prototype.slice.call(b, 4).toString());
		run('u8-slice-copy', () => { const u = new Uint8Array([1, 2, 3]); const s = u.slice(1); s[0] = 9; return [u[1], Buffer.isBuffer(s)]; });
		run('new-buffer-str', () => new Buffer('hi').toString());
	`)
	want := "slice [true,\"bc\"]\n" +
		"slice-neg \"ef\"\n" +
		"slice-shares \"Ayz\"\n" +
		"map [true,\"ABCDEF\"]\n" +
		"filter \"ef\"\n" +
		"proto-slice \"ef\"\n" +
		"u8-slice-copy [2,false]\n" +
		"new-buffer-str \"hi\"\n"
	if out != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}
