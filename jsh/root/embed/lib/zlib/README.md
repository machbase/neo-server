# zlib Module for jsh

A Node.js-compatible compression module for jsh (JavaScript Shell). This module provides data compression and decompression using gzip, deflate, and other compression algorithms.

## Installation

The module is located at `/lib/zlib` and can be required in your jsh scripts:

```javascript
const zlib = require('/lib/zlib');
```

## Features

- **Node.js Compatible API**: Familiar function names and behavior similar to Node.js zlib module
- **Multiple Compression Formats**: Support for gzip, deflate, raw deflate, and auto-detection (unzip)
- **Synchronous & Asynchronous**: Both sync and async (callback-based) operations
- **Stream API**: Create compression/decompression streams for piping
- **Constants**: Access to zlib constants for fine-tuned control

## API Reference

### Synchronous Convenience Methods

These methods compress or decompress data synchronously and return ArrayBuffer.

#### gzipSync(data)
Compress data using gzip.

```javascript
const zlib = require('/lib/zlib');
const compressed = zlib.gzipSync('Hello, World!');
console.println('Compressed size:', compressed.byteLength);
```

**Parameters:**
- `data` (string|ArrayBuffer): Data to compress

**Returns:** ArrayBuffer containing compressed data

#### gunzipSync(data)
Decompress gzip data.

```javascript
const zlib = require('/lib/zlib');
const compressed = zlib.gzipSync('Hello, World!');
const decompressed = zlib.gunzipSync(compressed);
const text = String.fromCharCode.apply(null, new Uint8Array(decompressed));
console.println(text); // "Hello, World!"
```

**Parameters:**
- `data` (ArrayBuffer): Compressed data

**Returns:** ArrayBuffer containing decompressed data

#### deflateSync(data)
Compress data using deflate.

```javascript
const compressed = zlib.deflateSync('Some data to compress');
```

**Parameters:**
- `data` (string|ArrayBuffer): Data to compress

**Returns:** ArrayBuffer containing compressed data

#### inflateSync(data)
Decompress deflate data.

```javascript
const compressed = zlib.deflateSync('Some data');
const decompressed = zlib.inflateSync(compressed);
```

**Parameters:**
- `data` (ArrayBuffer): Compressed data

**Returns:** ArrayBuffer containing decompressed data

#### deflateRawSync(data)
Compress data using raw deflate (without zlib header).

```javascript
const compressed = zlib.deflateRawSync('Raw deflate data');
```

**Parameters:**
- `data` (string|ArrayBuffer): Data to compress

**Returns:** ArrayBuffer containing compressed data

#### inflateRawSync(data)
Decompress raw deflate data.

```javascript
const compressed = zlib.deflateRawSync('Raw data');
const decompressed = zlib.inflateRawSync(compressed);
```

**Parameters:**
- `data` (ArrayBuffer): Compressed data

**Returns:** ArrayBuffer containing decompressed data

#### unzipSync(data)
Decompress gzip or deflate data (auto-detect format).

```javascript
const decompressed = zlib.unzipSync(compressedData);
```

**Parameters:**
- `data` (ArrayBuffer): Compressed data

**Returns:** ArrayBuffer containing decompressed data

### Asynchronous Convenience Methods

These methods compress or decompress data asynchronously using callbacks.

#### gzip(data, callback)
Compress data using gzip asynchronously.

```javascript
const zlib = require('/lib/zlib');
zlib.gzip('Hello, World!', (err, compressed) => {
    if (err) {
        console.println('Error:', err.message);
        return;
    }
    console.println('Compressed size:', compressed.byteLength);
});
```

**Parameters:**
- `data` (string|ArrayBuffer): Data to compress
- `callback` (Function): Callback function (err, result)

#### gunzip(data, callback)
Decompress gzip data asynchronously.

```javascript
zlib.gunzip(compressed, (err, decompressed) => {
    if (err) {
        console.println('Error:', err.message);
        return;
    }
    const text = String.fromCharCode.apply(null, new Uint8Array(decompressed));
    console.println(text);
});
```

**Parameters:**
- `data` (ArrayBuffer): Compressed data
- `callback` (Function): Callback function (err, result)

