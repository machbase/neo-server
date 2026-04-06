package zip

import (
	stdzip "archive/zip"
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func mustRunZipValue(t *testing.T, rt *goja.Runtime, script string) goja.Value {
	t.Helper()
	value, err := rt.RunString(script)
	if err != nil {
		t.Fatalf("RunString(%q) failed: %v", script, err)
	}
	return value
}

func mustZipCallable(t *testing.T, rt *goja.Runtime, fn func(goja.FunctionCall) goja.Value) goja.Callable {
	t.Helper()
	callable, ok := goja.AssertFunction(rt.ToValue(fn))
	if !ok {
		t.Fatal("expected callable")
	}
	return callable
}

func mustZipJSFunction(t *testing.T, value goja.Value) goja.Callable {
	t.Helper()
	callable, ok := goja.AssertFunction(value)
	if !ok {
		t.Fatal("expected JavaScript function")
	}
	return callable
}

func zipCallMustFail(t *testing.T, fn goja.Callable, this goja.Value, args ...goja.Value) error {
	t.Helper()
	_, err := fn(this, args...)
	if err == nil {
		t.Fatal("expected call to fail")
	}
	return err
}

func TestZipFilesAndDefaultEntryName(t *testing.T) {
	files := Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 embedded file, got %d", len(files))
	}
	if len(files["archive/zip.js"]) == 0 {
		t.Fatal("embedded zip.js must not be empty")
	}
	if got := defaultEntryNameFor(1); got != defaultZipEntryName {
		t.Fatalf("unexpected first default entry name: %s", got)
	}
	if got := defaultEntryNameFor(4); got != "entry-4" {
		t.Fatalf("unexpected indexed default entry name: %s", got)
	}
}

func TestZipNormalizeEntryAndEntries(t *testing.T) {
	rt := goja.New()

	entry, err := normalizeEntry(rt, rt.ToValue(rt.NewArrayBuffer([]byte("alpha"))), "fallback")
	if err != nil {
		t.Fatalf("normalizeEntry(arrayBuffer) failed: %v", err)
	}
	if entry.Name != "fallback" || entry.Uncompressed != 5 || entry.Method != stdzip.Deflate {
		t.Fatalf("unexpected buffer entry: %+v", entry)
	}

	storeEntry, err := normalizeEntry(rt, mustRunZipValue(t, rt, `({ name: 'docs/readme.txt', data: 'hello', comment: 'note', method: 'store' })`), "fallback")
	if err != nil {
		t.Fatalf("normalizeEntry(object) failed: %v", err)
	}
	if storeEntry.Name != "docs/readme.txt" || storeEntry.Comment != "note" || storeEntry.Method != stdzip.Store || storeEntry.Uncompressed != 5 {
		t.Fatalf("unexpected object entry: %+v", storeEntry)
	}

	entries, total, err := normalizeEntries(rt, mustRunZipValue(t, rt, `[new Uint8Array([65, 66]), { name: 'two.txt', data: 'cd', method: 'default' }]`))
	if err != nil {
		t.Fatalf("normalizeEntries(array) failed: %v", err)
	}
	if len(entries) != 2 || total != 4 {
		t.Fatalf("unexpected entries/total: len=%d total=%d", len(entries), total)
	}
	if entries[0].Name != defaultZipEntryName || entries[1].Name != "two.txt" {
		t.Fatalf("unexpected normalized entry names: %+v", entries)
	}

	if _, _, err := normalizeEntries(rt, goja.Undefined()); err == nil {
		t.Fatal("expected required data error")
	}
	if _, err := normalizeEntry(rt, mustRunZipValue(t, rt, `({ name: 'broken' })`), "fallback"); err == nil {
		t.Fatal("expected missing data error")
	}
	if _, err := normalizeEntry(rt, mustRunZipValue(t, rt, `({ name: 'bad', data: 'x', method: 'lzma' })`), "fallback"); err == nil {
		t.Fatal("expected unsupported method error")
	}
}

