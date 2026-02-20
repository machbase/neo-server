package stream

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/engine"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("NewReadable", NewReadable)
	m.Set("NewWritable", NewWritable)
	m.Set("NewDuplex", NewDuplex)
	m.Set("NewPassThrough", NewPassThrough)
}

// Readable wraps io.Reader to JavaScript
func NewReadable(obj *goja.Object, reader io.Reader, dispatch engine.EventDispatchFunc) *Readable {
	return &Readable{
		obj:    obj,
		reader: reader,
		emit: func(event string, data any) {
			dispatch(obj, event, data)
		},
		closed: false,
	}
}

type Readable struct {
	obj    *goja.Object
	reader io.Reader
	emit   func(event string, data any)
	closed bool
	mu     sync.Mutex
}

func (r *Readable) Read(size int) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, fmt.Errorf("stream is closed")
	}

	if size <= 0 {
		size = 8192 // default chunk size
	}

	buf := make([]byte, size)
	n, err := r.reader.Read(buf)
	if err != nil {
		if err == io.EOF {
			r.emit("end", nil)
			return buf[:n], err
		}
		r.emit("error", err)
		return nil, err
	}

	if n > 0 {
		r.emit("data", buf[:n])
	}

	return buf[:n], nil
}

func (r *Readable) ReadString(size int, encoding string) (string, error) {
	data, err := r.Read(size)
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(data), err
}

func (r *Readable) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	r.emit("close", nil)

	if closer, ok := r.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (r *Readable) IsClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

func (r *Readable) Pause() {
	// Emit pause event for JavaScript side to handle
	r.emit("pause", nil)
}

func (r *Readable) Resume() {
	// Emit resume event for JavaScript side to handle
	r.emit("resume", nil)
}

// Writable wraps io.Writer to JavaScript
func NewWritable(obj *goja.Object, writer io.Writer, dispatch engine.EventDispatchFunc) *Writable {
	return &Writable{
		obj:    obj,
		writer: writer,
		emit: func(event string, data any) {
			dispatch(obj, event, data)
		},
		closed: false,
	}
}

type Writable struct {
	obj    *goja.Object
	writer io.Writer
	emit   func(event string, data any)
	closed bool
	mu     sync.Mutex
}

func (w *Writable) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, fmt.Errorf("stream is closed")
	}

	n, err := w.writer.Write(data)
	if err != nil {
		w.emit("error", err)
		return n, err
	}

	return n, nil
}

func (w *Writable) WriteString(s string, encoding string) (int, error) {
	return w.Write([]byte(s))
}

func (w *Writable) End(data []byte) error {
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return w.Close()
}

func (w *Writable) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true
	w.emit("finish", nil)
	w.emit("close", nil)

	if closer, ok := w.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (w *Writable) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

// Duplex combines Reader and Writer
func NewDuplex(obj *goja.Object, reader io.Reader, writer io.Writer, dispatch engine.EventDispatchFunc) *Duplex {
	return &Duplex{
		obj:    obj,
		reader: reader,
		writer: writer,
		emit: func(event string, data any) {
			dispatch(obj, event, data)
		},
		closed: false,
	}
}

type Duplex struct {
	obj    *goja.Object
	reader io.Reader
	writer io.Writer
	emit   func(event string, data any)
	closed bool
	mu     sync.Mutex
}

func (d *Duplex) Read(size int) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil, fmt.Errorf("stream is closed")
	}

	if size <= 0 {
		size = 8192
	}

	buf := make([]byte, size)
	n, err := d.reader.Read(buf)
	if err != nil {
		if err == io.EOF {
			d.emit("end", nil)
			return buf[:n], err
		}
		d.emit("error", err)
		return nil, err
	}

	if n > 0 {
		d.emit("data", buf[:n])
	}

	return buf[:n], nil
}

func (d *Duplex) ReadString(size int, encoding string) (string, error) {
	data, err := d.Read(size)
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(data), err
}

func (d *Duplex) Write(data []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return 0, fmt.Errorf("stream is closed")
	}

	n, err := d.writer.Write(data)
	if err != nil {
		d.emit("error", err)
		return n, err
	}

	return n, nil
}

func (d *Duplex) WriteString(s string, encoding string) (int, error) {
	return d.Write([]byte(s))
}

func (d *Duplex) End(data []byte) error {
	if len(data) > 0 {
		if _, err := d.Write(data); err != nil {
			return err
		}
	}
	return d.Close()
}

func (d *Duplex) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil
	}

	d.closed = true
	d.emit("finish", nil)
	d.emit("close", nil)

	if closer, ok := d.reader.(io.Closer); ok {
		closer.Close()
	}
	if closer, ok := d.writer.(io.Closer); ok {
		closer.Close()
	}
	return nil
}

func (d *Duplex) IsClosed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closed
}

func (d *Duplex) Pause() {
	d.emit("pause", nil)
}

func (d *Duplex) Resume() {
	d.emit("resume", nil)
}

// Helper to create a PassThrough stream (buffer-based duplex)
func NewPassThrough(obj *goja.Object, dispatch engine.EventDispatchFunc) *Duplex {
	buf := &bytes.Buffer{}
	return NewDuplex(obj, buf, buf, dispatch)
}
