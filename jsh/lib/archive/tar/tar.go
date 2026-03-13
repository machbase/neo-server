package tar

import (
	stdtar "archive/tar"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dop251/goja"
)

//go:embed tar.js
var tarJS []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"archive/tar.js": tarJS,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	m := module.Get("exports").(*goja.Object)

	m.Set("createTar", func() goja.Value {
		return exportArchiveWriter(rt, newArchiveWriter(rt))
	})
	m.Set("createUntar", func() goja.Value {
		return exportArchiveReader(rt, newArchiveReader(rt))
	})
	m.Set("tar", func(call goja.FunctionCall) goja.Value {
		return asyncTarArchive(rt, call)
	})
	m.Set("untar", func(call goja.FunctionCall) goja.Value {
		return asyncUntarArchive(rt, call)
	})
	m.Set("tarSync", func(call goja.FunctionCall) goja.Value {
		return syncTarArchive(rt, call)
	})
	m.Set("untarSync", func(call goja.FunctionCall) goja.Value {
		return syncUntarArchive(rt, call)
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
	Name     string
	Data     []byte
	Mode     int64
	Modified time.Time
	Size     int64
	Typeflag byte
	Linkname string
	IsDir    bool
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

const defaultTarEntryName = "data"

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

func asyncTarArchive(rt *goja.Runtime, call goja.FunctionCall) goja.Value {
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
		archive, tarErr := createArchive(entries)
		rt.Interrupt(func() {
			if tarErr != nil {
				callback(goja.Undefined(), rt.NewGoError(tarErr), goja.Null())
			} else {
				callback(goja.Undefined(), goja.Null(), rt.ToValue(rt.NewArrayBuffer(archive)))
			}
		})
	}()
	return goja.Undefined()
}

func asyncUntarArchive(rt *goja.Runtime, call goja.FunctionCall) goja.Value {
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
		entries, untarErr := openArchive(buf)
		rt.Interrupt(func() {
			if untarErr != nil {
				callback(goja.Undefined(), rt.NewGoError(untarErr), goja.Null())
			} else {
				callback(goja.Undefined(), goja.Null(), entriesValue(rt, entries))
			}
		})
	}()
	return goja.Undefined()
}

func syncTarArchive(rt *goja.Runtime, call goja.FunctionCall) goja.Value {
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

func syncUntarArchive(rt *goja.Runtime, call goja.FunctionCall) goja.Value {
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
	entry, err := normalizeEntry(rt, value, defaultTarEntryName)
	if err != nil {
		return nil, 0, err
	}
	return []archiveEntry{entry}, len(entry.Data), nil
}

func normalizeEntry(rt *goja.Runtime, value goja.Value, fallbackName string) (archiveEntry, error) {
	entry := archiveEntry{Name: fallbackName, Mode: 0644, Modified: time.Now().UTC(), Typeflag: stdtar.TypeReg}
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return entry, fmt.Errorf("tar entry is required")
	}
	if data, err := bytesFromValue(rt, value); err == nil {
		entry.Data = data
		entry.Size = int64(len(data))
		return entry, nil
	}
	obj := value.ToObject(rt)
	if obj == nil {
		return entry, fmt.Errorf("tar entry must be a Buffer, string, or object")
	}
	if name := obj.Get("name"); name != nil && !goja.IsUndefined(name) && !goja.IsNull(name) {
		trimmed := strings.TrimSpace(name.String())
		if trimmed != "" {
			entry.Name = trimmed
		}
	}
	if mode := obj.Get("mode"); mode != nil && !goja.IsUndefined(mode) && !goja.IsNull(mode) {
		entry.Mode = mode.ToInteger()
	}
	if modified := obj.Get("modified"); modified != nil && !goja.IsUndefined(modified) && !goja.IsNull(modified) {
		if parsed, err := time.Parse(time.RFC3339Nano, modified.String()); err == nil {
			entry.Modified = parsed
		}
	}
	if linkname := obj.Get("linkname"); linkname != nil && !goja.IsUndefined(linkname) && !goja.IsNull(linkname) {
		entry.Linkname = linkname.String()
	}
	if typeflag := obj.Get("typeflag"); typeflag != nil && !goja.IsUndefined(typeflag) && !goja.IsNull(typeflag) {
		entry.Typeflag = byte(typeflag.ToInteger())
	}
	if typeName := obj.Get("type"); typeName != nil && !goja.IsUndefined(typeName) && !goja.IsNull(typeName) {
		resolved, err := resolveTypeflag(typeName.String())
		if err != nil {
			return entry, err
		}
		entry.Typeflag = resolved
	}
	if isDir := obj.Get("isDir"); isDir != nil && !goja.IsUndefined(isDir) && !goja.IsNull(isDir) {
		entry.IsDir = isDir.ToBoolean()
	}
	dataValue := obj.Get("data")
	if dataValue != nil && !goja.IsUndefined(dataValue) && !goja.IsNull(dataValue) {
		data, err := bytesFromValue(rt, dataValue)
		if err != nil {
			return entry, err
		}
		entry.Data = data
	}
	if entry.IsDir || entry.Typeflag == stdtar.TypeDir {
		entry.IsDir = true
		entry.Typeflag = stdtar.TypeDir
		entry.Data = nil
		entry.Size = 0
		if !strings.HasSuffix(entry.Name, "/") {
			entry.Name += "/"
		}
		return entry, nil
	}
	if entry.Linkname != "" && (entry.Typeflag == stdtar.TypeSymlink || entry.Typeflag == stdtar.TypeLink) {
		entry.Size = 0
		entry.Data = nil
		return entry, nil
	}
	if len(entry.Data) == 0 && entry.Linkname == "" && entry.Typeflag != stdtar.TypeReg && entry.Typeflag != stdtar.TypeRegA {
		entry.Size = 0
		return entry, nil
	}
	if dataValue == nil || goja.IsUndefined(dataValue) || goja.IsNull(dataValue) {
		return entry, fmt.Errorf("tar entry data is required")
	}
	entry.Size = int64(len(entry.Data))
	if entry.Typeflag == 0 {
		entry.Typeflag = stdtar.TypeReg
	}
	return entry, nil
}

