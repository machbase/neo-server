# WebSocket Module

A JSH native module that provides WebSocket client functionality. Based on the Gorilla WebSocket library.

## Installation

```javascript
const { WebSocket } = require("/lib/ws");
```

## Classes

### WebSocket

A WebSocket client for bidirectional communication with WebSocket servers. Extends EventEmitter.

#### Constructor

```javascript
new WebSocket(url)
```

**Parameters:**

- `url` (string): WebSocket server URL (must start with `ws://` or `wss://`)

**Throws:** TypeError if URL is not a string

**Example:**

```javascript
const { WebSocket } = require("/lib/ws");
const ws = new WebSocket("ws://localhost:8080");
```

#### Properties

##### ws.url

The WebSocket server URL.

**Type:** string

**Example:**

```javascript
const ws = new WebSocket("ws://localhost:8080");
console.println(ws.url); // "ws://localhost:8080"
```

##### ws.readyState

The current state of the WebSocket connection.

**Type:** number

**Possible values:**
- `WebSocket.CONNECTING` (0): Connection is being established
- `WebSocket.OPEN` (1): Connection is open and ready to communicate
- `WebSocket.CLOSING` (2): Connection is in the process of closing
- `WebSocket.CLOSED` (3): Connection is closed

**Example:**

```javascript
console.println("State:", ws.readyState);
if (ws.readyState === WebSocket.OPEN) {
    ws.send("Hello");
}
```

#### Static Constants

##### Ready States

- `WebSocket.CONNECTING = 0` - Connection is being established
- `WebSocket.OPEN = 1` - Connection is open
- `WebSocket.CLOSING = 2` - Connection is closing
- `WebSocket.CLOSED = 3` - Connection is closed

**Example:**

```javascript
console.println("CONNECTING:", WebSocket.CONNECTING); // 0
console.println("OPEN:", WebSocket.OPEN);             // 1
console.println("CLOSING:", WebSocket.CLOSING);       // 2
console.println("CLOSED:", WebSocket.CLOSED);         // 3
```

##### Message Types

- `WebSocket.TextMessage = 1` - Text message
- `WebSocket.BinaryMessage = 2` - Binary message

## Methods

### send(data)

Sends data to the WebSocket server.

**Parameters:**
- `data` (string | Buffer): Data to send

**Throws:** Error if the WebSocket is not open

**Example:**

```javascript
ws.on("open", () => {
    ws.send("Hello, Server!");
});
```

### close()

Closes the WebSocket connection.

**Example:**

```javascript
ws.close();
```

## Events

WebSocket extends EventEmitter and emits the following events:

### 'open'

Emitted when the connection is successfully established.

```javascript
ws.on("open", () => {
    console.println("WebSocket connected");
});
```

### 'close'

Emitted when the connection is closed.

```javascript
ws.on("close", () => {
    console.println("WebSocket closed");
});
```

### 'message'

Emitted when a message is received from the server.

**Callback Parameters:**
- `event` (Object): Message event object
  - `data` (string | Buffer): Message data
  - `type` (number): Message type (1 for text, 2 for binary)

```javascript
ws.on("message", (evt) => {
    console.println("Received:", evt.data);
});
```

### 'error'

Emitted when an error occurs.

**Callback Parameters:**
- `error` (Error): Error object

```javascript
ws.on("error", (err) => {
    console.println("Error:", err.message);
});
```

## Complete Usage Examples

### Basic Connection

```javascript
const { WebSocket } = require("/lib/ws");

const ws = new WebSocket("ws://localhost:8080");

ws.on("error", (err) => {
    console.log("websocket error: " + err.message);
});

ws.on("open", () => {
    console.log("websocket open");
    setTimeout(() => { ws.close() }, 500);
});

ws.on("close", () => {
    console.log("websocket closed");
});
```

**Output:**
```
INFO  websocket open
INFO  websocket closed
```

### Simple Close

