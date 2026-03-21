# NATS Module

A JSH native module that provides a NATS client based on the Go NATS client.

## Installation

```javascript
const nats = require("nats");
```

## Classes

### Client

A client that connects to a NATS server and can publish and subscribe to subjects.

#### Constructor

```javascript
new nats.Client(options)
```

**Parameters:**

- `options` (Object): Client configuration object
  - `servers` (Array<string>): List of NATS server addresses such as `["nats://127.0.0.1:4222"]`
  - `name` (string): Connection name
  - `user` (string): Authentication user
  - `password` (string): Authentication password
  - `token` (string): Authentication token
  - `allowReconnect` (boolean): Enable reconnect handling
  - `maxReconnect` (number): Maximum reconnect attempts
  - `reconnectWait` (number): Reconnect wait in milliseconds
  - `timeout` (number): Connect timeout in milliseconds

## Methods

### publish(subject, message, options)

Publishes a message to the specified subject.

- `options.reply` (string): Reply subject used for request/reply patterns

For request/reply patterns, subscribe to a reply subject first, then publish with `options.reply` set to that subject.

### subscribe(subject, options)

Subscribes to the specified subject. Use `options.queue` for queue subscriptions.

### close()

Closes the client connection.

## Events

Client extends EventEmitter and emits the following events:

### `open`

Emitted when the connection is established.

### `close`

Emitted when the client is closed.

### `error`

Emitted when an error occurs.

### `message`

Emitted when a subscribed message is received.

The message object contains:

- `topic` (string): Alias of the subject for compatibility with MQTT-style handlers
- `subject` (string): NATS subject
- `reply` (string): Reply subject if the message expects a response
- `payload` (string): Message payload

### `subscribed`

Emitted after a successful subscribe request. The reason code is `1` on success.

### `published`

Emitted after a successful publish request. The reason code is `0` on success.

## Request/Reply Example

```javascript
const nats = require("nats");

const handler = new nats.Client({
  servers: ["nats://127.0.0.1:4222"],
});
handler.on("open", () => {
  handler.subscribe("request.subject");
});
handler.on("message", (msg) => {
  handler.publish(msg.reply, "pong");
});

const requester = new nats.Client({
  servers: ["nats://127.0.0.1:4222"],
});
requester.on("open", () => {
  requester.subscribe("reply.subject");
  requester.publish("request.subject", "ping", { reply: "reply.subject" });
});
requester.on("message", (msg) => {
  console.println(msg.payload);
  requester.close();
  handler.close();
});
```

## `nats_pub` Request Mode

The shell command `nats_pub` supports two reply modes:

- `--reply <subject>`: use an explicit reply subject
- `--request`: generate a temporary `_INBOX.*` reply subject automatically and wait for one response

Examples:

```javascript
// Explicit reply subject
nats_pub --broker 127.0.0.1:4222 --topic request.subject --message ping --reply reply.subject --timeout 3000
```

```javascript
// Auto-generated inbox subject
nats_pub --broker 127.0.0.1:4222 --topic request.subject --message ping --request --timeout 3000
```