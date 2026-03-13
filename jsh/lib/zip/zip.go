package zip

import (
	stdzip "archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dop251/goja"
)

//go:embed zip.js
var zipJS []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"zip.js": zipJS,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	m := module.Get("exports").(*goja.Object)

	m.Set("createZip", func() goja.Value {
		return exportArchiveWriter(rt, newArchiveWriter(rt))
	})
	m.Set("createUnzip", func() goja.Value {
		return exportArchiveReader(rt, newArchiveReader(rt))
	})
	m.Set("zip", func(call goja.FunctionCall) goja.Value {
		return asyncZipArchive(rt, call)
	})
	m.Set("unzip", func(call goja.FunctionCall) goja.Value {
		return asyncUnzipArchive(rt, call)
	})
	m.Set("zipSync", func(call goja.FunctionCall) goja.Value {
		return syncZipArchive(rt, call)
	})
	m.Set("unzipSync", func(call goja.FunctionCall) goja.Value {
		return syncUnzipArchive(rt, call)
	})
}

type archiveWriter struct {
	rt              *goja.Runtime
	obj             *goja.Object
	onDataCallback  goja.Callable
	onEndCallback   goja.Callable
	onErrorCallback goja.Callable
	entries         []archiveEntry
	bytesWritten    int64
	bytesRead       int64
}

type archiveReader struct {
	rt              *goja.Runtime
	obj             *goja.Object
	onEntryCallback goja.Callable
	onEndCallback   goja.Callable
	onErrorCallback goja.Callable
	buffer          bytes.Buffer
	bytesWritten    int64
	bytesRead       int64
}

type archiveEntry struct {
	Name           string
	Data           []byte
	Comment        string
	Method         uint16
	Modified       time.Time
	CompressedSize uint64
	Uncompressed   uint64
	IsDir          bool
}

type archiveWriterProxy struct {
	*archiveWriter
}

func (w archiveWriterProxy) Write(data []byte) (int, error) {
	return w.archiveWriter.Write(data)
}

func (w archiveWriterProxy) Close() error {
	return w.archiveWriter.Close()
}

const defaultZipEntryName = "data"

func newArchiveWriter(rt *goja.Runtime) *archiveWriter {
	return &archiveWriter{rt: rt}
}

func newArchiveReader(rt *goja.Runtime) *archiveReader {
	return &archiveReader{rt: rt}
}

func exportArchiveWriter(rt *goja.Runtime, writer *archiveWriter) goja.Value {
	obj := rt.NewObject()
	writer.obj = obj
	writer.updateStats()
	obj.Set("writer", archiveWriterProxy{writer})
	obj.Set("write", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(rt.NewTypeError("data is required"))
		}
		n, err := writer.Write(call.Argument(0))
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(n > 0)
	})
	obj.Set("end", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			if err := writer.End(call.Argument(0)); err != nil {
				panic(rt.NewGoError(err))
			}
		} else {
			if err := writer.End(); err != nil {
				panic(rt.NewGoError(err))
			}
		}
		return goja.Undefined()
	})
	obj.Set("on", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(rt.NewTypeError("event and callback are required"))
		}
		event := call.Argument(0).String()
		callback, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			panic(rt.NewTypeError("callback must be a function"))
		}
		writer.On(event, callback)
		return rt.ToValue(obj)
	})
	obj.Set("pipe", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(rt.NewTypeError("destination is required"))
		}
		result, err := writer.Pipe(call.Argument(0))
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(result)
	})
	obj.Set("close", func(call goja.FunctionCall) goja.Value {
		if err := writer.Close(); err != nil {
			panic(rt.NewGoError(err))
		}
		return goja.Undefined()
	})
	return rt.ToValue(obj)
}

