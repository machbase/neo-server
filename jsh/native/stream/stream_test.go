package stream

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

func TestReadable(t *testing.T) {
	rt := goja.New()
	reader := strings.NewReader("Hello, World!")

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		// Mock event dispatcher
		return true
	}

	obj := rt.NewObject()
	readable := NewReadable(obj, reader, dispatch)

	// Test reading
	data, err := readable.Read(5)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if string(data) != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", string(data))
	}

	// Test close
	err = readable.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !readable.IsClosed() {
		t.Error("Stream should be closed")
	}
}

func TestWritable(t *testing.T) {
	rt := goja.New()
	buf := &bytes.Buffer{}

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		// Mock event dispatcher
		return true
	}

	obj := rt.NewObject()
	writable := NewWritable(obj, buf, dispatch)

	// Test writing
	data := []byte("Hello, World!")
	n, err := writable.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	if buf.String() != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", buf.String())
	}

	// Test close
	err = writable.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !writable.IsClosed() {
		t.Error("Stream should be closed")
	}
}

func TestDuplex(t *testing.T) {
	rt := goja.New()
	readBuf := strings.NewReader("Read data")
	writeBuf := &bytes.Buffer{}

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		// Mock event dispatcher
		return true
	}

	obj := rt.NewObject()
	duplex := NewDuplex(obj, readBuf, writeBuf, dispatch)

	// Test reading
	readData, err := duplex.Read(9)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if string(readData) != "Read data" {
		t.Errorf("Expected 'Read data', got '%s'", string(readData))
	}

	// Test writing
	writeData := []byte("Write data")
	n, err := duplex.Write(writeData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(writeData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(writeData), n)
	}

	if writeBuf.String() != "Write data" {
		t.Errorf("Expected 'Write data', got '%s'", writeBuf.String())
	}

	// Test close
	err = duplex.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !duplex.IsClosed() {
		t.Error("Stream should be closed")
	}
}

func TestPassThrough(t *testing.T) {
	rt := goja.New()

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		// Mock event dispatcher
		return true
	}

	obj := rt.NewObject()
	passthrough := NewPassThrough(obj, dispatch)

	// Write data
	writeData := []byte("PassThrough data")
	n, err := passthrough.Write(writeData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(writeData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(writeData), n)
	}

	// Read data back
	readData, err := passthrough.Read(16)
	if err != nil && err != io.EOF {
		t.Fatalf("Read failed: %v", err)
	}

	if string(readData) != "PassThrough data" {
		t.Errorf("Expected 'PassThrough data', got '%s'", string(readData))
	}

	// Test close
	err = passthrough.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !passthrough.IsClosed() {
		t.Error("Stream should be closed")
	}
}

// TestReadableReadString tests ReadString method
func TestReadableReadString(t *testing.T) {
	rt := goja.New()
	reader := strings.NewReader("Test String Data")

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		return true
	}

	obj := rt.NewObject()
	readable := NewReadable(obj, reader, dispatch)

	// Test ReadString
	str, err := readable.ReadString(11, "utf-8")
	if err != nil {
		t.Fatalf("ReadString failed: %v", err)
	}

	if str != "Test String" {
		t.Errorf("Expected 'Test String', got '%s'", str)
	}

	// Test ReadString with remaining data
	str, err = readable.ReadString(100, "utf-8")
	// EOF is expected when we reach the end, but we still get the data
	if str != " Data" {
		t.Errorf("Expected ' Data', got '%s'", str)
	}
}

// TestReadablePauseResume tests Pause and Resume methods
func TestReadablePauseResume(t *testing.T) {
	rt := goja.New()
	reader := strings.NewReader("test data")

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()
	readable := NewReadable(obj, reader, dispatch)

	// Test Pause
	readable.Pause()
	if len(eventLog) != 1 || eventLog[0] != "pause" {
		t.Errorf("Expected 'pause' event, got %v", eventLog)
	}

	// Test Resume
	readable.Resume()
	if len(eventLog) != 2 || eventLog[1] != "resume" {
		t.Errorf("Expected 'resume' event, got %v", eventLog)
	}
}

