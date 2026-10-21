package engine

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestFilesystemExportsPositionedIOAndErrorCode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/data.txt", []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf(`
const fs = require('@jsh/fs');
function expect(label, got, want) {
	if (got !== want) throw new Error(label + ': got ' + got + ', want ' + want);
}
function expectThrow(label, fn, pattern) {
	try { fn(); } catch (e) {
		if (!pattern.test(String(e))) throw new Error(label + ': unexpected error ' + e);
		return e;
	}
	throw new Error(label + ': expected an error');
}
const fd = fs.open('/work/data.txt', %d, 0o644);
const buf = new Uint8Array(4);
expect('readAt bytes', fs.readAt(fd, buf, 0, 4, 3), 4);
expect('readAt data', String.fromCharCode.apply(null, buf), '3456');
expect('read after readAt keeps cursor', fs.read(fd, buf, 0, 2), 2);
expect('read data', String.fromCharCode(buf[0], buf[1]), '01');
expect('positioned write', fs.write(fd, new Uint8Array([65, 66]), 8), 2);
expect('sequential write', fs.write(fd, new Uint8Array([67]), undefined), 1);
expect('null position write', fs.write(fd, new Uint8Array([68]), null), 1);
fs.close(fd);
expect('file contents', String.fromCharCode.apply(null, fs.readFile('/work/data.txt')), '01CD4567AB');

expectThrow('readAt arity', () => fs.readAt(1, buf, 0, 1), /readAt requires 5 arguments/);
expectThrow('readAt buffer', () => fs.readAt(1, 'x', 0, 1, 0), /buffer argument must be a Uint8Array/);
expectThrow('readAt bad fd', () => fs.readAt(9999, buf, 0, 1, 0), /invalid argument/);
expectThrow('write arity', () => fs.write(1), /write requires 2 arguments/);
expectThrow('write data', () => fs.write(1, 'x'), /data argument must be a Uint8Array/);
expectThrow('write bad fd', () => fs.write(9999, new Uint8Array([1]), 0), /invalid argument/);

expect('errorCode undefined', fs.errorCode(undefined), '');
expect('errorCode null', fs.errorCode(null), '');
expect('errorCode non-error', fs.errorCode('text'), '');
let missing;
try { fs.stat('/work/missing.txt'); } catch (e) { missing = e; }
expect('errorCode missing', fs.errorCode(missing && missing.value !== undefined ? missing.value : missing), 'ENOENT');
`, os.O_RDWR)
	var out bytes.Buffer
	jr, err := New(Config{
		Name:   "fs-exports",
		Code:   script,
		FSTabs: FSTabs{{MountPoint: "/work", Source: dir}},
		Reader: &bytes.Buffer{},
		Writer: &out,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	jr.RegisterNativeModule("@jsh/fs", jr.Filesystem)
	if err := jr.Run(); err != nil {
		t.Fatalf("Run() error = %v\noutput: %s", err, out.String())
	}
}