func exportArchiveReader(rt *goja.Runtime, reader *archiveReader) goja.Value {
	obj := rt.NewObject()
	reader.obj = obj
	reader.updateStats()
	obj.Set("write", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(rt.NewTypeError("data is required"))
		}
		n, err := reader.Write(call.Argument(0))
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(n > 0)
	})
	obj.Set("end", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			if err := reader.End(call.Argument(0)); err != nil {
				panic(rt.NewGoError(err))
			}
		} else {
			if err := reader.End(); err != nil {
				panic(rt.NewGoError(err))
			}
		}
		return goja.Undefined()
	})
	obj.Set("on", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(rt.NewTypeError("event and callback are required"))
		}
		event := call.Argument(0).String()
		callback, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			panic(rt.NewTypeError("callback must be a function"))
		}
		reader.On(event, callback)
		return rt.ToValue(obj)
	})
	obj.Set("close", func(call goja.FunctionCall) goja.Value {
		if err := reader.Close(); err != nil {
			panic(rt.NewGoError(err))
		}
		return goja.Undefined()
	})
	return rt.ToValue(obj)
}

func (w *archiveWriter) Write(data interface{}) (int, error) {
	entries, total, err := normalizeEntries(w.rt, toGojaValue(w.rt, data))
	if err != nil {
		return 0, err
	}
	w.entries = append(w.entries, entries...)
	w.bytesWritten += int64(total)
	w.updateStats()
	return total, nil
}

func (w *archiveWriter) End(data ...interface{}) error {
	if len(data) > 0 {
		if _, err := w.Write(data[0]); err != nil {
			return err
		}
	}
	archive, err := createArchive(w.entries)
	if err != nil {
		return err
	}
	if w.onDataCallback != nil {
		w.bytesRead += int64(len(archive))
		w.updateStats()
		buf := w.rt.NewArrayBuffer(archive)
		w.onDataCallback(goja.Undefined(), w.rt.ToValue(buf))
	}
	if w.onEndCallback != nil {
		w.onEndCallback(goja.Undefined())
	}
	return nil
}

func (w *archiveWriter) Pipe(dest interface{}) (interface{}, error) {
	var writer io.Writer
	var jsDestObj *goja.Object
	var jsWrite goja.Callable
	var jsEnd goja.Callable

	if destVal, ok := dest.(goja.Value); ok {
		if goja.IsUndefined(destVal) || goja.IsNull(destVal) {
			return nil, fmt.Errorf("destination is required")
		}
		obj := destVal.ToObject(w.rt)
		if obj == nil {
			return nil, fmt.Errorf("destination is required")
		}
		if endFn, ok := goja.AssertFunction(obj.Get("end")); ok {
			jsDestObj = obj
			jsEnd = endFn
		}
		if writerVal := obj.Get("writer"); writerVal != nil && !goja.IsUndefined(writerVal) && !goja.IsNull(writerVal) {
			exported := writerVal.Export()
			if wc, ok := exported.(io.WriteCloser); ok {
				writer = wc
			} else if ww, ok := exported.(io.Writer); ok {
				writer = ww
			}
		}
		if writer == nil {
			if writeFn, ok := goja.AssertFunction(obj.Get("write")); ok {
				jsDestObj = obj
				jsWrite = writeFn
			} else {
				return nil, fmt.Errorf("destination must support write")
			}
		}
	} else if wc, ok := dest.(io.WriteCloser); ok {
		writer = wc
	} else if ww, ok := dest.(io.Writer); ok {
		writer = ww
	} else {
		return nil, fmt.Errorf("destination must support write")
	}

	callbackValue := w.rt.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		chunk, err := bytesFromValue(w.rt, call.Argument(0))
		if err != nil {
			return goja.Undefined()
		}
		if writer != nil {
			_, _ = writer.Write(chunk)
		} else if jsWrite != nil {
			_, _ = jsWrite(jsDestObj, w.rt.ToValue(w.rt.NewArrayBuffer(chunk)))
		}
		return goja.Undefined()
	})
	callback, ok := goja.AssertFunction(callbackValue)
	if !ok {
		return nil, fmt.Errorf("failed to create pipe callback")
	}
	w.On("data", callback)
	if jsEnd != nil {
		endValue := w.rt.ToValue(func(call goja.FunctionCall) goja.Value {
			_, _ = jsEnd(jsDestObj)
			return goja.Undefined()
		})
		endCallback, ok := goja.AssertFunction(endValue)
		if !ok {
			return nil, fmt.Errorf("failed to create pipe end callback")
		}
		w.On("end", endCallback)
	}
	return dest, nil
}