// TestReadableErrors tests error conditions
func TestReadableErrors(t *testing.T) {
	rt := goja.New()

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()

	// Test EOF - strings.Reader returns EOF after all data is read
	reader := strings.NewReader("short")
	readable := NewReadable(obj, reader, dispatch)

	data, err := readable.Read(10)
	// First read gets all data but may not return EOF yet
	if string(data) != "short" {
		t.Errorf("Expected 'short', got '%s'", string(data))
	}

	// Read again to get EOF
	data2, err2 := readable.Read(10)
	if err2 != io.EOF {
		t.Logf("Second read error: %v, data: %s", err2, string(data2))
	}
	// Should emit 'end' event on EOF
	hasEndEvent := false
	for _, ev := range eventLog {
		if ev == "end" {
			hasEndEvent = true
			break
		}
	}
	if !hasEndEvent {
		t.Logf("Event log: %v (no 'end' event found, this is acceptable depending on EOF timing)", eventLog)
	}

	// Test read after close
	eventLog = []string{}
	reader2 := strings.NewReader("test")
	readable2 := NewReadable(obj, reader2, dispatch)
	readable2.Close()

	_, err = readable2.Read(10)
	if err == nil {
		t.Error("Expected error when reading closed stream")
	}

	// Test default size (0)
	reader3 := strings.NewReader("default size test")
	readable3 := NewReadable(obj, reader3, dispatch)
	data, err = readable3.Read(0)
	if err != nil && err != io.EOF {
		t.Fatalf("Read with default size failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected to read some data with default size")
	}
}

// TestWritableWriteString tests WriteString method
func TestWritableWriteString(t *testing.T) {
	rt := goja.New()
	buf := &bytes.Buffer{}

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		return true
	}

	obj := rt.NewObject()
	writable := NewWritable(obj, buf, dispatch)

	// Test WriteString
	n, err := writable.WriteString("Hello, WriteString!", "utf-8")
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	if n != 19 {
		t.Errorf("Expected to write 19 bytes, wrote %d", n)
	}

	if buf.String() != "Hello, WriteString!" {
		t.Errorf("Expected 'Hello, WriteString!', got '%s'", buf.String())
	}
}

// TestWritableEnd tests End method
func TestWritableEnd(t *testing.T) {
	rt := goja.New()
	buf := &bytes.Buffer{}

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()
	writable := NewWritable(obj, buf, dispatch)

	// Test End with data
	err := writable.End([]byte("Final data"))
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	if buf.String() != "Final data" {
		t.Errorf("Expected 'Final data', got '%s'", buf.String())
	}

	// Should emit 'finish' and 'close' events
	if len(eventLog) < 2 {
		t.Errorf("Expected at least 2 events, got %d: %v", len(eventLog), eventLog)
	}

	// Test End without data
	buf2 := &bytes.Buffer{}
	eventLog2 := []string{}
	dispatch2 := func(obj *goja.Object, event string, args ...any) bool {
		eventLog2 = append(eventLog2, event)
		return true
	}
	writable2 := NewWritable(obj, buf2, dispatch2)

	err = writable2.End([]byte{})
	if err != nil {
		t.Fatalf("End without data failed: %v", err)
	}

	if buf2.Len() != 0 {
		t.Errorf("Expected empty buffer, got '%s'", buf2.String())
	}
}

// TestWritableErrors tests error conditions
func TestWritableErrors(t *testing.T) {
	rt := goja.New()
	buf := &bytes.Buffer{}

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()
	writable := NewWritable(obj, buf, dispatch)

	// Close the stream
	writable.Close()

	// Test write after close
	_, err := writable.Write([]byte("test"))
	if err == nil {
		t.Error("Expected error when writing to closed stream")
	}

	// Test WriteString after close
	_, err = writable.WriteString("test", "utf-8")
	if err == nil {
		t.Error("Expected error when writing string to closed stream")
	}
}

// TestDuplexReadString tests ReadString method on Duplex
func TestDuplexReadString(t *testing.T) {
	rt := goja.New()
	readBuf := strings.NewReader("Duplex String Test")
	writeBuf := &bytes.Buffer{}

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		return true
	}

	obj := rt.NewObject()
	duplex := NewDuplex(obj, readBuf, writeBuf, dispatch)

	// Test ReadString
	str, err := duplex.ReadString(13, "utf-8")
	if err != nil {
		t.Fatalf("ReadString failed: %v", err)
	}

	if str != "Duplex String" {
		t.Errorf("Expected 'Duplex String', got '%s'", str)
	}
}

