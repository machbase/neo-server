# HTTP Module

A JSH native module that provides HTTP client functionality similar to Node.js's `http` module.

## Installation

```javascript
const http = require("/lib/http");
```

## Functions

### http.request(url[, options][, callback])
### http.request(options[, callback])

Creates an HTTP request.

**Parameters:**

- `url` (string | URL): The URL to request
- `options` (Object): Optional request configuration
  - `method` (string): HTTP method (default: "GET")
  - `protocol` (string): Protocol (e.g., "http:", "https:")
  - `host` (string): Hostname with port
  - `hostname` (string): Hostname without port
  - `port` (number): Port number
  - `path` (string): Request path including query string
  - `headers` (Object): Request headers
  - `auth` (string): Basic authentication in the format "username:password"
  - `agent` (Agent): HTTP agent instance
  - `url` (string | URL): URL (when using options-only form)
- `callback` (Function): Optional callback function that receives the response

**Returns:** ClientRequest

**Examples:**

```javascript
// Simple GET request with URL string
const req = http.request("http://example.com");
req.end((response) => {
    console.println("Status:", response.statusCode);
});

// GET request with URL object
const url = new URL("http://example.com/api");
const req = http.request(url);
req.end();

// POST request with options
const req = http.request("http://example.com/api", {
    method: "POST",
    headers: {
        "Content-Type": "application/json"
    }
});
req.write('{"key": "value"}');
req.end();

// Using options object
const req = http.request({
    url: "http://example.com/api",
    method: "GET",
    headers: {
        "User-Agent": "MyApp/1.0"
    }
});
req.end();
```

### http.get(url[, options][, callback])
### http.get(options[, callback])

Convenience method for GET requests. Automatically calls `req.end()`.

**Parameters:**

Same as `http.request()`, but the method is automatically set to GET.

**Returns:** ClientRequest

**Examples:**

```javascript
// Simple GET with callback
http.get("http://example.com", (response) => {
    console.println("Status:", response.statusCode);
    console.println("Body:", response.text());
});

// GET with URL object
const url = new URL("http://example.com/api");
http.get(url, (response) => {
    console.println("Status:", response.statusCode);
});

// GET with custom headers
http.get("http://example.com", {
    headers: {
        "X-Custom-Header": "value"
    }
}, (response) => {
    console.println("Header:", response.headers["X-Custom-Header"]);
});

// Using event listener instead of callback
const req = http.get("http://example.com");
req.on("response", (response) => {
    console.println("Status:", response.statusCode);
});
```

## Classes

### Agent

Manages connection pooling for HTTP clients.

#### Constructor

```javascript
new http.Agent(options)
```

**Parameters:**

- `options` (Object): Optional agent configuration

**Example:**

```javascript
const agent = new http.Agent();
const req = http.request("http://example.com", {
    agent: agent
});
```

#### Methods

##### agent.destroy()

Closes the agent and releases all resources.

```javascript
agent.destroy();
```

### IncomingMessage

Represents an HTTP response. Extends EventEmitter.

#### Properties

- `ok` (boolean): `true` if status code is 200-299
- `statusCode` (number): HTTP status code
- `statusMessage` (string): HTTP status message (e.g., "200 OK")
- `headers` (Object): Response headers (lowercase keys)
- `rawHeaders` (Array): Raw headers as alternating key-value pairs
- `httpVersion` (string): HTTP protocol version (e.g., "1.1")
- `complete` (boolean): Whether the response has been fully received
- `raw` (Object): Raw Go response object

**Example:**

```javascript
http.get("http://example.com", (response) => {
    console.println("OK:", response.ok);
    console.println("Status Code:", response.statusCode);
    console.println("Status Message:", response.statusMessage);
    console.println("Content-Type:", response.headers["content-type"]);
    console.println("HTTP Version:", response.httpVersion);
});
```

#### Methods

##### response.json()

Parses the response body as JSON.

**Returns:** Object - Parsed JSON object or throws on error

**Example:**

```javascript
http.get("http://example.com/api", (response) => {
    const data = response.json();
    console.println("Message:", data.message);
});
```

##### response.text([encoding])

Reads the response body as a string.

**Parameters:**
- `encoding` (string): Character encoding (default: "utf-8")

**Returns:** string - Response body text or empty string on error

**Example:**

```javascript
http.get("http://example.com", (response) => {
    const body = response.text();
    console.println("Body:", body);
});
```

##### response.readBody([encoding])

Reads the response body as a string. (Alias for internal use)

**Parameters:**
- `encoding` (string): Character encoding (default: "utf-8")

**Returns:** string

##### response.readBodyBuffer()

