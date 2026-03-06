# MQTT Module

A JSH native module that provides an MQTT client. Based on the Eclipse Paho MQTT Go client.

## Installation

```javascript
const mqtt = require("/lib/mqtt");
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

### subscribe(topic)

Subscribes to the specified topic.

**Parameters:**
- `topic` (string): Topic to subscribe to

**Returns:** void

**Example:**

```javascript
client.subscribe('test/topic');
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

### 'message'

Emitted when a message is received on a subscribed topic.

**Callback Parameters:**
- `msg` (Object): Message object
  - `topic` (string): Topic the message was received on
  - `payload` (string): Message content

```javascript
client.on('message', (msg) => {
    console.println("Message received on topic:", msg.topic, "payload:", msg.payload);
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

Emitted when a message has been published.

**Callback Parameters:**
- `topic` (string): Published topic
- `reason` (number): Publish result code (0: success)

```javascript
client.on('published', (topic, reason) => {
    console.println("Published to:", topic, "Payload:", reason);
});
```

## Complete Usage Examples

### Basic Pub/Sub Example

```javascript
const mqtt = require("/lib/mqtt");

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
    client.subscribe('test/topic');
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
    console.println("Message received on topic:", msg.topic, "payload:", msg.payload);
    
    // Close connection after receiving message
    setTimeout(() => {
        client.close();
    }, 500);
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
```

### Configuration Check Example

```javascript
const mqtt = require("/lib/mqtt");

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

## Notes

- The client automatically attempts to reconnect when the connection is lost.
- The reconnection interval can be configured with the `connectRetryDelay` option.
- QoS is currently fixed at 1 for subscriptions and 0 for publishes.
- The client maintains an internal event loop, so the program will not terminate until the connection is closed.

## Dependencies

- [Eclipse Paho MQTT Go Client](https://github.com/eclipse/paho.golang)
- JSH EventEmitter