func resolveTypeflag(name string) (byte, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "file", "reg", "regular":
		return stdtar.TypeReg, nil
	case "dir", "directory":
		return stdtar.TypeDir, nil
	case "symlink", "symboliclink":
		return stdtar.TypeSymlink, nil
	case "link", "hardlink":
		return stdtar.TypeLink, nil
	default:
		return 0, fmt.Errorf("unsupported tar type: %s", name)
	}
}

func defaultEntryNameFor(index int) string {
	if index <= 1 {
		return defaultTarEntryName
	}
	return fmt.Sprintf("entry-%d", index)
}

func createArchive(entries []archiveEntry) ([]byte, error) {
	buf := &bytes.Buffer{}
	writer := stdtar.NewWriter(buf)
	for i, entry := range entries {
		name := entry.Name
		if strings.TrimSpace(name) == "" {
			name = defaultEntryNameFor(i + 1)
		}
		if entry.IsDir && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		typeflag := entry.Typeflag
		if typeflag == 0 {
			if entry.IsDir {
				typeflag = stdtar.TypeDir
			} else {
				typeflag = stdtar.TypeReg
			}
		}
		size := int64(len(entry.Data))
		if entry.IsDir || typeflag == stdtar.TypeDir {
			size = 0
		}
		hdr := &stdtar.Header{
			Name:     name,
			Mode:     entry.Mode,
			ModTime:  entry.Modified,
			Size:     size,
			Typeflag: typeflag,
			Linkname: entry.Linkname,
		}
		if err := writer.WriteHeader(hdr); err != nil {
			writer.Close()
			return nil, err
		}
		if size > 0 {
			if _, err := writer.Write(entry.Data); err != nil {
				writer.Close()
				return nil, err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func openArchive(data []byte) ([]archiveEntry, error) {
	reader := stdtar.NewReader(bytes.NewReader(data))
	entries := make([]archiveEntry, 0)
	for {
		hdr, err := reader.Next()
		if err == io.EOF {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
		entry := archiveEntry{
			Name:     hdr.Name,
			Mode:     hdr.Mode,
			Modified: hdr.ModTime,
			Size:     hdr.Size,
			Typeflag: hdr.Typeflag,
			Linkname: hdr.Linkname,
			IsDir:    hdr.FileInfo().IsDir() || hdr.Typeflag == stdtar.TypeDir,
		}
		if !entry.IsDir && hdr.Size > 0 {
			content, err := io.ReadAll(reader)
			if err != nil {
				return nil, err
			}
			entry.Data = content
			entry.Size = int64(len(content))
		}
		entries = append(entries, entry)
	}
}

func entryValue(rt *goja.Runtime, entry archiveEntry) goja.Value {
	obj := rt.NewObject()
	obj.Set("name", entry.Name)
	obj.Set("data", rt.NewArrayBuffer(entry.Data))
	obj.Set("mode", entry.Mode)
	obj.Set("size", entry.Size)
	obj.Set("isDir", entry.IsDir)
	obj.Set("modified", entry.Modified.Format(time.RFC3339Nano))
	obj.Set("typeflag", int64(entry.Typeflag))
	obj.Set("type", typeName(entry.Typeflag))
	obj.Set("linkname", entry.Linkname)
	return obj
}

func entriesValue(rt *goja.Runtime, entries []archiveEntry) goja.Value {
	arr := rt.NewArray(len(entries))
	for i, entry := range entries {
		arr.Set(fmt.Sprintf("%d", i), entryValue(rt, entry))
	}
	return arr
}

func typeName(typeflag byte) string {
	switch typeflag {
	case stdtar.TypeDir:
		return "dir"
	case stdtar.TypeSymlink:
		return "symlink"
	case stdtar.TypeLink:
		return "link"
	default:
		return "file"
	}
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