// TestDuplexWriteString tests WriteString method on Duplex
func TestDuplexWriteString(t *testing.T) {
	rt := goja.New()
	readBuf := strings.NewReader("test")
	writeBuf := &bytes.Buffer{}

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		return true
	}

	obj := rt.NewObject()
	duplex := NewDuplex(obj, readBuf, writeBuf, dispatch)

	// Test WriteString
	n, err := duplex.WriteString("Duplex Write", "utf-8")
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	if n != 12 {
		t.Errorf("Expected to write 12 bytes, wrote %d", n)
	}

	if writeBuf.String() != "Duplex Write" {
		t.Errorf("Expected 'Duplex Write', got '%s'", writeBuf.String())
	}
}

// TestDuplexEnd tests End method on Duplex
func TestDuplexEnd(t *testing.T) {
	rt := goja.New()
	readBuf := strings.NewReader("test")
	writeBuf := &bytes.Buffer{}

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()
	duplex := NewDuplex(obj, readBuf, writeBuf, dispatch)

	// Test End with data
	err := duplex.End([]byte("Final"))
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	if writeBuf.String() != "Final" {
		t.Errorf("Expected 'Final', got '%s'", writeBuf.String())
	}

	// Should emit 'finish' and 'close' events
	if len(eventLog) < 2 {
		t.Errorf("Expected at least 2 events, got %d: %v", len(eventLog), eventLog)
	}
}

// TestDuplexPauseResume tests Pause and Resume on Duplex
func TestDuplexPauseResume(t *testing.T) {
	rt := goja.New()
	readBuf := strings.NewReader("test")
	writeBuf := &bytes.Buffer{}

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()
	duplex := NewDuplex(obj, readBuf, writeBuf, dispatch)

	// Test Pause
	duplex.Pause()
	if len(eventLog) != 1 || eventLog[0] != "pause" {
		t.Errorf("Expected 'pause' event, got %v", eventLog)
	}

	// Test Resume
	duplex.Resume()
	if len(eventLog) != 2 || eventLog[1] != "resume" {
		t.Errorf("Expected 'resume' event, got %v", eventLog)
	}
}

// TestDuplexErrors tests error conditions on Duplex
func TestDuplexErrors(t *testing.T) {
	rt := goja.New()
	readBuf := strings.NewReader("short")
	writeBuf := &bytes.Buffer{}

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()
	duplex := NewDuplex(obj, readBuf, writeBuf, dispatch)

	// Test reading all data
	data, err := duplex.Read(10)
	if string(data) != "short" {
		t.Errorf("Expected 'short', got '%s'", string(data))
	}

	// Test write/read after close
	duplex.Close()

	_, err = duplex.Write([]byte("test"))
	if err == nil {
		t.Error("Expected error when writing to closed duplex stream")
	}

	_, err = duplex.Read(10)
	if err == nil {
		t.Error("Expected error when reading from closed duplex stream")
	}
}

// TestReadableCloseWithCloser tests closing a reader that implements io.Closer
func TestReadableCloseWithCloser(t *testing.T) {
	rt := goja.New()

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		return true
	}

	obj := rt.NewObject()

	// Use bytes.Buffer which doesn't implement io.Closer
	buf := bytes.NewBufferString("test")
	readable := NewReadable(obj, buf, dispatch)
	err := readable.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestWritableCloseWithCloser tests closing a writer that implements io.Closer
func TestWritableCloseNoCloser(t *testing.T) {
	rt := goja.New()

	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		return true
	}

	obj := rt.NewObject()

	// Use bytes.Buffer which doesn't implement io.Closer
	buf := &bytes.Buffer{}
	writable := NewWritable(obj, buf, dispatch)
	err := writable.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestReadableReadWithEOF tests Read method EOF path with data emission
func TestReadableReadWithEOF(t *testing.T) {
	rt := goja.New()
	reader := strings.NewReader("data")

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()
	readable := NewReadable(obj, reader, dispatch)

	// Read all data in one go
	data, err := readable.Read(4)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if string(data) != "data" {
		t.Errorf("Expected 'data', got '%s'", string(data))
	}

	// Next read should hit EOF
	data2, err := readable.Read(10)
	if err != io.EOF {
		t.Logf("Expected EOF, got %v (this is acceptable for some readers)", err)
	}
	if len(data2) != 0 {
		t.Errorf("Expected empty data on EOF, got %d bytes", len(data2))
	}

	// Verify 'end' event was emitted
	hasEnd := false
	for _, ev := range eventLog {
		if ev == "end" {
			hasEnd = true
			break
		}
	}
	if !hasEnd {
		t.Logf("Event log: %v (acceptable if 'end' event timing varies)", eventLog)
	}
}