func (w *archiveWriter) On(event string, callback goja.Callable) {
	switch event {
	case "data":
		w.onDataCallback = callback
	case "end":
		w.onEndCallback = callback
	case "error":
		w.onErrorCallback = callback
	}
}

func (w *archiveWriter) Close() error {
	return nil
}

func (w *archiveWriter) updateStats() {
	if w.obj == nil {
		return
	}
	w.obj.Set("bytesWritten", w.bytesWritten)
	w.obj.Set("bytesRead", w.bytesRead)
}

func (r *archiveReader) Write(data interface{}) (int, error) {
	buf, err := bytesFromAny(r.rt, data)
	if err != nil {
		return 0, err
	}
	if len(buf) == 0 {
		return 0, nil
	}
	n, err := r.buffer.Write(buf)
	r.bytesWritten += int64(n)
	r.updateStats()
	return n, err
}

func (r *archiveReader) End(data ...interface{}) error {
	if len(data) > 0 {
		if _, err := r.Write(data[0]); err != nil {
			return err
		}
	}
	entries, err := openArchive(r.buffer.Bytes())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if r.onEntryCallback != nil {
			r.bytesRead += int64(len(entry.Data))
			r.updateStats()
			r.onEntryCallback(goja.Undefined(), entryValue(r.rt, entry))
		}
	}
	if r.onEndCallback != nil {
		r.onEndCallback(goja.Undefined())
	}
	return nil
}

func (r *archiveReader) On(event string, callback goja.Callable) {
	switch event {
	case "entry":
		r.onEntryCallback = callback
	case "end":
		r.onEndCallback = callback
	case "error":
		r.onErrorCallback = callback
	}
}

func (r *archiveReader) Close() error {
	r.buffer.Reset()
	return nil
}

func (r *archiveReader) updateStats() {
	if r.obj == nil {
		return
	}
	r.obj.Set("bytesWritten", r.bytesWritten)
	r.obj.Set("bytesRead", r.bytesRead)
}

func asyncZipArchive(rt *goja.Runtime, call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(rt.NewTypeError("callback is required"))
	}
	entries, _, err := normalizeEntries(rt, call.Argument(0))
	if err != nil {
		panic(rt.NewTypeError(err.Error()))
	}
	callback, ok := goja.AssertFunction(call.Argument(len(call.Arguments) - 1))
	if !ok {
		panic(rt.NewTypeError("last argument must be a callback function"))
	}
	go func() {
		archive, zipErr := createArchive(entries)
		rt.Interrupt(func() {
			if zipErr != nil {
				callback(goja.Undefined(), rt.NewGoError(zipErr), goja.Null())
			} else {
				callback(goja.Undefined(), goja.Null(), rt.ToValue(rt.NewArrayBuffer(archive)))
			}
		})
	}()
	return goja.Undefined()
}

func asyncUnzipArchive(rt *goja.Runtime, call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(rt.NewTypeError("callback is required"))
	}
	buf, err := bytesFromValue(rt, call.Argument(0))
	if err != nil {
		panic(rt.NewTypeError(err.Error()))
	}
	callback, ok := goja.AssertFunction(call.Argument(len(call.Arguments) - 1))
	if !ok {
		panic(rt.NewTypeError("last argument must be a callback function"))
	}
	go func() {
		entries, unzipErr := openArchive(buf)
		rt.Interrupt(func() {
			if unzipErr != nil {
				callback(goja.Undefined(), rt.NewGoError(unzipErr), goja.Null())
			} else {
				callback(goja.Undefined(), goja.Null(), entriesValue(rt, entries))
			}
		})
	}()
	return goja.Undefined()
}

func syncZipArchive(rt *goja.Runtime, call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(rt.NewTypeError("data is required"))
	}
	entries, _, err := normalizeEntries(rt, call.Argument(0))
	if err != nil {
		panic(rt.NewTypeError(err.Error()))
	}
	archive, err := createArchive(entries)
	if err != nil {
		panic(rt.NewGoError(err))
	}
	return rt.ToValue(rt.NewArrayBuffer(archive))
}

