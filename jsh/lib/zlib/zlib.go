package zlib

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"

	"github.com/dop251/goja"
)

//go:embed zlib.js
var zlib_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"zlib.js": zlib_js,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)

	// Stream creation functions
	m.Set("createGzip", func() goja.Value {
		return exportZlibStream(rt, NewGzipStream(rt, false))
	})
	m.Set("createGunzip", func() goja.Value {
		return exportZlibStream(rt, NewGunzipStream(rt, false))
	})
	m.Set("createDeflate", func() goja.Value {
		return exportZlibStream(rt, NewDeflateStream(rt, false))
	})
	m.Set("createInflate", func() goja.Value {
		return exportZlibStream(rt, NewInflateStream(rt, false))
	})
	m.Set("createDeflateRaw", func() goja.Value {
		return exportZlibStream(rt, NewDeflateRawStream(rt, false))
	})
	m.Set("createInflateRaw", func() goja.Value {
		return exportZlibStream(rt, NewInflateRawStream(rt, false))
	})
	m.Set("createUnzip", func() goja.Value {
		return exportZlibStream(rt, NewUnzipStream(rt))
	})

	// Convenience methods (async with callback)
	m.Set("gzip", func(call goja.FunctionCall) goja.Value {
		return asyncCompress(rt, call, "gzip")
	})
	m.Set("gunzip", func(call goja.FunctionCall) goja.Value {
		return asyncDecompress(rt, call, "gunzip")
	})
	m.Set("deflate", func(call goja.FunctionCall) goja.Value {
		return asyncCompress(rt, call, "deflate")
	})
	m.Set("inflate", func(call goja.FunctionCall) goja.Value {
		return asyncDecompress(rt, call, "inflate")
	})
	m.Set("deflateRaw", func(call goja.FunctionCall) goja.Value {
		return asyncCompress(rt, call, "deflateRaw")
	})
	m.Set("inflateRaw", func(call goja.FunctionCall) goja.Value {
		return asyncDecompress(rt, call, "inflateRaw")
	})
	m.Set("unzip", func(call goja.FunctionCall) goja.Value {
		return asyncDecompress(rt, call, "unzip")
	})

	// Sync convenience methods
	m.Set("gzipSync", func(call goja.FunctionCall) goja.Value {
		return syncCompress(rt, call, "gzip")
	})
	m.Set("gunzipSync", func(call goja.FunctionCall) goja.Value {
		return syncDecompress(rt, call, "gunzip")
	})
	m.Set("deflateSync", func(call goja.FunctionCall) goja.Value {
		return syncCompress(rt, call, "deflate")
	})
	m.Set("inflateSync", func(call goja.FunctionCall) goja.Value {
		return syncDecompress(rt, call, "inflate")
	})
	m.Set("deflateRawSync", func(call goja.FunctionCall) goja.Value {
		return syncCompress(rt, call, "deflateRaw")
	})
	m.Set("inflateRawSync", func(call goja.FunctionCall) goja.Value {
		return syncDecompress(rt, call, "inflateRaw")
	})
	m.Set("unzipSync", func(call goja.FunctionCall) goja.Value {
		return syncDecompress(rt, call, "unzip")
	})

	// Constants
	constants := rt.NewObject()
	// Flush values
	constants.Set("Z_NO_FLUSH", 0)
	constants.Set("Z_PARTIAL_FLUSH", 1)
	constants.Set("Z_SYNC_FLUSH", 2)
	constants.Set("Z_FULL_FLUSH", 3)
	constants.Set("Z_FINISH", 4)
	constants.Set("Z_BLOCK", 5)

	// Return codes
	constants.Set("Z_OK", 0)
	constants.Set("Z_STREAM_END", 1)
	constants.Set("Z_NEED_DICT", 2)
	constants.Set("Z_ERRNO", -1)
	constants.Set("Z_STREAM_ERROR", -2)
	constants.Set("Z_DATA_ERROR", -3)
	constants.Set("Z_MEM_ERROR", -4)
	constants.Set("Z_BUF_ERROR", -5)
	constants.Set("Z_VERSION_ERROR", -6)

	// Compression levels
	constants.Set("Z_NO_COMPRESSION", 0)
	constants.Set("Z_BEST_SPEED", 1)
	constants.Set("Z_BEST_COMPRESSION", 9)
	constants.Set("Z_DEFAULT_COMPRESSION", -1)

	// Compression strategy
	constants.Set("Z_FILTERED", 1)
	constants.Set("Z_HUFFMAN_ONLY", 2)
	constants.Set("Z_RLE", 3)
	constants.Set("Z_FIXED", 4)
	constants.Set("Z_DEFAULT_STRATEGY", 0)

	m.Set("constants", constants)
}

