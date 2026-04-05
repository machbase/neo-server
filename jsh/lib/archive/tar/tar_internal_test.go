package tar

import (
	stdtar "archive/tar"
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func mustRunTarValue(t *testing.T, rt *goja.Runtime, script string) goja.Value {
	t.Helper()
	value, err := rt.RunString(script)
	if err != nil {
		t.Fatalf("RunString(%q) failed: %v", script, err)
	}
	return value
}

func mustTarCallable(t *testing.T, rt *goja.Runtime, fn func(goja.FunctionCall) goja.Value) goja.Callable {
	t.Helper()
	callable, ok := goja.AssertFunction(rt.ToValue(fn))
	if !ok {
		t.Fatal("expected callable")
	}
	return callable
}

func mustTarJSFunction(t *testing.T, value goja.Value) goja.Callable {
	t.Helper()
	callable, ok := goja.AssertFunction(value)
	if !ok {
		t.Fatal("expected JavaScript function")
	}
	return callable
}

func tarCallMustFail(t *testing.T, fn goja.Callable, this goja.Value, args ...goja.Value) error {
	t.Helper()
	_, err := fn(this, args...)
	if err == nil {
		t.Fatal("expected call to fail")
	}
	return err
}

func TestTarFilesAndDefaultEntryName(t *testing.T) {
	files := Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 embedded file, got %d", len(files))
	}
	if len(files["archive/tar.js"]) == 0 {
		t.Fatal("embedded tar.js must not be empty")
	}
	if got := defaultEntryNameFor(1); got != defaultTarEntryName {
		t.Fatalf("unexpected first default entry name: %s", got)
	}
	if got := defaultEntryNameFor(3); got != "entry-3" {
		t.Fatalf("unexpected indexed default entry name: %s", got)
	}
	if got := typeName(stdtar.TypeDir); got != "dir" {
		t.Fatalf("unexpected dir type name: %s", got)
	}
	if got := typeName(stdtar.TypeSymlink); got != "symlink" {
		t.Fatalf("unexpected symlink type name: %s", got)
	}
	if got := typeName(stdtar.TypeLink); got != "link" {
		t.Fatalf("unexpected hard link type name: %s", got)
	}
	if got := typeName(stdtar.TypeReg); got != "file" {
		t.Fatalf("unexpected regular type name: %s", got)
	}
}