#### deflate(data, callback), inflate(data, callback)
Compress/decompress using deflate asynchronously.

```javascript
zlib.deflate('Some data', (err, compressed) => {
    if (err) throw err;
    
    zlib.inflate(compressed, (err, decompressed) => {
        if (err) throw err;
        console.println('Decompressed');
    });
});
```

#### deflateRaw(data, callback), inflateRaw(data, callback)
Compress/decompress using raw deflate asynchronously.

#### unzip(data, callback)
Decompress data with auto-detection asynchronously.

### Stream API

Create compression/decompression streams for piping operations.

#### createGzip()
Create a gzip compression stream.

```javascript
const zlib = require('/lib/zlib');
const gzip = zlib.createGzip();

let result = null;
gzip.on('data', (chunk) => {
    result = chunk;
    console.println('Compressed chunk received');
});

gzip.on('end', () => {
    console.println('Compression complete');
});

gzip.on('error', (err) => {
    console.println('Error:', err.message);
});

gzip.write('Hello, ');
gzip.write('World!');
gzip.end();
```

**Returns:** Compression stream object with methods:
- `write(data)` - Write data to stream
- `end([data])` - End the stream (optionally write final data)
- `on(event, callback)` - Register event listener ('data', 'end', 'error')
- `pipe(dest)` - Pipe to destination stream
- `flush()` - Flush pending data
- `close()` - Close the stream

#### createGunzip()
Create a gzip decompression stream.

```javascript
const gunzip = zlib.createGunzip();

gunzip.on('data', (chunk) => {
    const text = String.fromCharCode.apply(null, new Uint8Array(chunk));
    console.println('Decompressed:', text);
});

gunzip.on('end', () => {
    console.println('Decompression complete');
});

gunzip.write(compressedData);
gunzip.end();
```

**Returns:** Decompression stream object

#### createDeflate(), createInflate()
Create deflate compression/decompression streams.

```javascript
const deflate = zlib.createDeflate();
const inflate = zlib.createInflate();
```

#### createDeflateRaw(), createInflateRaw()
Create raw deflate compression/decompression streams.

```javascript
const deflateRaw = zlib.createDeflateRaw();
const inflateRaw = zlib.createInflateRaw();
```

#### createUnzip()
Create a decompression stream with auto-detection.

```javascript
const unzip = zlib.createUnzip();
```

### Constants

Access zlib constants for compression options and status codes.

```javascript
const zlib = require('/lib/zlib');
const c = zlib.constants;

console.println('Z_NO_FLUSH:', c.Z_NO_FLUSH);
console.println('Z_BEST_COMPRESSION:', c.Z_BEST_COMPRESSION);
console.println('Z_DEFAULT_COMPRESSION:', c.Z_DEFAULT_COMPRESSION);
```

#### Flush Values
- `Z_NO_FLUSH` (0)
- `Z_PARTIAL_FLUSH` (1)
- `Z_SYNC_FLUSH` (2)
- `Z_FULL_FLUSH` (3)
- `Z_FINISH` (4)
- `Z_BLOCK` (5)

#### Return Codes
- `Z_OK` (0)
- `Z_STREAM_END` (1)
- `Z_NEED_DICT` (2)
- `Z_ERRNO` (-1)
- `Z_STREAM_ERROR` (-2)
- `Z_DATA_ERROR` (-3)
- `Z_MEM_ERROR` (-4)
- `Z_BUF_ERROR` (-5)
- `Z_VERSION_ERROR` (-6)

#### Compression Levels
- `Z_NO_COMPRESSION` (0)
- `Z_BEST_SPEED` (1)
- `Z_BEST_COMPRESSION` (9)
- `Z_DEFAULT_COMPRESSION` (-1)

#### Compression Strategy
- `Z_FILTERED` (1)
- `Z_HUFFMAN_ONLY` (2)
- `Z_RLE` (3)
- `Z_FIXED` (4)
- `Z_DEFAULT_STRATEGY` (0)

## Usage Examples

### Basic Compression and Decompression