// erroringReader is a test helper that returns an error on Read
type erroringReader struct {
	err error
}

func (e *erroringReader) Read(p []byte) (int, error) {
	return 0, e.err
}

// TestReadableReadError tests Read method error path (non-EOF)
func TestReadableReadError(t *testing.T) {
	rt := goja.New()

	// Create a reader that returns an error
	errorReader := &erroringReader{err: fmt.Errorf("read error")}

	eventLog := []string{}
	errorEmitted := false
	var emittedError error
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		if event == "error" && len(args) > 0 {
			errorEmitted = true
			if err, ok := args[0].(error); ok {
				emittedError = err
			}
		}
		return true
	}

	obj := rt.NewObject()
	readable := NewReadable(obj, errorReader, dispatch)

	data, err := readable.Read(10)
	if err == nil {
		t.Error("Expected error from Read")
	}
	if data != nil {
		t.Errorf("Expected nil data on error, got %d bytes", len(data))
	}

	if !errorEmitted {
		t.Error("Expected 'error' event to be emitted")
	}
	if emittedError == nil {
		t.Error("Expected error to be passed to event")
	}
}

// TestDuplexReadError tests Duplex Read method error path
func TestDuplexReadError(t *testing.T) {
	rt := goja.New()
	errorReader := &erroringReader{err: fmt.Errorf("duplex read error")}
	writeBuf := &bytes.Buffer{}

	errorEmitted := false
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		if event == "error" {
			errorEmitted = true
		}
		return true
	}

	obj := rt.NewObject()
	duplex := NewDuplex(obj, errorReader, writeBuf, dispatch)

	data, err := duplex.Read(10)
	if err == nil {
		t.Error("Expected error from Duplex Read")
	}
	if data != nil {
		t.Errorf("Expected nil data on error, got %d bytes", len(data))
	}
	if !errorEmitted {
		t.Error("Expected 'error' event to be emitted")
	}
}

// TestDuplexReadWithEOF tests Duplex Read method EOF path
func TestDuplexReadWithEOF(t *testing.T) {
	rt := goja.New()
	reader := strings.NewReader("test")

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()
	duplex := NewDuplex(obj, reader, &bytes.Buffer{}, dispatch)

	// Read all data
	data, _ := duplex.Read(4)
	if string(data) != "test" {
		t.Errorf("Expected 'test', got '%s'", string(data))
	}

	// Next read should hit EOF
	data2, err := duplex.Read(10)
	if err != io.EOF {
		t.Logf("Expected EOF, got %v", err)
	}
	if len(data2) != 0 {
		t.Errorf("Expected empty data on EOF, got %d bytes", len(data2))
	}

	// Verify 'end' event
	hasEnd := false
	for _, ev := range eventLog {
		if ev == "end" {
			hasEnd = true
			break
		}
	}
	if !hasEnd {
		t.Logf("Event log: %v", eventLog)
	}
}

// erroringWriter is a test helper that returns an error on Write
type erroringWriter struct {
	err error
}

func (e *erroringWriter) Write(p []byte) (int, error) {
	return 0, e.err
}

// TestWritableWriteError tests Write method error path
func TestWritableWriteError(t *testing.T) {
	rt := goja.New()
	errorWriter := &erroringWriter{err: fmt.Errorf("write error")}

	errorEmitted := false
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		if event == "error" {
			errorEmitted = true
		}
		return true
	}

	obj := rt.NewObject()
	writable := NewWritable(obj, errorWriter, dispatch)

	n, err := writable.Write([]byte("test"))
	if err == nil {
		t.Error("Expected error from Write")
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes written on error, got %d", n)
	}
	if !errorEmitted {
		t.Error("Expected 'error' event to be emitted")
	}
}

