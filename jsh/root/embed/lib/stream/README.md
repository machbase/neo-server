# Stream Module

Node.js-compatible stream module built on top of Go's `io.Reader` and `io.Writer` interfaces.

## Overview

This module wraps Go's I/O interfaces into Node.js-style streams for the jsh environment.
It provides `Readable`, `Writable`, `Duplex`, and `PassThrough` stream types.

## Installation/Usage

```javascript
const { Readable, Writable, Duplex, PassThrough } = require('/lib/stream');
```

## Architecture

### Go Implementation (jsh/native/stream/stream.go)

- **Readable**: Exports Go's `io.Reader` to JavaScript
- **Writable**: Exports Go's `io.Writer` to JavaScript
- **Duplex**: Stream that supports both reading and writing
- **PassThrough**: Buffer-based bidirectional stream

Events emitted:
- `data`: When a data chunk is read
- `end`: When the stream has ended (EOF)
- `error`: When an error occurs
- `close`: When the stream is closed
- `finish`: When writing is complete
- `pause`: When the stream is paused
- `resume`: When the stream is resumed

### JavaScript Implementation (jsh/native/root/lib/stream/index.js)

Provides Node.js-compatible stream API:
- Inherits from EventEmitter for event handling
- Standard methods like `read()`, `write()`, `pipe()`
- Automatic buffering and backpressure management

## API

### Readable

Read-only stream.

#### Constructor

```javascript
new Readable(reader)
```

- `reader`: Go's io.Reader implementation

#### Methods

- `read(size)`: Reads data from the stream. Returns a buffer or null if no data is available
- `readString(size, encoding)`: Reads data as a string
- `pause()`: Pauses the stream
- `resume()`: Resumes the stream
- `pipe(destination, options)`: Pipes data to another stream
- `unpipe(destination)`: Stops piping
- `destroy(error)`: Destroys the stream and cleans up resources
- `close()`: Closes the stream

#### Events

- `data`: When a data chunk is available
- `end`: When there is no more data to read
- `error`: When an error occurs
- `close`: When the stream is closed
- `pause`: When paused
- `resume`: When resumed

#### Properties

- `readable`: Whether the stream is readable (boolean)
- `readableEnded`: Whether the stream has ended (boolean)
- `readableFlowing`: Flow mode state (null, true, false)

### Writable

Write-only stream.

#### Constructor

```javascript
new Writable(writer)
```

- `writer`: Go's io.Writer implementation

#### Methods

- `write(chunk, encoding, callback)`: Writes data
- `end(chunk, encoding, callback)`: Writes final data and closes the stream
- `destroy(error)`: Destroys the stream
- `close()`: Closes the stream

#### Events

- `drain`: When the write buffer is emptied
- `finish`: When all data has been written
- `error`: When an error occurs
- `close`: When the stream is closed

#### Properties

- `writable`: Whether the stream is writable (boolean)
- `writableEnded`: Whether end has been called (boolean)
- `writableFinished`: Whether all data has been flushed (boolean)

### Duplex

Stream that supports both reading and writing.

#### Constructor

```javascript
new Duplex(reader, writer)
```

- `reader`: Go's io.Reader implementation
- `writer`: Go's io.Writer implementation

#### Methods

Includes all methods from both Readable and Writable.

#### Events

Includes all events from both Readable and Writable.

### PassThrough

Simple transform stream that passes input directly to output.

#### Constructor

```javascript
new PassThrough()
```

Creates an internal buffer.

## Examples

### Reading Files

```javascript
const { Readable } = require('/lib/stream');
const fs = require('/lib/fs');

const fd = fs.openSync('/path/to/file.txt', 'r');
const stream = new Readable(fd);

stream.on('data', (chunk) => {
    console.log(chunk.toString());
});

stream.on('end', () => {
    console.log('Done reading');
});
```

### Writing Files

```javascript
const { Writable } = require('/lib/stream');
const fs = require('/lib/fs');

const fd = fs.openSync('/path/to/output.txt', 'w');
const stream = new Writable(fd);

stream.write('Hello, ');
stream.write('World!');
stream.end('\n');

stream.on('finish', () => {
    console.log('Done writing');
});
```

### Piping Streams

```javascript
const { Readable, Writable } = require('/lib/stream');

const input = new Readable(inputReader);
const output = new Writable(outputWriter);

input.pipe(output);
```

### Data Transformation

```javascript
const { Readable, Writable } = require('/lib/stream');

const source = new Readable(sourceReader);
const dest = new Writable(destWriter);

source.on('data', (chunk) => {
    // Transform data (e.g., to uppercase)
    const transformed = chunk.toString().toUpperCase();
    dest.write(transformed);
});

source.on('end', () => {
    dest.end();
});
```

### Handling Backpressure

```javascript
const { Readable, Writable } = require('/lib/stream');

const input = new Readable(inputReader);
const output = new Writable(outputWriter);

input.on('data', (chunk) => {
    const canContinue = output.write(chunk);
    if (!canContinue) {
        input.pause();
    }
});

output.on('drain', () => {
    input.resume();
});

input.on('end', () => {
    output.end();
});
```

## Integration with Go

### Passing io.Reader/Writer from Go

```go
package example

import (
    "io"
    "os"
    "github.com/machbase/neo-server/v8/jsh/engine"
    "github.com/machbase/neo-server/v8/jsh/native/stream"
)

func MyFunction(rt *engine.JSRuntime) {
    // Open file and pass as io.Reader
    file, _ := os.Open("example.txt")
    
    // Create Readable for use in JavaScript
    obj := rt.VM.NewObject()
    readable := stream.NewReadable(obj, file, rt.DispatchEvent)
    
    // Expose to JavaScript
    rt.VM.Set("myReader", readable)
}
```

### Custom io.Reader/Writer Implementation

```go
type MyReader struct {
    data []byte
    pos  int
}

func (r *MyReader) Read(p []byte) (n int, err error) {
    if r.pos >= len(r.data) {
        return 0, io.EOF
    }
    n = copy(p, r.data[r.pos:])
    r.pos += n
    return n, nil
}

// Use in JavaScript
reader := &MyReader{data: []byte("Hello, World!")}
readable := stream.NewReadable(obj, reader, dispatch)
```

## Notes

- All streams inherit from EventEmitter
- Always listen for 'error' events for proper error handling
- Call `close()` or `destroy()` after using streams to clean up resources
- Backpressure is automatically managed when using `pipe()`
- Buffer class is implemented based on Uint8Array

## Related Modules

- `/lib/events`: EventEmitter
- `/lib/fs`: File system operations
- `/lib/http`: HTTP request/response streams
- `@jsh/stream`: Native Go implementation

## License

This module is part of the jsh project.
