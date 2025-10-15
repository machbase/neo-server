# Machbase Neo JavaScript MQTT Module

## Client

The MQTT client.

**Usage example**

```js
const mqtt = require("@jsh/mqtt");
const client = new mqtt.Client({ serverUrls: ["tcp://127.0.0.1:1236"] });
try {
    client.onConnect = connAck => { println("connected."); }
    client.onConnectError = err => { println("connect error", err); }
    client.connect({timeout: 10*1000});
    client.publish("test/topic", "Hello, MQTT!", 0)
} catch(e) {
    console.log("Error:", e);
} finally {
    client.disconnect({waitForEmptyQueue:true});
}
```

**Creation**

| Constructor             | Description                          |
|:------------------------|:----------------------------------------------|
| new Client(*options*)   | Instantiates a MQTT client object with an options |

**Options**

| Option                             | Type         | Default        | Description         |
|:-----------------------------------|:-------------|:---------------|:--------------------|
| serverUrls                         | String[]     |                | server addresses    |
| keepAlive                          | Number       | `10`           |                     |
| cleanStart                         | Boolean      | `true`         | clean session       |
| username                           | String       |                |                     |
| password                           | String       |                |                     |
| clientID                           | String       | random id      |                     |
| debug                              | Boolean      | `false`        |                     |
| sessionExpiryInterval              | Number       | `60`           |                     |
| connectRetryDelay                  | Number       | `10`           |                     |
| connectTimeout                     | Number       | `10`           |                     |
| packetTimeout                      | Number       | `5`            |                     |
| queue                              | String       | `memory`       |                     |

### connect()

**Syntax**

```js
connect(opts)
```

**Parameters**

- `opts` `Object`

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| timeout            | Number     | connection timeout in milliseconds |

**Return value**

None.

### disconnect()

**Syntax**

```js
disconnect(opts)
```

**Parameters**

- `opts` `Object`

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| waitForEmptyQueue  | Boolean    |                       |
| timeout            | Number     | disconnect wait timeout in milliseconds |

**Return value**

None.

### subscribe()

**Syntax**

```js
subscribe(opts)
```

**Parameters**

- `opts` `Object` *SubscriptionOption*

**SubscriptionOption**

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| **subscriptions**  | Object[]   | Array of *Subscription* |
| properties         | Object     | *Properties*            |

**Subscription**

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| **topic**          | String     |                       |
| qos                | Number     | `0`, `1`, `2`         |
| retainHandling     | Number     |                       |
| noLocal            | Boolean    |                       |
| retainAsPublished  | Boolean    |                       |

**Properties**

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| user               | Object     | key-value properties  |

**Return value**

None.

**Usage example**

```js
const topicName = 'sensor/temperature';
client.subscribe({subscriptions:[{topic:topicName, qos:0}]});
```

### unsubscribe()

**Syntax**

```js
unsubscribe(opts)
```

**Parameters**

- `opts` `Object` *UnsubscribeOption*

**UnsubscribeOption**

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| **topics**         | String[]   | Array of topics to unsubscribe |
| properties         | Object     | *Properties*          |

**Properties**

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| user               | Object     | user key-value properties |

**Return value**

None.

**Usage example**

```js
const topicName = 'sensor/temperature';
client.unsubscribe({topics:[topicName]});
```

### publish()

**Syntax**

```js
publish(opts, payload)
```

**Parameters**

- `opts` `Object` *PublishOptions*
- `payload` `String` or `Number`

**PublishOptions**

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| **topic**          | String     |                       |
| qos                | Number     | `0`, `1`, `2`         |
| packetID           | String     |                       |
| retain             | Boolean    |                       |
| properties         | Object     |                       |

**Return value**

- `Object`

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| reasonCode         | Number     |                       |
| properties         | Object     |                       |

**Usage example**

```js
let r = client.publish('sensor/temperature', 'Hello World', 1)
console.log(r.reasonCode)
```

### onMessage callback

Callback function that receives a message.

**Syntax**

```js
function (msg) { }
```

- `msg` `Object` Message

**Message**

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| packetID           | Number     |                       |
| topic              | String     |                       |
| qos                | Number     | 0, 1, 2               |
| retain             | Boolean    |                       |
| payload            | Object     | Payload               |
| properties         | Object     | Properties            |

**Payload**

- `msg.payload.bytes()`
- `msg.payload.string()`

**Properties**

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| correlationData    | byte[]     |                       |
| contentType        | String     |                       |
| responseTopic      | String     |                       |
| payloadFormat      | Number     | or undefined          |
| messageExpiry      | Number     | or undefined          |
| subscriptionIdentifier | Number | or undefined          |
| topicAlias         | Number     | or undefined          |
| user               | Object     | user properties       |

### onConnect callback

On connect callback.

**Syntax**

```js
function (ack) { }
```

**Parameters**

- `ack` `Object`

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| sessionPresent     | Boolean    |                       |
| reasonCode         | Number     |                       |
| properties         | Object     | Properties            |

**Properties**

| Property              | Type       | Description           |
|:----------------------|:-----------|:----------------------|
| reasonString          | String     |                       |
| reasonInfo            | String     |                       |
| assignedClientID      | String     |                       |
| authMethod            | String     |                       |
| serverKeepAlive       | Number     | or undefined          |
| sessionExpiryInterval | Number     | or undefined          |
| user                  | Object     |                       |

**Return value**

None.

### onConnectError callback

On connect error callback.

**Syntax**

```js
function (err) { }
```

**Parameters**

- `error` `String`

**Return value**

None.

### onDisconnect callback

On disconnect callback

**Syntax**

```js
function (disconn) { }
```

**Parameters**

- `disconn` `Object`

**Return value**

None.

### onClientError callback

On client error callback

**Syntax**

```js
function (err) { }
```

**Parameters**

- `err` `String`

**Return value**

None.