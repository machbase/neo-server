# Machbase Neo JavaScript System Module

## now()

Get the process id of the current process.

**Syntax**

```js
now()
```

**Parameters**

None.

**Return value**

Current time in native object.

**Usage example**

```js
const m = require("@jsh/system")
console.log("now =", m.now())
```

## parseTime()

**Syntax**

```js
parseTime(epoch, epoch_format)
parseTime(datetime, format)
parseTime(datetime, format, location)
```

**Parameters**

- `epoch` `Number`
- `epoch_format` `String` "s", "ms", "us", "ns"
- `datetime` `String`
- `format` `String`
- `location` `Location` timezone, default is 'Local' if omitted., e.g. system.location('EST'), system.location('America/New_York')

**Return value**

Time in native object

**Usage example**

```js
const {println} = require("@jsh/process");
const system = require("@jsh/system");
ts = system.parseTime(
    "2023-10-01 12:00:00",
    "2006-01-02 15:04:05",
    system.location("UTC"));
println(ts.In(system.location("UTC")).Format("2006-01-02 15:04:05"));

// 2023-10-01 12:00:00
```

## location()

**Syntax**

```js
location(timezone)
```

**Parameters**

- `timezone` `String` time zone, e.g. `"UTC"`, `"Local"`, `"GMT"`, `"ETS"`, `"America/New_York"`...

**Return value**

Location in native object

**Usage example**

```js
const {println} = require("@jsh/process");
const system = require("@jsh/system");
ts = system.time(1).In(system.location("UTC"));
println(ts.Format("2006-01-02 15:04:05"));

// 1970-01-01 00:00:01
```

## Log

**Creation**

| Constructor             | Description                          |
|:------------------------|:----------------------------------------------|
| new Log(*name*)         | Instantiates a logger with the given name     |

**Options**

- `name` `String` logger name

**Usage example**

```js
const system = require("@jsh/system");
const log = new system.Log("testing");

log.info("hello", "world");

// Log output:
//
// 2025/05/13 14:08:41.937 INFO  testing    hello world
```

### trace()

**Syntax**

```js
trace(...args)
```

**Parameters**

- `args` `any` variable length of arguments for writing log message.

**Return value**

None.

### debug()

**Syntax**

```js
debug(...args)
```

**Parameters**

- `args` `any` variable length of arguments for writing log message.

**Return value**

None.

### info()

**Syntax**

```js
info(...args)
```

**Parameters**

- `args` `any` variable length of arguments for writing log message.

**Return value**

None.

### warn()

**Syntax**

```js
warn(...args)
```

**Parameters**

- `args` `any` variable length of arguments for writing log message.

**Return value**

None.

### error()

**Syntax**

```js
error(...args)
```

**Parameters**

- `args` `any` variable length of arguments for writing log message.

**Return value**

None.