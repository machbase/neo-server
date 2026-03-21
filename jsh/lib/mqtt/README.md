# MQTT Module

A JSH native module that provides an MQTT client. Based on the Eclipse Paho MQTT Go client.

## Installation

```javascript
const mqtt = require("mqtt");
```

## Classes

### Client

A client that connects to an MQTT broker and can send and receive messages.

#### Constructor

```javascript
new mqtt.Client(options)
```

**Parameters:**

- `options` (Object): Client configuration object
  - `servers` (Array<string>): List of MQTT broker addresses (e.g., `["tcp://127.0.0.1:1883"]`)
  - `username` (string): Authentication username
  - `password` (string): Authentication password
  - `keepAlive` (number): Keep-alive interval in seconds
  - `cleanStartOnInitialConnection` (boolean): Clean start flag on initial connection
  - `connectRetryDelay` (number): Reconnection delay in milliseconds
  - `connectTimeout` (number): Connection timeout in milliseconds

**Example:**

```javascript
const client = new mqtt.Client({
    servers: ["tcp://127.0.0.1:1883"],
    username: "user",
    password: "pass",
    keepAlive: 60,
    cleanStartOnInitialConnection: true,
    connectRetryDelay: 2000,
    connectTimeout: 10000,
});
```

#### Properties

##### client.config

An object containing client configuration information.

- `serverUrls`: List of server URLs
- `connectUsername`: Connection username
- `connectPassword`: Connection password (byte array)
- `keepAlive`: Keep-alive interval
- `reconnectBackoff(n)`: Function that calculates the backoff time for the nth reconnection attempt
- `cleanStartOnInitialConnection`: Clean start flag
- `connectTimeout`: Connection timeout

## Methods

### publish(topic, message)

Publishes a message to the specified topic.

**Parameters:**
- `topic` (string): Topic to publish to
- `message` (string | Buffer): Message to publish

**Returns:** void

**Example:**

```javascript
client.publish('test/topic', 'Hello, MQTT!');
```

### subscribe(topic, options)

Subscribes to the specified topic.

**Parameters:**
- `topic` (string): Topic to subscribe to
- `options` (Object, optional): Subscription options

**Options:**
- `qos` (number): QoS level. Default is `1`
- `retainHandling` (number): MQTT v5 retain handling mode
- `noLocal` (boolean): Suppress messages published by the same client
- `retainAsPublished` (boolean): Preserve the retain flag from the broker
- `properties` (Object): MQTT v5 subscribe properties

`options.properties` fields:

- `subscriptionIdentifier` (number): Subscription identifier
- `user` (Object): User properties as `key: value`

**Returns:** void

**Example:**

```javascript
client.subscribe('test/topic', {
    qos: 0,
    properties: {
        subscriptionIdentifier: 7,
        user: {
            source: 'example',
        },
    },
});
```

### unsubscribe(topic, options)

Unsubscribes from the specified topic.

**Parameters:**
- `topic` (string): Topic to unsubscribe from
- `options` (Object, optional): Unsubscribe options

**Options:**
- `properties` (Object): MQTT v5 unsubscribe properties

`options.properties` fields:

- `user` (Object): User properties as `key: value`

**Returns:** void

**Example:**

```javascript
client.unsubscribe('test/topic', {
    properties: {
        user: {
            source: 'example',
        },
    },
});
```

### close()

Closes the client connection.

**Example:**

```javascript
client.close();
```

## Events

Client extends EventEmitter and emits the following events:

### 'open'

Emitted when successfully connected to the broker.

```javascript
client.on('open', () => {
    console.println("Connected");
});
```

### 'close'

Emitted when the connection to the broker is closed.

```javascript
client.on('close', () => {
    console.println("Disconnected");
});
```

### 'error'

Emitted when an error occurs.

**Callback Parameters:**
- `err` (Error): Error object

```javascript
client.on('error', (err) => {
    console.println("Error:", err.message);
});
```

Operations like `publish()`, `subscribe()`, and `unsubscribe()` emit `error` if the client is not connected or has already been closed.

### 'message'

Emitted when a message is received on a subscribed topic.

**Callback Parameters:**
- `msg` (Object): Message object
  - `topic` (string): Topic the message was received on
    - `payload` (Buffer): Binary-safe payload buffer
    - `payloadText` (string): UTF-8 decoded payload text convenience field
    - `properties` (Object): MQTT v5 publish properties when present

`msg.properties` fields:

- `payloadFormat` (number): Payload format indicator
- `messageExpiry` (number): Expiry interval
- `contentType` (string): Content type
- `responseTopic` (string): Response topic
- `correlationData` (Buffer): Binary-safe correlation data
- `topicAlias` (number): Topic alias when present
- `subscriptionIdentifier` (number): Subscription identifier when present
- `user` (Object): User properties as `key: value` or `key: string[]` for duplicate keys

```javascript
client.on('message', (msg) => {
    console.println("Message received on topic:", msg.topic, "payload:", msg.payloadText);
});
```

For binary payloads, inspect `msg.payload` directly:

```javascript
client.on('message', (msg) => {
    console.println('Payload is buffer:', Buffer.isBuffer(msg.payload));
    console.println('Payload bytes:', Array.from(msg.payload).join(','));
});
```

MQTT v5 publish properties are available as `msg.properties`:

```javascript
client.on('message', (msg) => {
    console.println('Content type:', msg.properties.contentType);
    console.println('Response topic:', msg.properties.responseTopic);
    console.println('Correlation data:', msg.properties.correlationData.toString());
    console.println('User source:', msg.properties.user.source);
});
```