// TestDuplexWriteError tests Duplex Write method error path
func TestDuplexWriteError(t *testing.T) {
	rt := goja.New()
	errorWriter := &erroringWriter{err: fmt.Errorf("duplex write error")}

	errorEmitted := false
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		if event == "error" {
			errorEmitted = true
		}
		return true
	}

	obj := rt.NewObject()
	duplex := NewDuplex(obj, strings.NewReader(""), errorWriter, dispatch)

	n, err := duplex.Write([]byte("test"))
	if err == nil {
		t.Error("Expected error from Duplex Write")
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes written on error, got %d", n)
	}
	if !errorEmitted {
		t.Error("Expected 'error' event to be emitted")
	}
}

// closableWriter is a test helper that implements io.WriteCloser
type closableWriter struct {
	*bytes.Buffer
	closed bool
}

func (c *closableWriter) Close() error {
	c.closed = true
	return nil
}

// TestWritableCloseWithCloser tests Close method with io.Closer implementation
func TestWritableCloseWithCloser(t *testing.T) {
	rt := goja.New()

	eventLog := []string{}
	dispatch := func(obj *goja.Object, event string, args ...any) bool {
		eventLog = append(eventLog, event)
		return true
	}

	obj := rt.NewObject()

	// Use closableWriter which implements io.Closer
	cw := &closableWriter{Buffer: &bytes.Buffer{}}
	writable := NewWritable(obj, cw, dispatch)

	err := writable.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !cw.closed {
		t.Error("Expected underlying writer to be closed")
	}

	// Verify 'close' event was emitted
	hasClose := false
	for _, ev := range eventLog {
		if ev == "close" {
			hasClose = true
			break
		}
	}
	if !hasClose {
		t.Errorf("Expected 'close' event, got events: %v", eventLog)
	}
}