```javascript
const zlib = require('/lib/zlib');

// Compress a string
const original = "This is some text that will be compressed.";
const compressed = zlib.gzipSync(original);
console.println('Original size:', original.length);
console.println('Compressed size:', compressed.byteLength);

// Decompress
const decompressed = zlib.gunzipSync(compressed);
const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
console.println('Decompressed:', result);
console.println('Match:', result === original);
```

### Using Deflate

```javascript
const zlib = require('/lib/zlib');

const data = "Data to compress with deflate";
const compressed = zlib.deflateSync(data);
const decompressed = zlib.inflateSync(compressed);
const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
console.println('Result:', result);
```

### Async Compression

```javascript
const zlib = require('/lib/zlib');

const data = "Async compression example";
zlib.gzip(data, (err, compressed) => {
    if (err) {
        console.println('Compression error:', err.message);
        return;
    }
    
    console.println('Compressed successfully');
    
    zlib.gunzip(compressed, (err, decompressed) => {
        if (err) {
            console.println('Decompression error:', err.message);
            return;
        }
        
        const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
        console.println('Result:', result);
    });
});
```

### Stream-based Compression

```javascript
const zlib = require('/lib/zlib');

const gzip = zlib.createGzip();
let compressedData = null;

// Set up event handlers
gzip.on('data', (chunk) => {
    compressedData = chunk;
});

gzip.on('end', () => {
    console.println('Compression finished');
    console.println('Compressed size:', compressedData.byteLength);
    
    // Now decompress
    const gunzip = zlib.createGunzip();
    
    gunzip.on('data', (chunk) => {
        const text = String.fromCharCode.apply(null, new Uint8Array(chunk));
        console.println('Original text:', text);
    });
    
    gunzip.on('end', () => {
        console.println('Decompression finished');
    });
    
    gunzip.write(compressedData);
    gunzip.end();
});

gzip.on('error', (err) => {
    console.println('Error:', err.message);
});

// Write data and close
gzip.write('Hello, ');
gzip.write('streaming ');
gzip.write('compression!');
gzip.end();
```

### Destructuring Imports

```javascript
const { gzipSync, gunzipSync, constants } = require('/lib/zlib');

const compressed = gzipSync('Quick compression');
const decompressed = gunzipSync(compressed);
const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));

console.println('Result:', result);
console.println('Best compression level:', constants.Z_BEST_COMPRESSION);
```

## Working with Binary Data

When working with compressed data, remember:
- Compression functions return `ArrayBuffer` objects
- To convert ArrayBuffer to string, use: `String.fromCharCode.apply(null, new Uint8Array(buffer))`
- Compressed data is binary and should not be converted to string before decompression

```javascript
const zlib = require('/lib/zlib');

// Compress
const text = "Hello, World!";
const compressed = zlib.gzipSync(text);

// compressed is an ArrayBuffer - don't convert to string!
// Store or transmit as binary data

// Decompress
const decompressed = zlib.gunzipSync(compressed);

// Convert decompressed ArrayBuffer to string
const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
console.println(result); // "Hello, World!"
```

## Error Handling

```javascript
const zlib = require('/lib/zlib');

try {
    // Try to decompress invalid data
    const invalidData = new ArrayBuffer(10);
    const result = zlib.gunzipSync(invalidData);
} catch (err) {
    console.println('Error:', err.message);
    // Error: gzip: invalid header
}

// Async error handling
zlib.gunzip(invalidData, (err, result) => {
    if (err) {
        console.println('Decompression failed:', err.message);
        return;
    }
    // Process result
});
```

## Notes

- All compression/decompression operations work with binary data (ArrayBuffer)
- String data is automatically converted to bytes using UTF-8 encoding
- Compressed data cannot be meaningfully displayed as text
- For large data, consider using streaming API to avoid memory issues
- The module uses Go's standard compress/gzip and compress/flate packages

## Differences from Node.js

- No support for Brotli or Zstd compression (only gzip and deflate)
- Options like compression level, window bits, etc. are not yet configurable
- All operations use default compression settings
- Stream API is simplified compared to Node.js streams
- No support for promisified versions (use callbacks or sync methods)

## See Also

- [Node.js zlib documentation](https://nodejs.org/api/zlib.html) - Reference for Node.js zlib module
- `/lib/fs` - Filesystem module
- `/lib/stream` - Stream module