type ZlibStreamWriter struct {
	*ZlibStream
}

func (z ZlibStreamWriter) Write(data []byte) (int, error) {
	return z.ZlibStream.Write(data)
}
func (z ZlibStreamWriter) Close() error {
	return z.ZlibStream.Close()
}

// exportZlibStream creates a JavaScript object with the stream methods
func exportZlibStream(rt *goja.Runtime, stream *ZlibStream) goja.Value {
	obj := rt.NewObject()
	stream.obj = obj
	stream.updateStats()

	// Export writer for direct access
	obj.Set("writer", ZlibStreamWriter{stream})

	// Export write method
	obj.Set("write", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(rt.NewTypeError("data is required"))
		}

		n, err := stream.Write(call.Argument(0))
		if err != nil {
			panic(rt.NewGoError(err))
		}

		return rt.ToValue(n > 0)
	})

	// Export end method
	obj.Set("end", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			if err := stream.End(call.Argument(0)); err != nil {
				panic(rt.NewGoError(err))
			}
		} else {
			if err := stream.End(); err != nil {
				panic(rt.NewGoError(err))
			}
		}

		return goja.Undefined()
	})

	// Export pipe method
	obj.Set("pipe", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(rt.NewTypeError("destination is required"))
		}

		dest := call.Argument(0)
		options := PipeOptions{End: true}
		if len(call.Arguments) > 1 {
			optArg := call.Argument(1)
			if !goja.IsUndefined(optArg) && !goja.IsNull(optArg) {
				if optObj := optArg.ToObject(rt); optObj != nil {
					endValue := optObj.Get("end")
					if endValue != nil && !goja.IsUndefined(endValue) && !goja.IsNull(endValue) {
						options.End = endValue.ToBoolean()
					}
				}
			}
		}

		result, err := stream.Pipe(dest, options)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(result)
	})

	// Export on method for event listeners
	obj.Set("on", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(rt.NewTypeError("event and callback are required"))
		}

		event := call.Argument(0).String()
		callback, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			panic(rt.NewTypeError("callback must be a function"))
		}

		stream.On(event, callback)
		return rt.ToValue(obj)
	})

	// Export flush method
	obj.Set("flush", func(call goja.FunctionCall) goja.Value {
		if err := stream.Flush(); err != nil {
			panic(rt.NewGoError(err))
		}
		return goja.Undefined()
	})

	// Export close method
	obj.Set("close", func(call goja.FunctionCall) goja.Value {
		if err := stream.Close(); err != nil {
			panic(rt.NewGoError(err))
		}
		return goja.Undefined()
	})

	return rt.ToValue(obj)
}

// ZlibStream represents a compression/decompression stream
type ZlibStream struct {
	rt              *goja.Runtime
	obj             *goja.Object
	buffer          *bytes.Buffer
	writer          io.WriteCloser
	reader          io.ReadCloser
	isCompress      bool
	streamType      string
	onDataCallback  goja.Callable
	onEndCallback   goja.Callable
	onErrorCallback goja.Callable
	bytesWritten    int64
	bytesRead       int64
	decompressPipeW *io.PipeWriter
	decompressOutCh chan []byte
	decompressErrCh chan error
	decompressDone  chan struct{}
	decompressInit  bool
}

type PipeOptions struct {
	End bool
}

func NewGzipStream(rt *goja.Runtime, _ bool) *ZlibStream {
	buf := &bytes.Buffer{}
	writer := gzip.NewWriter(buf)
	return &ZlibStream{
		rt:         rt,
		buffer:     buf,
		writer:     writer,
		isCompress: true,
		streamType: "gzip",
	}
}

func NewGunzipStream(rt *goja.Runtime, _ bool) *ZlibStream {
	return &ZlibStream{
		rt:         rt,
		isCompress: false,
		streamType: "gunzip",
	}
}

func NewDeflateStream(rt *goja.Runtime, _ bool) *ZlibStream {
	buf := &bytes.Buffer{}
	writer, _ := flate.NewWriter(buf, flate.DefaultCompression)
	return &ZlibStream{
		rt:         rt,
		buffer:     buf,
		writer:     writer,
		isCompress: true,
		streamType: "deflate",
	}
}