### 'subscribed'

Emitted when successfully subscribed to a topic.

**Callback Parameters:**
- `topic` (string): Subscribed topic
- `reason` (number): Subscription result code (1: success)

```javascript
client.on('subscribed', (topic, reason) => {
    console.println("Subscribed to:", topic, "reason:", reason);
});
```

### 'published'

Emitted when a publish request completes.

**Callback Parameters:**
- `topic` (string): Published topic
- `reason` (number): Publish result code returned by the broker client

```javascript
client.on('published', (topic, reason) => {
    console.println("Published to:", topic, "Payload:", reason);
});
```

### 'unsubscribed'

Emitted when successfully unsubscribed from a topic.

**Callback Parameters:**
- `topic` (string): Unsubscribed topic
- `reason` (number): Unsubscribe result code (0: success)

```javascript
client.on('unsubscribed', (topic, reason) => {
    console.println("Unsubscribed from:", topic, "reason:", reason);
});
```

## Complete Usage Examples

### Basic Pub/Sub Example

```javascript
const mqtt = require("mqtt");

const client = new mqtt.Client({
    servers: ["tcp://127.0.0.1:1883"],
    username: "user",
    password: "pass",
    keepAlive: 60,
    cleanStartOnInitialConnection: true,
    connectRetryDelay: 2000,
    connectTimeout: 10000,
});

// On connection success
client.on('open', () => {
    console.println("Connected");
    client.subscribe('test/topic', {
        qos: 0,
        properties: {
            subscriptionIdentifier: 7,
        },
    });
});

// Error handling
client.on('error', (err) => {
    console.println("Error:", err.message);
});

// On connection close
client.on('close', () => {
    console.println("Disconnected");
});

// Message received
client.on('message', (msg) => {
    console.println("Message received on topic:", msg.topic, "payload:", msg.payloadText);
    client.unsubscribe(msg.topic, {
        properties: {
            user: {
                source: 'example',
            },
        },
    });
});

// On successful subscription
client.on('subscribed', (topic, reason) => {
    console.println("Subscribed to:", topic, "reason:", reason);
    // Publish message
    client.publish('test/topic', 'Hello, MQTT!');
});

// On successful publish
client.on('published', (topic, reason) => {
    console.println("Published to:", topic, "Payload:", reason);
});

// On successful unsubscription
client.on('unsubscribed', (topic, reason) => {
    console.println("Unsubscribed from:", topic, "reason:", reason);
    setTimeout(() => {
        client.close();
    }, 500);
});
```

### Binary Payload Example

```javascript
client.on('subscribed', (topic) => {
    client.publish(topic, new Uint8Array([0, 1, 2, 255]));
});

client.on('message', (msg) => {
    console.println('Payload is buffer:', Buffer.isBuffer(msg.payload));
    console.println('Payload bytes:', Array.from(msg.payload).join(','));
});
```

### Configuration Check Example

```javascript
const mqtt = require("mqtt");

const client = new mqtt.Client({
    servers: ["tcp://127.0.0.1:1883"],
    username: "user",
    password: "pass",
    keepAlive: 60,
    cleanStartOnInitialConnection: true,
    connectRetryDelay: 2000,
    connectTimeout: 10000,
});

// Print configuration information
console.println("SERVERS:", client.config.serverUrls);
console.println("USERNAME:", client.config.connectUsername);
console.println("PASSWORD:", client.config.connectPassword);
console.println("KEEPALIVE:", client.config.keepAlive);
console.println("RECONNECT_BACKOFF:", client.config.reconnectBackoff(1));
console.println("CLEAN_START:", client.config.cleanStartOnInitialConnection);
console.println("CONNECT_TIMEOUT:", client.config.connectTimeout);

client.close();
```

## `mqtt_pub` Usage

The shell command `mqtt_pub` publishes one message and exits after the publish request completes.

When `--qos 1` or `--qos 2` is used, `mqtt_pub` waits for the corresponding broker acknowledgment before closing the connection. With `--qos 0`, there is no broker acknowledgment and the command exits after the client publish call returns.

Supported input modes:

- `--message <text>`: publish an inline text payload
- `--file <path>`: publish the content of a file

Options:

- `--qos <0|1|2>`: MQTT publish QoS level. Default is `0`.

Examples:

```sh
# Publish an inline message
mqtt_pub --broker 127.0.0.1:5653 --topic test/topic --message "hello-mqtt"
```

```sh
# Publish and wait for PUBACK before exiting
mqtt_pub --broker 127.0.0.1:5653 --topic test/topic --qos 1 --message "hello-mqtt"
```

```sh
# Publish the content of a file relative to the current working directory
mqtt_pub --broker 127.0.0.1:5653 --topic test/topic --file payload.txt
```

```sh
# Enable debug logs while publishing
mqtt_pub --debug --broker tcp://127.0.0.1:5653 --topic test/topic --message "hello-mqtt"
```

## Notes

- The client automatically attempts to reconnect when the connection is lost.
- The reconnection interval can be configured with the `connectRetryDelay` option.
- Subscriptions default to QoS 1 unless a different value is provided in `subscribe()` options.
- Publishes honor the `qos` option passed to `publish()` or `mqtt_pub --qos`.
- The client maintains an internal event loop, so the program will not terminate until the connection is closed.

## Dependencies

- [Eclipse Paho MQTT Go Client](https://github.com/eclipse/paho.golang)
- JSH EventEmitter