Reads the response body as a buffer.

**Returns:** Uint8Array - Response body as binary data

##### response.setTimeout(msecs[, callback])

Sets a timeout for the response.

**Parameters:**
- `msecs` (number): Timeout in milliseconds
- `callback` (Function): Optional callback when timeout occurs

**Returns:** this

##### response.close()

Closes the response body. This is automatically called after the response is processed.

**Example:**

```javascript
response.close();
```

### ClientRequest

Represents an HTTP request. Extends EventEmitter.

#### Events

##### 'response'

Emitted when the response is received.

**Callback Parameters:**
- `response` (Response): The HTTP response object

```javascript
req.on('response', (response) => {
    console.println("Status:", response.statusCode);
});
```

##### 'error'

Emitted when an error occurs.

**Callback Parameters:**
- `error` (Error): The error object

```javascript
req.on('error', (err) => {
    console.println("Error:", err.message);
});
```

##### 'end'

Emitted when the request has completed.

```javascript
req.on('end', () => {
    console.println("Request completed");
});
```

#### Methods

##### request.setHeader(name, value)

Sets a single header value.

**Parameters:**
- `name` (string): Header name
- `value` (string | number): Header value

**Returns:** this

**Example:**

```javascript
req.setHeader('Content-Type', 'application/json');
req.setHeader('X-Custom-Header', 'value');
```

##### request.getHeader(name)

Gets a header value.

**Parameters:**
- `name` (string): Header name (case-insensitive)

**Returns:** string | undefined - The header value or undefined if not set

**Example:**

```javascript
const contentType = req.getHeader('Content-Type');
console.println('Content-Type:', contentType);
```

##### request.removeHeader(name)

Removes a header.

**Parameters:**
- `name` (string): Header name (case-insensitive)

**Returns:** this

**Example:**

```javascript
req.removeHeader('X-Custom-Header');
```

##### request.hasHeader(name)

Checks if a header exists.

**Parameters:**
- `name` (string): Header name (case-insensitive)

**Returns:** boolean - `true` if the header exists

**Example:**

```javascript
if (req.hasHeader('Content-Type')) {
    console.println('Content-Type is set');
}
```

##### request.getHeaders()

Gets all headers as an object.

**Returns:** Object - All headers with original case names

**Example:**

```javascript
const headers = req.getHeaders();
console.println('Headers:', JSON.stringify(headers));
```

##### request.getHeaderNames()

Gets all header names.

**Returns:** Array<string> - Array of header names with original case

**Example:**

```javascript
const names = req.getHeaderNames();
console.println('Header names:', names.join(', '));
```

##### request.write(chunk[, encoding][, callback])

Writes data to the request body.

**Parameters:**
- `chunk` (string | Buffer | Uint8Array): Data to write
- `encoding` (string): Character encoding (default: "utf-8")
- `callback` (Function): Optional callback when write completes

**Returns:** boolean - `true` if successful

**Example:**

```javascript
req.write('{"message": "Hello, ');
req.write('World!"}');
```

##### request.end([data[, encoding]][, callback])

Finishes sending the request.

**Parameters:**
- `data` (string | Buffer | Uint8Array): Optional final data to write
- `encoding` (string): Character encoding
- `callback` (Function): Optional callback that receives the response

**Returns:** this

**Example:**

```javascript
// End without data
req.end();

// End with data
req.end('{"message": "Hello"}');

// End with callback
req.end((response) => {
    console.println("Status:", response.statusCode);
});
```

##### request.destroy([error])

Destroys the request.

**Parameters:**
- `error` (Error): Optional error to emit

**Example:**

```javascript
req.destroy(new Error("Request cancelled"));
```

## Complete Usage Examples

### Simple GET Request

```javascript
const http = require("/lib/http");

const url = "http://example.com?echo=Hello";
const req = http.request(url);
req.end((response) => {
    const {statusCode, statusMessage} = response;
    console.println("Status Code:", statusCode);
    console.println("Status:", statusMessage);
});
```

**Output:**
```
Status Code: 200
Status: 200 OK
```

### GET Request with Options

```javascript
const http = require("/lib/http");

const url = new URL("http://example.com?echo=Hello");
const req = http.request(url, {
    host: url.host,
    port: url.port,
    path: url.pathname + url.search,
    method: "GET",
    agent: new http.Agent(),
});

req.end();
req.on("response", (response) => {
    if (!response.ok) {
        throw new Error("Request failed with status " + response.statusCode);
    }
    const {statusCode, statusMessage} = response;
    console.println("Status Code:", statusCode);
    console.println("Status:", statusMessage);
});
```