func NewInflateStream(rt *goja.Runtime, _ bool) *ZlibStream {
	return &ZlibStream{
		rt:         rt,
		isCompress: false,
		streamType: "inflate",
	}
}

func NewDeflateRawStream(rt *goja.Runtime, _ bool) *ZlibStream {
	buf := &bytes.Buffer{}
	writer, _ := flate.NewWriter(buf, flate.DefaultCompression)
	return &ZlibStream{
		rt:         rt,
		buffer:     buf,
		writer:     writer,
		isCompress: true,
		streamType: "deflateRaw",
	}
}

func NewInflateRawStream(rt *goja.Runtime, _ bool) *ZlibStream {
	return &ZlibStream{
		rt:         rt,
		isCompress: false,
		streamType: "inflateRaw",
	}
}

func NewUnzipStream(rt *goja.Runtime) *ZlibStream {
	return &ZlibStream{
		rt:         rt,
		isCompress: false,
		streamType: "unzip",
	}
}

//var _ io.WriteCloser = (*ZlibStream)(nil)

// Write writes data to the stream
func (z *ZlibStream) Write(data interface{}) (int, error) {
	buf, err := z.coerceBytes(data)
	if err != nil {
		return 0, err
	}

	if z.isCompress {
		if z.writer == nil {
			return 0, fmt.Errorf("writer not initialized")
		}
		n, err := z.writer.Write(buf)
		if err != nil {
			return n, err
		}
		z.bytesWritten += int64(n)
		z.updateStats()

		if z.onDataCallback != nil {
			z.emitCompressedAvailable()
		}

		return n, nil
	} else {
		if err := z.ensureDecompressPipeline(); err != nil {
			return 0, err
		}

		n, err := z.decompressPipeW.Write(buf)
		if err != nil {
			return n, err
		}
		z.bytesWritten += int64(n)
		z.updateStats()

		if err := z.drainDecompressedChunks(false); err != nil {
			if z.onErrorCallback != nil {
				errObj := z.rt.NewGoError(err)
				z.onErrorCallback(goja.Undefined(), errObj)
			}
			return n, err
		}

		return n, nil
	}
}

// End finalizes the stream
func (z *ZlibStream) End(data ...interface{}) error {
	if len(data) > 0 {
		_, err := z.Write(data[0])
		if err != nil {
			return err
		}
	}

	if z.isCompress && z.writer != nil {
		if err := z.writer.Close(); err != nil {
			return err
		}

		z.emitCompressedAvailable()

		// Emit end event
		if z.onEndCallback != nil {
			z.onEndCallback(goja.Undefined())
		}
	} else if !z.isCompress {
		if z.decompressPipeW != nil {
			if err := z.decompressPipeW.Close(); err != nil {
				if z.onErrorCallback != nil {
					errObj := z.rt.NewGoError(err)
					z.onErrorCallback(goja.Undefined(), errObj)
				}
				return err
			}
		}

		if z.decompressDone != nil {
			<-z.decompressDone
		}

		if err := z.drainDecompressedChunks(true); err != nil {
			if z.onErrorCallback != nil {
				errObj := z.rt.NewGoError(err)
				z.onErrorCallback(goja.Undefined(), errObj)
			}
			return err
		}

		// Emit end event
		if z.onEndCallback != nil {
			z.onEndCallback(goja.Undefined())
		}
	}

	return nil
}

func (z *ZlibStream) updateStats() {
	if z.obj == nil {
		return
	}
	z.obj.Set("bytesWritten", z.bytesWritten)
	z.obj.Set("bytesRead", z.bytesRead)
}

func (z *ZlibStream) emitCompressedAvailable() {
	if z.onDataCallback == nil || z.buffer == nil || z.buffer.Len() == 0 {
		return
	}

	chunk := make([]byte, z.buffer.Len())
	copy(chunk, z.buffer.Next(z.buffer.Len()))
	z.bytesRead += int64(len(chunk))
	z.updateStats()
	bufObj := z.rt.NewArrayBuffer(chunk)
	z.onDataCallback(goja.Undefined(), z.rt.ToValue(bufObj))
}

func (z *ZlibStream) coerceBytes(data interface{}) ([]byte, error) {
	switch v := data.(type) {
	case nil:
		return nil, nil
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case goja.ArrayBuffer:
		return v.Bytes(), nil
	case goja.Value:
		return bytesFromValue(z.rt, v)
	default:
		return nil, fmt.Errorf("data must be a Buffer or string")
	}
}

