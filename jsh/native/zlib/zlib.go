package zlib

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/dop251/goja"
)

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

	// Export writer for direct access
	obj.Set("writer", ZlibStreamWriter{stream})

	// Export write method
	obj.Set("write", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(rt.NewTypeError("data is required"))
		}

		data := call.Argument(0)
		var buf []byte

		if exp := data.Export(); exp != nil {
			switch v := exp.(type) {
			case []byte:
				buf = v
			case string:
				buf = []byte(v)
			default:
				// Try ArrayBuffer
				if ab, ok := data.Export().(goja.ArrayBuffer); ok {
					buf = ab.Bytes()
				} else {
					buf = []byte(data.String())
				}
			}
		} else {
			buf = []byte(data.String())
		}

		n, err := stream.Write(buf)
		if err != nil {
			panic(rt.NewGoError(err))
		}

		return rt.ToValue(n > 0)
	})

	// Export end method
	obj.Set("end", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			data := call.Argument(0)
			var buf []byte

			if exp := data.Export(); exp != nil {
				switch v := exp.(type) {
				case []byte:
					buf = v
				case string:
					buf = []byte(v)
				default:
					buf = []byte(data.String())
				}
			} else {
				buf = []byte(data.String())
			}

			if err := stream.End(buf); err != nil {
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
		result, err := stream.Pipe(dest.Export())
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
	buffer          *bytes.Buffer
	writer          io.WriteCloser
	reader          io.ReadCloser
	isCompress      bool
	streamType      string
	onDataCallback  goja.Callable
	onEndCallback   goja.Callable
	onErrorCallback goja.Callable
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
	var buf []byte

	switch v := data.(type) {
	case []byte:
		buf = v
	case string:
		buf = []byte(v)
	case goja.ArrayBuffer:
		buf = v.Bytes()
	default:
		if obj, ok := data.(goja.Value); ok {
			if exp := obj.Export(); exp != nil {
				if b, ok := exp.([]byte); ok {
					buf = b
				}
			}
		}
	}

	if z.isCompress {
		if z.writer == nil {
			return 0, fmt.Errorf("writer not initialized")
		}
		return z.writer.Write(buf)
	} else {
		// For decompression, we need to handle data differently
		if z.buffer == nil {
			z.buffer = &bytes.Buffer{}
		}
		return z.buffer.Write(buf)
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

		// Emit data event
		if z.onDataCallback != nil {
			result := z.buffer.Bytes()
			bufObj := z.rt.NewArrayBuffer(result)
			z.onDataCallback(goja.Undefined(), z.rt.ToValue(bufObj))
		}

		// Emit end event
		if z.onEndCallback != nil {
			z.onEndCallback(goja.Undefined())
		}
	} else if !z.isCompress && z.buffer != nil {
		// Decompress the data
		result, err := z.decompress(z.buffer.Bytes())
		if err != nil {
			if z.onErrorCallback != nil {
				errObj := z.rt.NewGoError(err)
				z.onErrorCallback(goja.Undefined(), errObj)
			}
			return err
		}

		// Emit data event
		if z.onDataCallback != nil {
			bufObj := z.rt.NewArrayBuffer(result)
			z.onDataCallback(goja.Undefined(), z.rt.ToValue(bufObj))
		}

		// Emit end event
		if z.onEndCallback != nil {
			z.onEndCallback(goja.Undefined())
		}
	}

	return nil
}

func (z *ZlibStream) decompress(data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(data)
	var reader io.ReadCloser
	var err error

	switch z.streamType {
	case "gunzip", "unzip":
		reader, err = gzip.NewReader(buf)
	case "inflate", "inflateRaw":
		reader = flate.NewReader(buf)
	default:
		return nil, fmt.Errorf("unsupported decompression type: %s", z.streamType)
	}

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

// Pipe connects this stream to another stream
func (z *ZlibStream) Pipe(dest interface{}) (interface{}, error) {
	var writer io.Writer

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
		return nil, fmt.Errorf("dest must be io.Writer, io.WriteCloser, or map with \"writer\" key")
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
				if _, err := writer.Write(bytes); err != nil {
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
	var buf []byte
	if exp := data.Export(); exp != nil {
		switch v := exp.(type) {
		case []byte:
			buf = v
		case string:
			buf = []byte(v)
		default:
			panic(rt.NewTypeError("data must be a Buffer or string"))
		}
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
	var buf []byte
	if exp := data.Export(); exp != nil {
		switch v := exp.(type) {
		case []byte:
			buf = v
		case string:
			buf = []byte(v)
		default:
			panic(rt.NewTypeError("data must be a Buffer or string"))
		}
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
	var buf []byte
	exp := data.Export()
	if exp != nil {
		switch v := exp.(type) {
		case []byte:
			buf = v
		case string:
			buf = []byte(v)
		default:
			// Try to convert to string
			buf = []byte(data.String())
		}
	} else {
		// Try as string
		buf = []byte(data.String())
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
	var buf []byte
	exp := data.Export()
	if exp != nil {
		switch v := exp.(type) {
		case []byte:
			buf = v
		case string:
			buf = []byte(v)
		case goja.ArrayBuffer:
			buf = v.Bytes()
		default:
			// Try ArrayBuffer through reflection
			if obj, ok := exp.(goja.Value); ok {
				if ab := obj.ToObject(rt); ab != nil {
					// Check if it's an ArrayBuffer by trying to access byteLength
					if byteLength := ab.Get("byteLength"); byteLength != nil && !goja.IsUndefined(byteLength) {
						// It's likely an ArrayBuffer, try to get bytes
						if bytes := ab.Export(); bytes != nil {
							if b, ok := bytes.([]byte); ok {
								buf = b
							} else if ab, ok := bytes.(goja.ArrayBuffer); ok {
								buf = ab.Bytes()
							} else {
								buf = []byte(data.String())
							}
						} else {
							buf = []byte(data.String())
						}
					} else {
						buf = []byte(data.String())
					}
				} else {
					buf = []byte(data.String())
				}
			} else {
				buf = []byte(data.String())
			}
		}
	} else {
		// Try as string
		buf = []byte(data.String())
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
	var writer io.WriteCloser
	var err error

	switch method {
	case "gzip":
		writer = gzip.NewWriter(buf)
	case "deflate", "deflateRaw":
		writer, err = flate.NewWriter(buf, flate.DefaultCompression)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported compression method: %s", method)
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
	buf := bytes.NewBuffer(data)
	var reader io.ReadCloser
	var err error

	switch method {
	case "gunzip", "unzip":
		reader, err = gzip.NewReader(buf)
		if err != nil {
			return nil, err
		}
	case "inflate", "inflateRaw":
		reader = flate.NewReader(buf)
	default:
		return nil, fmt.Errorf("unsupported decompression method: %s", method)
	}

	defer reader.Close()

	result := &bytes.Buffer{}
	_, err = io.Copy(result, reader)
	if err != nil {
		return nil, err
	}

	return result.Bytes(), nil
}