func syncUnzipArchive(rt *goja.Runtime, call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(rt.NewTypeError("data is required"))
	}
	buf, err := bytesFromValue(rt, call.Argument(0))
	if err != nil {
		panic(rt.NewTypeError(err.Error()))
	}
	entries, err := openArchive(buf)
	if err != nil {
		panic(rt.NewGoError(err))
	}
	return entriesValue(rt, entries)
}

func normalizeEntries(rt *goja.Runtime, value goja.Value) ([]archiveEntry, int, error) {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil, 0, fmt.Errorf("data is required")
	}
	if obj := value.ToObject(rt); obj != nil && obj.ClassName() == "Array" {
		length := int(obj.Get("length").ToInteger())
		entries := make([]archiveEntry, 0, length)
		total := 0
		for i := 0; i < length; i++ {
			entry, err := normalizeEntry(rt, obj.Get(fmt.Sprintf("%d", i)), defaultEntryNameFor(i+1))
			if err != nil {
				return nil, 0, err
			}
			entries = append(entries, entry)
			total += len(entry.Data)
		}
		return entries, total, nil
	}
	entry, err := normalizeEntry(rt, value, defaultZipEntryName)
	if err != nil {
		return nil, 0, err
	}
	return []archiveEntry{entry}, len(entry.Data), nil
}

func normalizeEntry(rt *goja.Runtime, value goja.Value, fallbackName string) (archiveEntry, error) {
	entry := archiveEntry{Name: fallbackName, Method: stdzip.Deflate, Modified: time.Now().UTC()}
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return entry, fmt.Errorf("zip entry is required")
	}
	if data, err := bytesFromValue(rt, value); err == nil {
		entry.Data = data
		entry.Uncompressed = uint64(len(data))
		return entry, nil
	}
	obj := value.ToObject(rt)
	if obj == nil {
		return entry, fmt.Errorf("zip entry must be a Buffer, string, or object")
	}
	dataValue := obj.Get("data")
	if dataValue == nil || goja.IsUndefined(dataValue) || goja.IsNull(dataValue) {
		return entry, fmt.Errorf("zip entry data is required")
	}
	data, err := bytesFromValue(rt, dataValue)
	if err != nil {
		return entry, err
	}
	entry.Data = data
	entry.Uncompressed = uint64(len(data))
	if name := obj.Get("name"); name != nil && !goja.IsUndefined(name) && !goja.IsNull(name) {
		trimmed := strings.TrimSpace(name.String())
		if trimmed != "" {
			entry.Name = trimmed
		}
	}
	if comment := obj.Get("comment"); comment != nil && !goja.IsUndefined(comment) && !goja.IsNull(comment) {
		entry.Comment = comment.String()
	}
	if method := obj.Get("method"); method != nil && !goja.IsUndefined(method) && !goja.IsNull(method) {
		switch strings.ToLower(method.String()) {
		case "store", "stored":
			entry.Method = stdzip.Store
		case "deflate", "", "default":
			entry.Method = stdzip.Deflate
		default:
			return entry, fmt.Errorf("unsupported zip method: %s", method.String())
		}
	}
	return entry, nil
}

func defaultEntryNameFor(index int) string {
	if index <= 1 {
		return defaultZipEntryName
	}
	return fmt.Sprintf("entry-%d", index)
}