func (z *ZlibStream) ensureDecompressPipeline() error {
	if z.decompressInit {
		return nil
	}

	pr, pw := io.Pipe()
	z.decompressPipeW = pw
	z.decompressOutCh = make(chan []byte, 16)
	z.decompressErrCh = make(chan error, 1)
	z.decompressDone = make(chan struct{})
	z.decompressInit = true

	go func() {
		defer close(z.decompressDone)
		defer close(z.decompressOutCh)

		rdr, err := newDecompressReader(pr, z.streamType)
		if err != nil {
			select {
			case z.decompressErrCh <- err:
			default:
			}
			return
		}

		defer rdr.Close()

		readBuf := make([]byte, 32*1024)
		for {
			n, readErr := rdr.Read(readBuf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, readBuf[:n])
				z.decompressOutCh <- chunk
			}

			if readErr != nil {
				if readErr != io.EOF {
					select {
					case z.decompressErrCh <- readErr:
					default:
					}
				}
				return
			}
		}
	}()

	return nil
}

func (z *ZlibStream) drainDecompressedChunks(waitAll bool) error {
	if z.decompressOutCh == nil {
		return nil
	}

	drainChunk := func(chunk []byte) {
		if len(chunk) == 0 || z.onDataCallback == nil {
			return
		}
		z.bytesRead += int64(len(chunk))
		z.updateStats()
		bufObj := z.rt.NewArrayBuffer(chunk)
		z.onDataCallback(goja.Undefined(), z.rt.ToValue(bufObj))
	}

	if waitAll {
		for chunk := range z.decompressOutCh {
			drainChunk(chunk)
		}
	} else {
		for {
			select {
			case chunk, ok := <-z.decompressOutCh:
				if !ok {
					goto CHECK_ERR
				}
				drainChunk(chunk)
			default:
				goto CHECK_ERR
			}
		}
	}

CHECK_ERR:
	select {
	case err := <-z.decompressErrCh:
		return err
	default:
		return nil
	}
}