**Output:**
```
Status Code: 200
Status: 200 OK
```

### POST Request with JSON

```javascript
const http = require("/lib/http");

const req = http.request("http://example.com/api", {
    method: "POST",
    headers: {
        "Content-Type": "application/json"
    }
});

req.on("response", (response) => {
    if (!response.ok) {
        throw new Error("Request failed with status " + response.statusCode);
    }
    const {statusCode, statusMessage} = response;
    console.println("Status Code:", statusCode);
    console.println("Status:", statusMessage);
    
    const body = response.json();
    console.println("message:" + body.message + ", reply:" + body.reply);
});

req.on("error", (err) => {
    console.println("Request error:", err.message);
});

req.write('{"message": "Hello, ');
req.end('World!"}');
```

**Output:**
```
Status Code: 200
Status: 200 OK
message:Hello, World!, reply:Received
```

### Using http.get() with Callback

```javascript
const http = require("/lib/http");

const url = "http://example.com?echo=Hi";
http.get(url, (response) => {
    console.println("Status Code:", response.statusCode);
    console.println("Status:", response.statusMessage);
});
```

**Output:**
```
Status Code: 200
Status: 200 OK
```

### Using http.get() with Event Listener

```javascript
const http = require("/lib/http");

const url = "http://example.com?echo=Hi";
const req = http.get(url);
req.on("response", (response) => {
    console.println("Status Code:", response.statusCode);
    console.println("Status:", response.statusMessage);
});
```

**Output:**
```
Status Code: 200
Status: 200 OK
```

### GET Request with Custom Headers

```javascript
const http = require("/lib/http");

const url = "http://example.com?echo=Hi";
const options = {
    headers: {
        "X-Test-Header": "TestValue"
    }
};

http.get(url, options, (response) => {
    console.println("Status Code:", response.statusCode);
    console.println("Status:", response.statusMessage);
    console.println("X-Test-Header:", response.headers["X-Test-Header"]);
});
```

**Output:**
```
Status Code: 200
Status: 200 OK
X-Test-Header: TestValue
```

### Reading Response Body and Headers

```javascript
const http = require("/lib/http");

const options = {
    url: new URL("http://example.com?echo=Hi"),
    headers: {
        "X-Test-Header": "TestValue"
    }
};

http.get(options, (response) => {
    const {statusCode, statusMessage} = response;
    console.println("Status Code:", statusCode);
    console.println("Status:", statusMessage);
    console.println("Body:", response.text());
    
    // Access response headers (note: header keys are lowercase)
    const contentLength = response.headers["content-length"];
    const contentType = response.headers["content-type"];
    const dateHeader = response.headers["date"];
    
    console.println("Content-Length:", contentLength);
    console.println("Content-Type:", contentType);
    console.println("Date:", dateHeader);
});
```

**Output:**
```
Status Code: 200
Status: 200 OK
Body: Hi
Content-Length: 2
Content-Type: text/plain; charset=utf-8
Date: Fri, 12 Dec 2025 12:20:01 GMT
```

### Handling 404 Errors

```javascript
const http = require("/lib/http");

const url = "http://example.com/notfound";
http.get(url, (response) => {
    console.println("Status Code:", response.statusCode);
    console.println("Status:", response.statusMessage);
    
    if (!response.ok) {
        console.println("Request failed!");
    }
});
```

**Output:**
```
Status Code: 404
Status: 404 Not Found
Request failed!
```

## Notes

- The module uses Go's standard `net/http` package for HTTP operations
- Response bodies are automatically closed after being processed
- The `Agent` class manages connection pooling and reuse
- All requests are executed asynchronously using `setImmediate()`
- The `response.ok` property provides a convenient way to check for successful status codes (200-299)
- Response header keys are stored in lowercase for consistency
- Request headers set via `setHeader()` preserve their original case but are compared case-insensitively
- When using `write()` multiple times, data is accumulated before being sent
- The `IncomingMessage` class wraps Go's HTTP response and provides a Node.js-compatible interface

## API Compatibility

This module provides a subset of Node.js's `http` module API with the following key classes:

- `Agent` - Connection pooling manager
- `ClientRequest` - Outgoing HTTP request (extends EventEmitter)
- `IncomingMessage` - HTTP response wrapper (extends EventEmitter)

Key methods:
- `http.request()` - Create an HTTP request
- `http.get()` - Convenience method for GET requests
- `request.setHeader()`, `getHeader()`, `removeHeader()`, `hasHeader()` - Header management
- `request.write()`, `end()` - Send request data
- `response.json()`, `text()` - Parse response body

## Dependencies

- Go's standard `net/http` package
- JSH EventEmitter (built-in)