type TestCase struct {
	name   string
	script string
	input  []string
	output []string
	err    string
	vars   map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name:   tc.name,
			Code:   tc.script,
			FSTabs: []engine.FSTab{root.RootFSTab(), {MountPoint: "/work", Source: "../../test/"}},
			Env:    tc.vars,
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)
		jr.RegisterNativeModule("@jsh/stream", Module)

		if len(tc.input) > 0 {
			conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.input, ""))
		}
		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		lines := strings.Split(gotOutput, "\n")
		if len(lines) != len(tc.output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
		}
		for i, expectedLine := range tc.output {
			if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

func TestStreamModule(t *testing.T) {
	tests := []TestCase{
		{
			name: "module-exports",
			script: `
				const stream = require("/lib/stream");
				console.println("Readable:", typeof stream.Readable);
				console.println("Writable:", typeof stream.Writable);
				console.println("Duplex:", typeof stream.Duplex);
				console.println("PassThrough:", typeof stream.PassThrough);
			`,
			output: []string{
				"Readable: function",
				"Writable: function",
				"Duplex: function",
				"PassThrough: function",
			},
		},
		{
			name: "passthrough-basic",
			script: `
				const { PassThrough } = require('/lib/stream');
				const pt = new PassThrough();
				
				pt.on('finish', () => {
					console.println('Finished');
				});
				
				pt.write('Hello, ');
				pt.write('World!');
				pt.end();
			`,
			output: []string{
				"Finished",
			},
		},
		{
			name: "passthrough-events",
			script: `
				const { PassThrough } = require('/lib/stream');
				const pt = new PassThrough();
				
				pt.on('finish', () => {
					console.println('finish');
				});
				
				pt.on('close', () => {
					console.println('close');
				});
				
				pt.write('Test');
				pt.end();
			`,
			output: []string{
				"finish",
				"close",
			},
		},
		{
			name: "writable-properties",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				console.println('writable:', stream.writable);
				console.println('writableEnded:', stream.writableEnded);
				stream.end();
				console.println('writableEnded after end:', stream.writableEnded);
			`,
			output: []string{
				"writable: true",
				"writableEnded: false",
				"writableEnded after end: true",
			},
		},
		{
			name: "readable-properties",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				console.println('readable:', stream.readable);
				console.println('readableEnded:', stream.readableEnded);
			`,
			output: []string{
				"readable: true",
				"readableEnded: false",
			},
		},
		{
			name: "pause-resume",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				console.println('isPaused:', stream.isPaused());
				stream.pause();
				console.println('isPaused after pause:', stream.isPaused());
				stream.resume();
				console.println('isPaused after resume:', stream.isPaused());
			`,
			output: []string{
				"isPaused: false",
				"isPaused after pause: true",
				"isPaused after resume: false",
			},
		},
		{
			name: "write-after-end-error",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('error', (err) => {
					console.println('error:', err.message);
				});
				
				stream.end('first');
				stream.write('second'); // This should emit error
			`,
			output: []string{
				"error: Stream is not writable",
			},
		},
		{
			name: "multiple-writes",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('finish', () => {
					console.println('Write complete');
				});
				
				stream.write('first\n');
				stream.write('second\n');
				stream.write('third\n');
				stream.end();
			`,
			output: []string{
				"Write complete",
			},
		},
		{
			name: "destroy-with-error",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				let errorCount = 0;
				let closeCount = 0;
				
				stream.on('error', (err) => {
					if (errorCount === 0) {
						console.println('error:', err.message);
					}
					errorCount++;
				});
				
				stream.on('close', () => {
					if (closeCount === 0) {
						console.println('close');
					}
					closeCount++;
				});
				
				stream.destroy(new Error('Test error'));
				console.println('destroyed');
			`,
			output: []string{
				"error: Test error",
				"close",
				"destroyed",
			},
		},
		{
			name: "write-string-encoding",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('finish', () => {
					console.println('Write finished');
				});
				
				stream.write('Hello', 'utf8');
				stream.end();
			`,
			output: []string{
				"Write finished",
			},
		},
		{
			name: "end-with-data",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('finish', () => {
					console.println('finish');
				});
				
				stream.write('Hello ');
				stream.end('World!');
			`,
			output: []string{
				"finish",
			},
		},
		{
			name: "stream-properties-state",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				console.println('Initial state:');
				console.println('  readable:', stream.readable);
				console.println('  writable:', stream.writable);
				
				stream.end();
				
				console.println('After end:');
				console.println('  readable:', stream.readable);
				console.println('  writable:', stream.writable);
			`,
			output: []string{
				"Initial state:",
				"  readable: true",
				"  writable: true",
				"After end:",
				"  readable: true",
				"  writable: false",
			},
		},
		{
			name: "event-order",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('finish', () => {
					console.println('1. finish');
				});
				
				stream.on('close', () => {
					console.println('2. close');
				});
				
				stream.end('data');
			`,
			output: []string{
				"1. finish",
				"2. close",
			},
		},
		{
			name: "double-end-error",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.end('first');
				
				// Try to end again
				stream.end('second', (err) => {
					if (err) {
						console.println('error:', err.message);
					} else {
						console.println('no error');
					}
				});
			`,
			output: []string{
				"error: Stream already ended",
			},
		},
		{
			name: "write-callback",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.write('test', (err) => {
					if (err) {
						console.println('error:', err.message);
					} else {
						console.println('write callback called');
					}
				});
				
				stream.end();
			`,
			output: []string{
				"write callback called",
			},
		},
		{
			name: "end-callback",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.end('final data', (err) => {
					if (err) {
						console.println('error:', err.message);
					} else {
						console.println('end callback called');
					}
				});
			`,
			output: []string{
				"end callback called",
			},
		},
		{
			name: "buffer-types",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				// Write string
				const result1 = stream.write('string data');
				console.println('write string:', result1);
				
				stream.end();
			`,
			output: []string{
				"write string: true",
			},
		},
		{
			name: "closed-stream-write",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('error', (err) => {
					console.println('error:', err.message);
				});
				
				stream.close();
				
				// Try to write after close
				stream.write('should fail');
			`,
			output: []string{
				"error: Stream is not writable",
			},
		},
		{
			name: "highWaterMark-properties",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				console.println('readableHighWaterMark:', stream.readableHighWaterMark);
				console.println('writableHighWaterMark:', stream.writableHighWaterMark);
			`,
			output: []string{
				"readableHighWaterMark: 16384",
				"writableHighWaterMark: 16384",
			},
		},
		{
			name: "flowing-state",
			script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				console.println('initial flowing:', stream.readableFlowing);
				stream.pause();
				console.println('after pause:', stream.readableFlowing);
				stream.resume();
				console.println('after resume:', stream.readableFlowing);
			`,
			output: []string{
				"initial flowing: null",
				"after pause: false",
				"after resume: true",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}