func createArchive(entries []archiveEntry) ([]byte, error) {
	buf := &bytes.Buffer{}
	writer := stdzip.NewWriter(buf)
	for i, entry := range entries {
		name := entry.Name
		if strings.TrimSpace(name) == "" {
			name = defaultEntryNameFor(i + 1)
		}
		hdr := &stdzip.FileHeader{
			Name:     name,
			Method:   entry.Method,
			Modified: entry.Modified,
			Comment:  entry.Comment,
		}
		zipWriter, err := writer.CreateHeader(hdr)
		if err != nil {
			writer.Close()
			return nil, err
		}
		if _, err := zipWriter.Write(entry.Data); err != nil {
			writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func openArchive(data []byte) ([]archiveEntry, error) {
	reader, err := stdzip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	entries := make([]archiveEntry, 0, len(reader.File))
	for _, file := range reader.File {
		entry := archiveEntry{
			Name:           file.Name,
			Comment:        file.Comment,
			Method:         file.Method,
			Modified:       file.Modified,
			CompressedSize: file.CompressedSize64,
			Uncompressed:   file.UncompressedSize64,
			IsDir:          file.FileInfo().IsDir(),
		}
		if !entry.IsDir {
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			entry.Data = content
			if entry.Uncompressed == 0 {
				entry.Uncompressed = uint64(len(content))
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func entryValue(rt *goja.Runtime, entry archiveEntry) goja.Value {
	obj := rt.NewObject()
	obj.Set("name", entry.Name)
	obj.Set("data", rt.NewArrayBuffer(entry.Data))
	obj.Set("comment", entry.Comment)
	obj.Set("method", entry.Method)
	obj.Set("compressedSize", entry.CompressedSize)
	obj.Set("size", entry.Uncompressed)
	obj.Set("isDir", entry.IsDir)
	obj.Set("modified", entry.Modified.Format(time.RFC3339Nano))
	return obj
}

func entriesValue(rt *goja.Runtime, entries []archiveEntry) goja.Value {
	arr := rt.NewArray(len(entries))
	for i, entry := range entries {
		arr.Set(fmt.Sprintf("%d", i), entryValue(rt, entry))
	}
	return arr
}

func bytesFromAny(rt *goja.Runtime, data interface{}) ([]byte, error) {
	if value, ok := data.(goja.Value); ok {
		return bytesFromValue(rt, value)
	}
	return bytesFromValue(rt, rt.ToValue(data))
}

func bytesFromValue(rt *goja.Runtime, value goja.Value) ([]byte, error) {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil, nil
	}
	if exp := value.Export(); exp != nil {
		switch v := exp.(type) {
		case []byte:
			return v, nil
		case string:
			return []byte(v), nil
		case goja.ArrayBuffer:
			return v.Bytes(), nil
		}
	}
	obj := value.ToObject(rt)
	if obj != nil {
		if obj.ClassName() == "Array" {
			length := int(obj.Get("length").ToInteger())
			buf := make([]byte, length)
			for i := 0; i < length; i++ {
				buf[i] = byte(obj.Get(fmt.Sprintf("%d", i)).ToInteger())
			}
			return buf, nil
		}
		if byteLength := obj.Get("byteLength"); byteLength != nil && !goja.IsUndefined(byteLength) && !goja.IsNull(byteLength) {
			if buffer := obj.Get("buffer"); buffer != nil && !goja.IsUndefined(buffer) && !goja.IsNull(buffer) {
				if ab, ok := buffer.Export().(goja.ArrayBuffer); ok {
					start := 0
					if byteOffset := obj.Get("byteOffset"); byteOffset != nil && !goja.IsUndefined(byteOffset) && !goja.IsNull(byteOffset) {
						start = int(byteOffset.ToInteger())
					}
					end := start + int(byteLength.ToInteger())
					bytes := ab.Bytes()
					if start >= 0 && end <= len(bytes) && start <= end {
						return append([]byte(nil), bytes[start:end]...), nil
					}
				}
			}
			if ab, ok := obj.Export().(goja.ArrayBuffer); ok {
				return append([]byte(nil), ab.Bytes()...), nil
			}
		}
		if buffer := obj.Get("buffer"); buffer != nil && !goja.IsUndefined(buffer) && !goja.IsNull(buffer) {
			if ab, ok := buffer.Export().(goja.ArrayBuffer); ok {
				return append([]byte(nil), ab.Bytes()...), nil
			}
		}
		if obj.ClassName() != "Object" {
			return []byte(value.String()), nil
		}
	}
	if obj != nil && obj.ClassName() == "Object" {
		return nil, fmt.Errorf("data must be a Buffer or string")
	}
	return []byte(value.String()), nil
}

func toGojaValue(rt *goja.Runtime, data interface{}) goja.Value {
	if value, ok := data.(goja.Value); ok {
		return value
	}
	return rt.ToValue(data)
}