func TestZipBytesFromValue(t *testing.T) {
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
	if buf, err := bytesFromValue(rt, mustRunZipValue(t, rt, `[65, 66, 67]`)); err != nil || string(buf) != "ABC" {
		t.Fatalf("unexpected array conversion result: %q %v", string(buf), err)
	}
	if buf, err := bytesFromValue(rt, mustRunZipValue(t, rt, `new Uint8Array([68, 69, 70]).subarray(1)`)); err != nil || string(buf) != "EF" {
		t.Fatalf("unexpected typed array conversion result: %q %v", string(buf), err)
	}
	if _, err := bytesFromValue(rt, mustRunZipValue(t, rt, `({ plain: true })`)); err == nil {
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

func TestZipCreateAndOpenArchive(t *testing.T) {
	modified := time.Date(2024, time.March, 4, 5, 6, 7, 0, time.UTC)
	archive, err := createArchive([]archiveEntry{
		{Name: "docs/", IsDir: true, Method: stdzip.Store, Modified: modified, Comment: "dir"},
		{Name: "docs/readme.txt", Data: []byte("hello"), Method: stdzip.Store, Modified: modified, Comment: "note"},
	})
	if err != nil {
		t.Fatalf("createArchive failed: %v", err)
	}

	entries, err := openArchive(archive)
	if err != nil {
		t.Fatalf("openArchive failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if !entries[0].IsDir || entries[0].Name != "docs/" || entries[0].Comment != "dir" {
		t.Fatalf("unexpected dir entry: %+v", entries[0])
	}
	if string(entries[1].Data) != "hello" || entries[1].Method != stdzip.Store || entries[1].Comment != "note" {
		t.Fatalf("unexpected file entry: %+v", entries[1])
	}

	if _, err := openArchive([]byte("not-a-zip")); err == nil {
		t.Fatal("expected invalid zip data error")
	}
}

func TestZipEntryValueAndEntriesValue(t *testing.T) {
	rt := goja.New()
	entry := archiveEntry{
		Name:           "file.txt",
		Data:           []byte("abc"),
		Comment:        "note",
		Method:         stdzip.Store,
		CompressedSize: 3,
		Uncompressed:   3,
		IsDir:          false,
		Modified:       time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
	}
	value := entryValue(rt, entry).ToObject(rt)
	if value.Get("name").String() != "file.txt" || value.Get("comment").String() != "note" {
		t.Fatalf("unexpected entryValue metadata: name=%s comment=%s", value.Get("name").String(), value.Get("comment").String())
	}
	data, err := bytesFromValue(rt, value.Get("data"))
	if err != nil || string(data) != "abc" {
		t.Fatalf("unexpected entryValue data: %q %v", string(data), err)
	}
	entriesArray := entriesValue(rt, []archiveEntry{entry, {Name: "dir/", IsDir: true}}).ToObject(rt)
	if entriesArray.Get("length").ToInteger() != 2 {
		t.Fatalf("unexpected array length: %d", entriesArray.Get("length").ToInteger())
	}
}

func TestZipWriterReaderAndPipe(t *testing.T) {
	rt := goja.New()
	writer := newArchiveWriter(rt)
	reader := newArchiveReader(rt)
	exportArchiveWriter(rt, writer)
	exportArchiveReader(rt, reader)

	var piped bytes.Buffer
	if _, err := writer.Pipe(&piped); err != nil {
		t.Fatalf("Pipe(io.Writer) failed: %v", err)
	}
	if _, err := writer.Write(map[string]any{"name": "one.txt", "data": "One", "comment": "first"}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := writer.End(map[string]any{"name": "two.txt", "data": "Two", "method": "store"}); err != nil {
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
	var gotComments []string
	reader.On("entry", mustZipCallable(t, rt, func(call goja.FunctionCall) goja.Value {
		entryObj := call.Argument(0).ToObject(rt)
		gotNames = append(gotNames, entryObj.Get("name").String())
		gotComments = append(gotComments, entryObj.Get("comment").String())
		body, err := bytesFromValue(rt, entryObj.Get("data"))
		if err != nil {
			t.Fatalf("bytesFromValue(entry.data) failed: %v", err)
		}
		gotBodies = append(gotBodies, string(body))
		return goja.Undefined()
	}))
	ended := false
	reader.On("end", mustZipCallable(t, rt, func(call goja.FunctionCall) goja.Value {
		ended = true
		return goja.Undefined()
	}))
	if _, err := reader.Write(piped.Bytes()); err != nil {
		t.Fatalf("reader.Write failed: %v", err)
	}
	if err := reader.End(); err != nil {
		t.Fatalf("reader.End failed: %v", err)
	}
	if !reflect.DeepEqual(gotNames, []string{"one.txt", "two.txt"}) || !reflect.DeepEqual(gotBodies, []string{"One", "Two"}) || !reflect.DeepEqual(gotComments, []string{"first", ""}) || !ended {
		t.Fatalf("unexpected reader output names=%v bodies=%v comments=%v ended=%v", gotNames, gotBodies, gotComments, ended)
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
	if err := newArchiveReader(rt).End([]byte("not-a-zip")); err == nil {
		t.Fatal("expected reader end invalid archive error")
	}
}

func TestZipModuleExportsAndAsyncValidation(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	Module(context.Background(), rt, module)
	exports = module.Get("exports").(*goja.Object)

	createZipFn := mustZipJSFunction(t, exports.Get("createZip"))
	createUnzipFn := mustZipJSFunction(t, exports.Get("createUnzip"))
	zipSyncFn := mustZipJSFunction(t, exports.Get("zipSync"))
	unzipSyncFn := mustZipJSFunction(t, exports.Get("unzipSync"))
	zipFn := mustZipJSFunction(t, exports.Get("zip"))
	unzipFn := mustZipJSFunction(t, exports.Get("unzip"))

	writerValue, err := createZipFn(exports)
	if err != nil {
		t.Fatalf("createZip() failed: %v", err)
	}
	writerObj := writerValue.ToObject(rt)
	writerWriteFn := mustZipJSFunction(t, writerObj.Get("write"))
	writerOnFn := mustZipJSFunction(t, writerObj.Get("on"))
	writerEndFn := mustZipJSFunction(t, writerObj.Get("end"))
	writerPipeFn := mustZipJSFunction(t, writerObj.Get("pipe"))
	writerCloseFn := mustZipJSFunction(t, writerObj.Get("close"))

	readerValue, err := createUnzipFn(exports)
	if err != nil {
		t.Fatalf("createUnzip() failed: %v", err)
	}
	readerObj := readerValue.ToObject(rt)
	readerWriteFn := mustZipJSFunction(t, readerObj.Get("write"))
	readerOnFn := mustZipJSFunction(t, readerObj.Get("on"))
	readerEndFn := mustZipJSFunction(t, readerObj.Get("end"))
	readerCloseFn := mustZipJSFunction(t, readerObj.Get("close"))

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

	archiveValue, err := zipSyncFn(exports, rt.ToValue("payload"))
	if err != nil {
		t.Fatalf("zipSync() failed: %v", err)
	}
	if _, err := unzipSyncFn(exports, archiveValue); err != nil {
		t.Fatalf("unzipSync() failed: %v", err)
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

	if err := zipCallMustFail(t, writerWriteFn, writerObj); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected writer.write() error: %v", err)
	}
	if err := zipCallMustFail(t, writerOnFn, writerObj, rt.ToValue("data")); !strings.Contains(err.Error(), "callback") {
		t.Fatalf("unexpected writer.on() error: %v", err)
	}
	if err := zipCallMustFail(t, writerPipeFn, writerObj); !strings.Contains(err.Error(), "destination") {
		t.Fatalf("unexpected writer.pipe() error: %v", err)
	}
	if err := zipCallMustFail(t, readerWriteFn, readerObj); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected reader.write() error: %v", err)
	}
	if err := zipCallMustFail(t, readerEndFn, readerObj, rt.ToValue([]byte("bad"))); !strings.Contains(err.Error(), "zip") && !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("unexpected reader.end() error: %v", err)
	}
	if err := zipCallMustFail(t, zipSyncFn, exports); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected zipSync() error: %v", err)
	}
	if err := zipCallMustFail(t, unzipSyncFn, exports); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected unzipSync() error: %v", err)
	}
	if err := zipCallMustFail(t, zipSyncFn, exports, mustRunZipValue(t, rt, `({ plain: true })`)); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected zipSync(object) error: %v", err)
	}
	if err := zipCallMustFail(t, unzipSyncFn, exports, mustRunZipValue(t, rt, `({ plain: true })`)); !strings.Contains(err.Error(), "Buffer or string") {
		t.Fatalf("unexpected unzipSync(object) error: %v", err)
	}
	if err := zipCallMustFail(t, zipFn, exports, rt.ToValue("payload")); !strings.Contains(err.Error(), "callback") {
		t.Fatalf("unexpected zip() error: %v", err)
	}
	if err := zipCallMustFail(t, zipFn, exports, mustRunZipValue(t, rt, `({ plain: true })`), callbackValue); !strings.Contains(err.Error(), "data is required") {
		t.Fatalf("unexpected zip(object) error: %v", err)
	}
	if err := zipCallMustFail(t, zipFn, exports, rt.ToValue("payload"), rt.ToValue("not-a-function")); !strings.Contains(err.Error(), "callback function") {
		t.Fatalf("unexpected zip(bad callback) error: %v", err)
	}
	if err := zipCallMustFail(t, unzipFn, exports, archiveValue); !strings.Contains(err.Error(), "callback") {
		t.Fatalf("unexpected unzip() error: %v", err)
	}
	if err := zipCallMustFail(t, unzipFn, exports, mustRunZipValue(t, rt, `({ plain: true })`), callbackValue); !strings.Contains(err.Error(), "Buffer or string") {
		t.Fatalf("unexpected unzip(object) error: %v", err)
	}
	if err := zipCallMustFail(t, unzipFn, exports, archiveValue, rt.ToValue("not-a-function")); !strings.Contains(err.Error(), "callback function") {
		t.Fatalf("unexpected unzip(bad callback) error: %v", err)
	}
}