// Pipe connects this stream to another stream
func (z *ZlibStream) Pipe(dest interface{}, options ...PipeOptions) (interface{}, error) {
	pipeOptions := PipeOptions{End: true}
	if len(options) > 0 {
		pipeOptions = options[0]
	}

	var writer io.Writer
	var jsDestObj *goja.Object
	var jsWrite goja.Callable
	var jsEnd goja.Callable

	if destVal, ok := dest.(goja.Value); ok {
		if goja.IsUndefined(destVal) || goja.IsNull(destVal) {
			return nil, fmt.Errorf("destination is required")
		}

		obj := destVal.ToObject(z.rt)
		if obj == nil {
			return nil, fmt.Errorf("destination is required")
		}

		if endFn, ok := goja.AssertFunction(obj.Get("end")); ok {
			jsDestObj = obj
			jsEnd = endFn
		}

		if writerVal := obj.Get("writer"); writerVal != nil && !goja.IsUndefined(writerVal) && !goja.IsNull(writerVal) {
			exported := writerVal.Export()
			if w, ok := exported.(io.WriteCloser); ok {
				writer = w
			} else if w, ok := exported.(io.Writer); ok {
				writer = w
			} else {
				return nil, fmt.Errorf("map[\"writer\"] must be io.Writer or io.WriteCloser")
			}
		}

		if writer == nil {
			if writeFn, ok := goja.AssertFunction(obj.Get("write")); ok {
				jsDestObj = obj
				jsWrite = writeFn
			} else if obj.ClassName() == "Object" {
				return nil, fmt.Errorf("map must contain \"writer\" key")
			} else {
				return nil, fmt.Errorf("dest must be io.Writer, io.WriteCloser, JS writable stream, or map with \"writer\" key")
			}
		}
	} else {
		// Check if dest is a map with "writer" key
		if destMap, ok := dest.(map[string]any); ok {
			if writerVal, exists := destMap["writer"]; exists {
				// Check if the writer value implements io.Writer or io.WriteCloser
				if w, ok := writerVal.(io.WriteCloser); ok {
					writer = w
				} else if w, ok := writerVal.(io.Writer); ok {
					writer = w
				} else {
					return nil, fmt.Errorf("map[\"writer\"] must be io.Writer or io.WriteCloser")
				}
			} else {
				return nil, fmt.Errorf("map must contain \"writer\" key")
			}
		} else if w, ok := dest.(io.WriteCloser); ok {
			// dest is directly an io.WriteCloser
			writer = w
		} else if w, ok := dest.(io.Writer); ok {
			// dest is directly an io.Writer
			writer = w
		} else {
			return nil, fmt.Errorf("dest must be io.Writer, io.WriteCloser, JS writable stream, or map with \"writer\" key")
		}
	}

	// Set up piping by registering a data callback
	dataCallback := z.rt.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			data := call.Arguments[0].Export()
			var bytes []byte

			// Convert data to bytes
			switch v := data.(type) {
			case []byte:
				bytes = v
			case string:
				bytes = []byte(v)
			case goja.ArrayBuffer:
				bytes = v.Bytes()
			default:
				// Try to get bytes from the value
				if ab := call.Arguments[0].ToObject(z.rt); ab != nil {
					if buffer := ab.Get("buffer"); buffer != nil {
						if ab, ok := buffer.Export().(goja.ArrayBuffer); ok {
							bytes = ab.Bytes()
						}
					}
				}
			}

			// Write to destination
			if len(bytes) > 0 {
				var err error
				if writer != nil {
					_, err = writer.Write(bytes)
				} else if jsWrite != nil {
					chunk := z.rt.ToValue(z.rt.NewArrayBuffer(bytes))
					if bufferCtor := z.rt.Get("Buffer"); bufferCtor != nil && !goja.IsUndefined(bufferCtor) && !goja.IsNull(bufferCtor) {
						if bufferObj := bufferCtor.ToObject(z.rt); bufferObj != nil {
							if fromFn, ok := goja.AssertFunction(bufferObj.Get("from")); ok {
								if bufferValue, fromErr := fromFn(bufferCtor, chunk); fromErr == nil {
									chunk = bufferValue
								}
							}
						}
					}
					_, err = jsWrite(jsDestObj, chunk)
				}

				if err != nil {
					// Emit error event if write fails
					if z.onErrorCallback != nil {
						z.rt.Interrupt(func() {
							z.onErrorCallback(goja.Undefined(), z.rt.ToValue(err))
						})
					}
				}
			}
		}
		return goja.Undefined()
	})

	// Assert as callable function
	callback, ok := goja.AssertFunction(dataCallback)
	if !ok {
		return nil, fmt.Errorf("failed to create pipe callback")
	}

	z.On("data", callback)

	if pipeOptions.End && jsEnd != nil {
		endCallback := z.rt.ToValue(func(call goja.FunctionCall) goja.Value {
			if _, err := jsEnd(jsDestObj); err != nil {
				if z.onErrorCallback != nil {
					z.rt.Interrupt(func() {
						z.onErrorCallback(goja.Undefined(), z.rt.ToValue(err))
					})
				}
			}
			return goja.Undefined()
		})

		endFn, ok := goja.AssertFunction(endCallback)
		if !ok {
			return nil, fmt.Errorf("failed to create pipe end callback")
		}
		z.On("end", endFn)
	}

	return dest, nil
}

// On registers event listeners
func (z *ZlibStream) On(event string, callback goja.Callable) *ZlibStream {
	switch event {
	case "data":
		z.onDataCallback = callback
	case "end":
		z.onEndCallback = callback
	case "error":
		z.onErrorCallback = callback
	}
	return z
}

// Flush flushes pending data
func (z *ZlibStream) Flush() error {
	if z.isCompress && z.writer != nil {
		if flusher, ok := z.writer.(interface{ Flush() error }); ok {
			return flusher.Flush()
		}
	}
	return nil
}

// Close closes the stream
func (z *ZlibStream) Close() error {
	if z.writer != nil {
		return z.writer.Close()
	}
	if z.reader != nil {
		return z.reader.Close()
	}
	return nil
}

// Async compression function
func asyncCompress(rt *goja.Runtime, call goja.FunctionCall, method string) goja.Value {
	if len(call.Arguments) < 2 {
		panic(rt.NewTypeError("callback is required"))
	}

	data := call.Argument(0)
	callback, ok := goja.AssertFunction(call.Argument(len(call.Arguments) - 1))
	if !ok {
		panic(rt.NewTypeError("last argument must be a callback function"))
	}

	// Convert data to bytes
	buf, err := bytesFromValue(rt, data)
	if err != nil {
		panic(rt.NewTypeError(err.Error()))
	}

	// Perform compression in background
	go func() {
		result, err := compress(buf, method)

		// Schedule callback on the event loop
		rt.Interrupt(func() {
			if err != nil {
				errObj := rt.NewGoError(err)
				callback(goja.Undefined(), errObj, goja.Null())
			} else {
				bufObj := rt.NewArrayBuffer(result)
				callback(goja.Undefined(), goja.Null(), rt.ToValue(bufObj))
			}
		})
	}()

	return goja.Undefined()
}