```javascript
const { WebSocket } = require("/lib/ws");

const ws = new WebSocket("ws://localhost:8080");

ws.on("open", () => {
    console.println("websocket open");
    ws.close();
});

ws.on("close", () => {
    console.println("websocket closed");
});
```

**Output:**
```
websocket open
websocket closed
```

### Send and Receive Messages

```javascript
const { WebSocket } = require("/lib/ws");

const ws = new WebSocket("ws://localhost:8080");

ws.on("error", (err) => {
    console.log("websocket error: " + err);
});

ws.on("close", (evt) => {
    console.log("websocket closed");
});

ws.on("open", () => {
    console.log("websocket open");
    for (let i = 0; i < 3; i++) {
        ws.send("test message " + i);
    }
});

ws.on("message", (evt) => {
    console.println(evt.data);
});

setTimeout(() => { ws.close(); }, 100);
```

**Output:**
```
INFO  websocket open
test message 0
test message 1
test message 2
INFO  websocket closed
```

### Multiple Event Listeners

```javascript
const { WebSocket } = require("/lib/ws");

const ws = new WebSocket("ws://localhost:8080");

const onMessage = (m) => {
    console.println("got: " + m.data);
};

// Register the same handler twice
ws.on("message", onMessage);
ws.addEventListener("message", onMessage);

ws.on("open", () => {
    ws.send("trigger message");
    setTimeout(() => { ws.close(); }, 500);
});

ws.on("close", () => {
    console.println("websocket closed");
});
```

**Output:**
```
got: trigger message
got: trigger message
websocket closed
```

### Handling Connection Errors

```javascript
const { WebSocket } = require("/lib/ws");

// Try to connect to a non-existent server
const ws = new WebSocket("ws://127.0.0.1:9999");

ws.on("error", (err) => {
    console.println("err:", err.message);
});
```

**Output:**
```
err: dial tcp 127.0.0.1:9999: connect: connection refused
```

### Sending Without Connection

```javascript
const { WebSocket } = require("/lib/ws");

const ws = new WebSocket("ws://127.0.0.1:9999");

ws.on("error", (err) => {
    console.println("err:", err.message);
});

setTimeout(() => {
    // This will fail because the connection was never established
    ws.send("test message");
}, 500);
```

**Output:**
```
err: dial tcp 127.0.0.1:9999: connect: connection refused
err: websocket is not open
```

### Checking Ready State

```javascript
const { WebSocket } = require("/lib/ws");

const ws = new WebSocket("ws://localhost:8080");

console.println("Initial state:", ws.readyState); // CONNECTING (0)

ws.on("open", () => {
    console.println("Open state:", ws.readyState); // OPEN (1)
    ws.send("Hello");
    ws.close();
});

ws.on("close", () => {
    console.println("Closed state:", ws.readyState); // CLOSED (3)
});
```

### Echo Client Example

```javascript
const { WebSocket } = require("/lib/ws");

const ws = new WebSocket("ws://echo.websocket.org");

ws.on("open", () => {
    console.println("Connected to echo server");
    ws.send("Hello, Echo!");
});

ws.on("message", (evt) => {
    console.println("Echo received:", evt.data);
    ws.close();
});

ws.on("close", () => {
    console.println("Connection closed");
});

ws.on("error", (err) => {
    console.println("Error:", err.message);
});
```

## Notes

- The WebSocket connection is established asynchronously using `setImmediate()`
- The connection automatically starts when the WebSocket object is created
- Messages are received in a background goroutine and dispatched as events
- The `readyState` property reflects the current connection state
- Both `on()` and `addEventListener()` can be used to register event listeners
- The module currently supports text and binary message types
- Closing a WebSocket that is already closed has no effect
- Attempting to send data on a closed or non-open WebSocket will emit an error event

## Dependencies

- [Gorilla WebSocket](https://github.com/gorilla/websocket)
- JSH EventEmitter