func TestTarResolveTypeflagAndNormalizeEntry(t *testing.T) {
	rt := goja.New()

	for _, tc := range []struct {
		name string
		text string
		want byte
	}{
		{name: "regular", text: "regular", want: stdtar.TypeReg},
		{name: "dir", text: "directory", want: stdtar.TypeDir},
		{name: "symlink", text: "symlink", want: stdtar.TypeSymlink},
		{name: "hardlink", text: "hardlink", want: stdtar.TypeLink},
	} {
		got, err := resolveTypeflag(tc.text)
		if err != nil {
			t.Fatalf("resolveTypeflag(%q) failed: %v", tc.text, err)
		}
		if got != tc.want {
			t.Fatalf("resolveTypeflag(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
	if _, err := resolveTypeflag("unknown"); err == nil {
		t.Fatal("expected unsupported tar type error")
	}

	fileEntry, err := normalizeEntry(rt, rt.ToValue(rt.NewArrayBuffer([]byte("alpha"))), "fallback")
	if err != nil {
		t.Fatalf("normalizeEntry(arrayBuffer) failed: %v", err)
	}
	if fileEntry.Name != "fallback" || fileEntry.Size != 5 || fileEntry.Typeflag != stdtar.TypeReg {
		t.Fatalf("unexpected file entry: %+v", fileEntry)
	}

	dirEntry, err := normalizeEntry(rt, mustRunTarValue(t, rt, `({ name: 'docs', isDir: true, type: 'dir' })`), "fallback")
	if err != nil {
		t.Fatalf("normalizeEntry(dir) failed: %v", err)
	}
	if !dirEntry.IsDir || dirEntry.Typeflag != stdtar.TypeDir || dirEntry.Name != "docs/" || dirEntry.Size != 0 {
		t.Fatalf("unexpected dir entry: %+v", dirEntry)
	}

	linkEntry, err := normalizeEntry(rt, mustRunTarValue(t, rt, `({ name: 'latest', type: 'symlink', linkname: 'docs/readme.txt' })`), "fallback")
	if err != nil {
		t.Fatalf("normalizeEntry(link) failed: %v", err)
	}
	if linkEntry.Typeflag != stdtar.TypeSymlink || linkEntry.Linkname != "docs/readme.txt" || linkEntry.Size != 0 {
		t.Fatalf("unexpected link entry: %+v", linkEntry)
	}

	metaOnlyEntry, err := normalizeEntry(rt, mustRunTarValue(t, rt, `({ name: 'fifo', typeflag: 54 })`), "fallback")
	if err != nil {
		t.Fatalf("normalizeEntry(metaOnly) failed: %v", err)
	}
	if metaOnlyEntry.Size != 0 || metaOnlyEntry.Typeflag != 54 {
		t.Fatalf("unexpected metadata-only entry: %+v", metaOnlyEntry)
	}

	if _, err := normalizeEntry(rt, mustRunTarValue(t, rt, `({ name: 'broken' })`), "fallback"); err == nil {
		t.Fatal("expected missing data error")
	}
	if _, err := normalizeEntry(rt, mustRunTarValue(t, rt, `({ name: 'bad', type: 'invalid', data: 'x' })`), "fallback"); err == nil {
		t.Fatal("expected invalid type error")
	}
}

func TestTarNormalizeEntries(t *testing.T) {
	rt := goja.New()

	entries, total, err := normalizeEntries(rt, mustRunTarValue(t, rt, `[new Uint8Array([65, 66]), { name: 'two.txt', data: 'cd' }]`))
	if err != nil {
		t.Fatalf("normalizeEntries(array) failed: %v", err)
	}
	if len(entries) != 2 || total != 4 {
		t.Fatalf("unexpected entries/total: len=%d total=%d", len(entries), total)
	}
	if entries[0].Name != defaultTarEntryName || entries[1].Name != "two.txt" {
		t.Fatalf("unexpected normalized entry names: %+v", entries)
	}

	if _, _, err := normalizeEntries(rt, goja.Undefined()); err == nil {
		t.Fatal("expected required data error")
	}
}

func TestTarBytesFromValue(t *testing.T) {
	rt := goja.New()

	if buf, err := bytesFromValue(rt, goja.Undefined()); err != nil || buf != nil {
		t.Fatalf("unexpected undefined conversion result: %v %v", buf, err)
	}
	if buf, err := bytesFromValue(rt, rt.ToValue("abc")); err != nil || string(buf) != "abc" {
		t.Fatalf("unexpected string conversion result: %q %v", string(buf), err)
	}
	if buf, err := bytesFromValue(rt, rt.ToValue([]byte("raw"))); err != nil || string(buf) != "raw" {
		t.Fatalf("unexpected bytes conversion result: %q %v", string(buf), err)
	}
	if buf, err := bytesFromValue(rt, mustRunTarValue(t, rt, `[65, 66, 67]`)); err != nil || string(buf) != "ABC" {
		t.Fatalf("unexpected array conversion result: %q %v", string(buf), err)
	}
	if buf, err := bytesFromValue(rt, mustRunTarValue(t, rt, `new Uint8Array([68, 69, 70]).subarray(1)`)); err != nil || string(buf) != "EF" {
		t.Fatalf("unexpected typed array conversion result: %q %v", string(buf), err)
	}
	if _, err := bytesFromValue(rt, mustRunTarValue(t, rt, `({ plain: true })`)); err == nil {
		t.Fatal("expected plain object conversion error")
	}

	buf, err := bytesFromAny(rt, rt.ToValue("goja-value"))
	if err != nil || string(buf) != "goja-value" {
		t.Fatalf("unexpected bytesFromAny(goja.Value) result: %q %v", string(buf), err)
	}
	buf, err = bytesFromAny(rt, "native")
	if err != nil || string(buf) != "native" {
		t.Fatalf("unexpected bytesFromAny(native) result: %q %v", string(buf), err)
	}
	if value := toGojaValue(rt, "value"); value.String() != "value" {
		t.Fatalf("unexpected goja value: %s", value.String())
	}
}

func TestTarCreateAndOpenArchive(t *testing.T) {
	modified := time.Date(2024, time.March, 4, 5, 6, 7, 0, time.UTC)
	archive, err := createArchive([]archiveEntry{
		{Name: "docs", IsDir: true, Typeflag: stdtar.TypeDir, Modified: modified},
		{Name: "docs/readme.txt", Data: []byte("hello"), Mode: 0600, Modified: modified, Typeflag: stdtar.TypeReg},
		{Name: "latest", Typeflag: stdtar.TypeSymlink, Linkname: "docs/readme.txt", Modified: modified},
	})
	if err != nil {
		t.Fatalf("createArchive failed: %v", err)
	}

	entries, err := openArchive(archive)
	if err != nil {
		t.Fatalf("openArchive failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if !entries[0].IsDir || entries[0].Name != "docs/" {
		t.Fatalf("unexpected dir entry: %+v", entries[0])
	}
	if string(entries[1].Data) != "hello" || entries[1].Size != 5 {
		t.Fatalf("unexpected file entry: %+v", entries[1])
	}
	if entries[2].Typeflag != stdtar.TypeSymlink || entries[2].Linkname != "docs/readme.txt" {
		t.Fatalf("unexpected symlink entry: %+v", entries[2])
	}

	if _, err := openArchive([]byte("not-a-tar")); err == nil {
		t.Fatal("expected invalid tar data error")
	}
}

func TestTarEntryValueAndEntriesValue(t *testing.T) {
	rt := goja.New()
	entry := archiveEntry{
		Name:     "file.txt",
		Data:     []byte("abc"),
		Mode:     0640,
		Size:     3,
		IsDir:    false,
		Modified: time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
		Typeflag: stdtar.TypeReg,
		Linkname: "",
	}
	value := entryValue(rt, entry).ToObject(rt)
	if value.Get("name").String() != "file.txt" || value.Get("type").String() != "file" {
		t.Fatalf("unexpected entryValue metadata: name=%s type=%s", value.Get("name").String(), value.Get("type").String())
	}
	data, err := bytesFromValue(rt, value.Get("data"))
	if err != nil || string(data) != "abc" {
		t.Fatalf("unexpected entryValue data: %q %v", string(data), err)
	}
	entriesArray := entriesValue(rt, []archiveEntry{entry, {Name: "dir/", IsDir: true, Typeflag: stdtar.TypeDir}}).ToObject(rt)
	if entriesArray.Get("length").ToInteger() != 2 {
		t.Fatalf("unexpected array length: %d", entriesArray.Get("length").ToInteger())
	}
}

func TestTarWriterReaderAndPipe(t *testing.T) {
	rt := goja.New()
	writer := newArchiveWriter(rt)
	reader := newArchiveReader(rt)
	exportArchiveWriter(rt, writer)
	exportArchiveReader(rt, reader)

	var piped bytes.Buffer
	if _, err := writer.Pipe(&piped); err != nil {
		t.Fatalf("Pipe(io.Writer) failed: %v", err)
	}
	if _, err := writer.Write(map[string]any{"name": "one.txt", "data": "One"}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := writer.End(map[string]any{"name": "two.txt", "data": "Two"}); err != nil {
		t.Fatalf("End failed: %v", err)
	}
	if writer.bytesWritten != 6 || writer.bytesRead == 0 || piped.Len() == 0 {
		t.Fatalf("unexpected writer stats: written=%d read=%d piped=%d", writer.bytesWritten, writer.bytesRead, piped.Len())
	}
	if got := writer.obj.Get("bytesWritten").ToInteger(); got != writer.bytesWritten {
		t.Fatalf("writer object bytesWritten mismatch: %d != %d", got, writer.bytesWritten)
	}

	var gotNames []string
	var gotBodies []string
	reader.On("entry", mustTarCallable(t, rt, func(call goja.FunctionCall) goja.Value {
		entryObj := call.Argument(0).ToObject(rt)
		gotNames = append(gotNames, entryObj.Get("name").String())
		body, err := bytesFromValue(rt, entryObj.Get("data"))
		if err != nil {
			t.Fatalf("bytesFromValue(entry.data) failed: %v", err)
		}
		gotBodies = append(gotBodies, string(body))
		return goja.Undefined()
	}))
	ended := false
	reader.On("end", mustTarCallable(t, rt, func(call goja.FunctionCall) goja.Value {
		ended = true
		return goja.Undefined()
	}))
	if _, err := reader.Write(piped.Bytes()); err != nil {
		t.Fatalf("reader.Write failed: %v", err)
	}
	if err := reader.End(); err != nil {
		t.Fatalf("reader.End failed: %v", err)
	}
	if !reflect.DeepEqual(gotNames, []string{"one.txt", "two.txt"}) || !reflect.DeepEqual(gotBodies, []string{"One", "Two"}) || !ended {
		t.Fatalf("unexpected reader output names=%v bodies=%v ended=%v", gotNames, gotBodies, ended)
	}
	if reader.bytesWritten != int64(piped.Len()) || reader.bytesRead != 6 {
		t.Fatalf("unexpected reader stats: written=%d read=%d", reader.bytesWritten, reader.bytesRead)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader.Close failed: %v", err)
	}
	if reader.buffer.Len() != 0 {
		t.Fatal("reader buffer should be reset after Close")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close failed: %v", err)
	}

	jsWriter := newArchiveWriter(rt)
	exportArchiveWriter(rt, jsWriter)
	chunks := 0
	ended = false
	dest := rt.NewObject()
	dest.Set("write", func(call goja.FunctionCall) goja.Value {
		chunks++
		return goja.Undefined()
	})
	dest.Set("end", func(call goja.FunctionCall) goja.Value {
		ended = true
		return goja.Undefined()
	})
	if _, err := jsWriter.Pipe(dest); err != nil {
		t.Fatalf("Pipe(js object) failed: %v", err)
	}
	if err := jsWriter.End(map[string]any{"name": "js.txt", "data": strings.Repeat("z", 3)}); err != nil {
		t.Fatalf("jsWriter.End failed: %v", err)
	}
	if chunks != 1 || !ended {
		t.Fatalf("unexpected js pipe state: chunks=%d ended=%v", chunks, ended)
	}

	if _, err := newArchiveWriter(rt).Pipe(123); err == nil {
		t.Fatal("expected unsupported destination error")
	}
	if _, err := newArchiveReader(rt).Write(map[string]any{"plain": true}); err == nil {
		t.Fatal("expected reader write conversion error")
	}
	if _, err := newArchiveWriter(rt).Write(goja.Undefined()); err == nil {
		t.Fatal("expected writer write required data error")
	}
	if err := newArchiveReader(rt).End([]byte("not-a-tar")); err == nil {
		t.Fatal("expected reader end invalid archive error")
	}
}

func TestTarModuleExportsAndAsyncValidation(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	Module(context.Background(), rt, module)
	exports = module.Get("exports").(*goja.Object)

	createTarFn := mustTarJSFunction(t, exports.Get("createTar"))
	createUntarFn := mustTarJSFunction(t, exports.Get("createUntar"))
	tarSyncFn := mustTarJSFunction(t, exports.Get("tarSync"))
	untarSyncFn := mustTarJSFunction(t, exports.Get("untarSync"))
	tarFn := mustTarJSFunction(t, exports.Get("tar"))
	untarFn := mustTarJSFunction(t, exports.Get("untar"))

	writerValue, err := createTarFn(exports)
	if err != nil {
		t.Fatalf("createTar() failed: %v", err)
	}
	writerObj := writerValue.ToObject(rt)
	writerWriteFn := mustTarJSFunction(t, writerObj.Get("write"))
	writerOnFn := mustTarJSFunction(t, writerObj.Get("on"))
	writerEndFn := mustTarJSFunction(t, writerObj.Get("end"))
	writerPipeFn := mustTarJSFunction(t, writerObj.Get("pipe"))
	writerCloseFn := mustTarJSFunction(t, writerObj.Get("close"))

	readerValue, err := createUntarFn(exports)
	if err != nil {
		t.Fatalf("createUntar() failed: %v", err)
	}
	readerObj := readerValue.ToObject(rt)
	readerWriteFn := mustTarJSFunction(t, readerObj.Get("write"))
	readerOnFn := mustTarJSFunction(t, readerObj.Get("on"))
	readerEndFn := mustTarJSFunction(t, readerObj.Get("end"))
	readerCloseFn := mustTarJSFunction(t, readerObj.Get("close"))

	proxyWriter := writerObj.Get("writer").Export().(archiveWriterProxy)
	if n, err := proxyWriter.Write([]byte("proxy")); err != nil || n <= 0 {
		t.Fatalf("archiveWriterProxy.Write() = (%d, %v)", n, err)
	}
	if err := proxyWriter.Close(); err != nil {
		t.Fatalf("archiveWriterProxy.Close() failed: %v", err)
	}

	callbackValue := rt.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Undefined()
	})
	if _, err := writerOnFn(writerObj, rt.ToValue("data"), callbackValue); err != nil {
		t.Fatalf("writer.on(data) failed: %v", err)
	}
	if _, err := writerOnFn(writerObj, rt.ToValue("error"), callbackValue); err != nil {
		t.Fatalf("writer.on(error) failed: %v", err)
	}
	if _, err := readerOnFn(readerObj, rt.ToValue("entry"), callbackValue); err != nil {
		t.Fatalf("reader.on(entry) failed: %v", err)
	}
	if _, err := readerOnFn(readerObj, rt.ToValue("error"), callbackValue); err != nil {
		t.Fatalf("reader.on(error) failed: %v", err)
	}

	archiveValue, err := tarSyncFn(exports, rt.ToValue("payload"))
	if err != nil {
		t.Fatalf("tarSync() failed: %v", err)
	}
	if _, err := untarSyncFn(exports, archiveValue); err != nil {
		t.Fatalf("untarSync() failed: %v", err)
	}
	if _, err := writerWriteFn(writerObj, rt.ToValue(map[string]any{"name": "one.txt", "data": "One"})); err != nil {
		t.Fatalf("writer.write() failed: %v", err)
	}
	if _, err := writerPipeFn(writerObj, readerObj); err != nil {
		t.Fatalf("writer.pipe() failed: %v", err)
	}
	if _, err := writerEndFn(writerObj, rt.ToValue(map[string]any{"name": "two.txt", "data": "Two"})); err != nil {
		t.Fatalf("writer.end() failed: %v", err)
	}
	if _, err := readerCloseFn(readerObj); err != nil {
		t.Fatalf("reader.close() failed: %v", err)
	}
	if _, err := writerCloseFn(writerObj); err != nil {
		t.Fatalf("writer.close() failed: %v", err)
	}

	if err := tarCallMustFail(t, writerWriteFn, writerObj); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected writer.write() error: %v", err)
	}
	if err := tarCallMustFail(t, writerOnFn, writerObj, rt.ToValue("data")); !strings.Contains(err.Error(), "callback") {
		t.Fatalf("unexpected writer.on() error: %v", err)
	}
	if err := tarCallMustFail(t, writerPipeFn, writerObj); !strings.Contains(err.Error(), "destination") {
		t.Fatalf("unexpected writer.pipe() error: %v", err)
	}
	if err := tarCallMustFail(t, readerWriteFn, readerObj); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected reader.write() error: %v", err)
	}
	if err := tarCallMustFail(t, readerEndFn, readerObj, rt.ToValue([]byte("bad"))); !strings.Contains(err.Error(), "archive/tar") && !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("unexpected reader.end() error: %v", err)
	}
	if err := tarCallMustFail(t, tarSyncFn, exports); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected tarSync() error: %v", err)
	}
	if err := tarCallMustFail(t, untarSyncFn, exports); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected untarSync() error: %v", err)
	}
	if err := tarCallMustFail(t, tarSyncFn, exports, mustRunTarValue(t, rt, `({ plain: true })`)); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected tarSync(object) error: %v", err)
	}
	if err := tarCallMustFail(t, untarSyncFn, exports, mustRunTarValue(t, rt, `({ plain: true })`)); !strings.Contains(err.Error(), "Buffer or string") {
		t.Fatalf("unexpected untarSync(object) error: %v", err)
	}
	if err := tarCallMustFail(t, tarFn, exports, rt.ToValue("payload")); !strings.Contains(err.Error(), "callback") {
		t.Fatalf("unexpected tar() error: %v", err)
	}
	if err := tarCallMustFail(t, tarFn, exports, mustRunTarValue(t, rt, `({ plain: true })`), callbackValue); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected tar(object) error: %v", err)
	}
	if err := tarCallMustFail(t, tarFn, exports, rt.ToValue("payload"), rt.ToValue("not-a-function")); !strings.Contains(err.Error(), "callback function") {
		t.Fatalf("unexpected tar(bad callback) error: %v", err)
	}
	if err := tarCallMustFail(t, untarFn, exports, archiveValue); !strings.Contains(err.Error(), "callback") {
		t.Fatalf("unexpected untar() error: %v", err)
	}
	if err := tarCallMustFail(t, untarFn, exports, mustRunTarValue(t, rt, `({ plain: true })`), callbackValue); !strings.Contains(err.Error(), "Buffer or string") {
		t.Fatalf("unexpected untar(object) error: %v", err)
	}
	if err := tarCallMustFail(t, untarFn, exports, archiveValue, rt.ToValue("not-a-function")); !strings.Contains(err.Error(), "callback function") {
		t.Fatalf("unexpected untar(bad callback) error: %v", err)
	}
}