// Async decompression function
func asyncDecompress(rt *goja.Runtime, call goja.FunctionCall, method string) goja.Value {
	if len(call.Arguments) < 2 {
		panic(rt.NewTypeError("callback is required"))
	}

	data := call.Argument(0)
	callback, ok := goja.AssertFunction(call.Argument(len(call.Arguments) - 1))
	if !ok {
		panic(rt.NewTypeError("last argument must be a callback function"))
	}

	// Convert data to bytes
	buf, err := bytesFromValue(rt, data)
	if err != nil {
		panic(rt.NewTypeError(err.Error()))
	}

	// Perform decompression in background
	go func() {
		result, err := decompress(buf, method)

		// Schedule callback on the event loop
		rt.Interrupt(func() {
			if err != nil {
				errObj := rt.NewGoError(err)
				callback(goja.Undefined(), errObj, goja.Null())
			} else {
				bufObj := rt.NewArrayBuffer(result)
				callback(goja.Undefined(), goja.Null(), rt.ToValue(bufObj))
			}
		})
	}()

	return goja.Undefined()
}

// Sync compression function
func syncCompress(rt *goja.Runtime, call goja.FunctionCall, method string) goja.Value {
	if len(call.Arguments) < 1 {
		panic(rt.NewTypeError("data is required"))
	}

	data := call.Argument(0)

	// Convert data to bytes
	buf, err := bytesFromValue(rt, data)
	if err != nil {
		panic(rt.NewTypeError(err.Error()))
	}

	result, err := compress(buf, method)
	if err != nil {
		panic(rt.NewGoError(err))
	}

	return rt.ToValue(rt.NewArrayBuffer(result))
}

// Sync decompression function
func syncDecompress(rt *goja.Runtime, call goja.FunctionCall, method string) goja.Value {
	if len(call.Arguments) < 1 {
		panic(rt.NewTypeError("data is required"))
	}

	data := call.Argument(0)

	// Convert data to bytes
	buf, err := bytesFromValue(rt, data)
	if err != nil {
		panic(rt.NewTypeError(err.Error()))
	}

	result, err := decompress(buf, method)
	if err != nil {
		panic(rt.NewGoError(err))
	}

	return rt.ToValue(rt.NewArrayBuffer(result))
}

// compress compresses data based on the method
func compress(data []byte, method string) ([]byte, error) {
	buf := &bytes.Buffer{}
	writer, err := newCompressWriter(buf, method)
	if err != nil {
		return nil, err
	}

	_, err = writer.Write(data)
	if err != nil {
		writer.Close()
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompress decompresses data based on the method
func decompress(data []byte, method string) ([]byte, error) {
	reader, err := newDecompressReader(bytes.NewBuffer(data), method)
	if err != nil {
		return nil, err
	}

	defer reader.Close()

	result := &bytes.Buffer{}
	_, err = io.Copy(result, reader)
	if err != nil {
		return nil, err
	}

	return result.Bytes(), nil
}

func newCompressWriter(w io.Writer, method string) (io.WriteCloser, error) {
	switch method {
	case "gzip":
		return gzip.NewWriter(w), nil
	case "deflate", "deflateRaw":
		return flate.NewWriter(w, flate.DefaultCompression)
	default:
		return nil, fmt.Errorf("unsupported compression method: %s", method)
	}
}

func newDecompressReader(r io.Reader, method string) (io.ReadCloser, error) {
	switch method {
	case "gunzip", "unzip":
		return gzip.NewReader(r)
	case "inflate", "inflateRaw":
		return flate.NewReader(r), nil
	default:
		return nil, fmt.Errorf("unsupported decompression method: %s", method)
	}
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
		if byteLength := obj.Get("byteLength"); byteLength != nil && !goja.IsUndefined(byteLength) && !goja.IsNull(byteLength) {
			if ab, ok := obj.Export().(goja.ArrayBuffer); ok {
				return ab.Bytes(), nil
			}
		}
		if buffer := obj.Get("buffer"); buffer != nil && !goja.IsUndefined(buffer) && !goja.IsNull(buffer) {
			if ab, ok := buffer.Export().(goja.ArrayBuffer); ok {
				return ab.Bytes(), nil
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
