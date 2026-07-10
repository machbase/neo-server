# DBus module for JSH

This module provides Linux-only DBus integration for JSH scripts.

## Module load

```javascript
const dbus = require("dbus");
const conn = new dbus.Connection({ busType: dbus.BusType.Session });
```

`busType` values:
- `dbus.BusType.Session`
- `dbus.BusType.System`

If the runtime OS is not Linux, creating a connection fails.

## Main APIs

### Connection

- `new dbus.Connection(options)`
- `conn.close()`
- `conn.object(destination, path)`
- `conn.call(request)`
- `conn.getProperty(request)`
- `conn.setProperty(request)`
- `conn.introspect(request)`
- `conn.subscribeSignal(request)`
- `conn.unsubscribeSignal(request)`
- `conn.watchName(name)`
- `conn.unwatchName(name)`
- `conn.getNameOwner(name)`

Events:
- `signal`: raw DBus signal payload
- `name-owner-changed`: owner change payload for watched names

### ObjectProxy

Created with `conn.object(destination, path)`.

- `obj.call(method, ...args)`
- `obj.getProperty(name, interfaceName)`
- `obj.setProperty(name, value, interfaceName)`
- `obj.get(name, interfaceName)`
- `obj.set(name, value, interfaceName)`
- `obj.introspect()`
- `obj.subscribeSignal(member, interfaceName)`
- `obj.unsubscribeSignal(member, interfaceName)`

## Request and response shapes

### conn.call(request)

Request:

```js
{
  destination: "org.example.Service",
  path: "/org/example/Object",
  method: "org.example.Interface.Method",
  args: [1, "x"],
  flags: 0
}
```

Typed args for strict DBus method signatures:

- JavaScript numbers can be ambiguous for strict integer DBus types.
- When an exact DBus type is required, pass each argument as a string in `"type:value"` format.
- Examples: `"uint16:123"`, `"int32:-7"`, `"bool:true"`, `"objectpath:/org/freedesktop/DBus"`.

Supported `type:value` hints:

- `byte`, `uint8`, `uint16`, `uint32`, `uint64`
- `int16`, `int32`, `int64`
- `float32`, `float64`, `double`
- `bool`, `string`
- `objectpath`, `path`
- `signature`

Behavior notes:

- Strings without a type prefix are passed as plain strings.
- Unknown type prefixes (for example, `"custom:123"`) are passed as-is.
- If parsing fails for a recognized type, the call throws an error.

Response:

```js
{
  destination: "org.example.Service",
  path: "/org/example/Object",
  method: "org.example.Interface.Method",
  body: [ ... ]
}
```

### conn.getProperty(request)

Request:

```js
{
  destination: "org.example.Service",
  path: "/org/example/Object",
  interface: "org.example.Interface",
  name: "Mode"
}
```

Response:

```js
{
  signature: "s",
  value: "AUTO"
}
```

### conn.setProperty(request)

Request:

```js
{
  destination: "org.example.Service",
  path: "/org/example/Object",
  interface: "org.example.Interface",
  name: "Mode",
  value: "MANUAL"
}
```

### conn.introspect(request)

Request:

```js
{
  destination: "org.example.Service",
  path: "/org/example/Object"
}
```

Response (simplified):

```js
{
  name: "/org/example/Object",
  interfaces: [
    {
      name: "org.example.Interface",
      methods: [ { name, args, annotations } ],
      signals: [ { name, args, annotations } ],
      properties: [ { name, type, access, annotations } ],
      annotations: [ { name, value } ]
    }
  ],
  children: [ { name } ]
}
```

### conn.getNameOwner(name)

Request:

```js
"org.example.Service"
```

Response:

```js
{
  name: "org.example.Service",
  owner: ":1.42",
  hasOwner: true
}
```

If the name currently has no owner:

```js
{
  name: "org.example.Service",
  owner: "",
  hasOwner: false
}
```

## Examples for JSH developers

### 1) Basic method call

```javascript
const dbus = require("dbus");

const conn = new dbus.Connection();
const svc = conn.object("org.freedesktop.DBus", "/org/freedesktop/DBus");

const names = svc.call("org.freedesktop.DBus.ListNames");
console.println("name count:", names.body[0].length);

conn.close();
```

### 2) Property read and write

```javascript
const dbus = require("dbus");

const conn = new dbus.Connection();
const dev = conn.object("com.plc.manufacture.Service", "/com/plc/device0");

console.println("mode:", dev.get("Mode", "com.plc.manufacture.Status"));
dev.set("Mode", "MANUAL", "com.plc.manufacture.Status");
console.println("mode:", dev.get("Mode", "com.plc.manufacture.Status"));

conn.close();
```

### 3) Introspection driven discovery

```javascript
const dbus = require("dbus");

const conn = new dbus.Connection();
const obj = conn.object("com.plc.manufacture.Service", "/com/plc/device0");
const node = obj.introspect();

for (const iface of node.interfaces) {
  console.println("iface:", iface.name);
  for (const method of iface.methods) {
    console.println("  method:", method.name);
  }
}

conn.close();
```

### 4) Signal subscribe

```javascript
const dbus = require("dbus");

const conn = new dbus.Connection();
const obj = conn.object("com.plc.manufacture.Service", "/com/plc/device0");

obj.subscribeSignal("TemperatureChanged", "com.plc.manufacture.Interval");
conn.on("signal", (sig) => {
  if (sig.name !== "com.plc.manufacture.Interval.TemperatureChanged") {
    return;
  }
  console.println("temp changed:", sig.body[0]);
});
```

### 5) Name watch with initial owner check

```javascript
const dbus = require("dbus");

const conn = new dbus.Connection();
const name = "com.example.Worker";

const current = conn.getNameOwner(name);
console.println("current owner exists:", current.hasOwner);

conn.watchName(name);
conn.on("name-owner-changed", (evt) => {
  if (evt.name !== name) {
    return;
  }
  console.println("owner changed:", evt.oldOwner, "->", evt.newOwner);
});
```

## Error behavior notes

- Invalid `busType` throws on `new dbus.Connection(...)`.
- Missing required fields in requests throw errors.
- `conn.getNameOwner(name)` does not throw when there is no owner; it returns `{ hasOwner: false }`.
- Calling methods after `conn.close()` throws `connection not initialized`.

## Development/testing notes

- DBus tests are Linux-only.
- Integration tests live in `jsh/lib/dbus/dbus_test.go`.
