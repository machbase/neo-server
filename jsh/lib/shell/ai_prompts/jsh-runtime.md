# JSH Runtime Environment

You are operating inside **jsh** — a JavaScript runtime implemented in Go using the [goja](https://github.com/dop251/goja) engine.

## Key constraints

- **Never use `await` / `async` not supported** — the runtime is synchronous. Asynchronous operations use callbacks or the `setImmediate` pattern.
- **No `import`** — use `require('module')` (CommonJS style).
- **No Node.js built-ins** unless explicitly listed below.

## Available globals

| Global | Description |
|--------|-------------|
| `require(id)` | Load a module by name or path |
| `console.log(...) ` | Print to stdout |
| `console.error(...)` | Print to stderr |
| `console.print(...)` and `console.println(...)` | Print to stdout, preferred over `console.log(...)` in JSH |
| `Buffer` | Node.js-compatible Buffer (available globally) |
| `URL` | WHATWG URL (available globally) |
| `setImmediate(fn)` | Schedule microtask after current execution |
| `clearImmediate(id)` | Cancel a setImmediate |
| `setTimeout(fn, ms)` / `clearTimeout(id)` | Timer-based scheduling |
| `process.env` | Host environment variables (read-only object) |
| `process.argv` | Script arguments array |
| `process.exit(code)` | Terminate the process |
| `process.stdin` / `process.stdout` / `process.stderr` | Readable/Writable streams |

## Module resolution

```javascript
// Native module (Go-implemented)
const mod = require('@jsh/process');

// Library module (JavaScript)
const { Client } = require('machcli');       // from /lib/machcli.js
const http = require('http');                 // from /lib/http.js
const fs   = require('fs');                   // from /lib/fs.js
```

Relative paths are resolved from the script's directory. Absolute paths from `/` are resolved in the virtual filesystem (VFS).

## Event-driven pattern (no await)

```javascript
const emitter = someAsyncOperation();
emitter.on('data', function(chunk) { /* handle */ });
emitter.on('end',  function(result) { /* done */ });
emitter.on('error', function(err)  { /* handle error */ });
```

## Error handling

```javascript
try {
    const result = syncOperation();
} catch (e) {
    console.error('Error:', e.message);
    process.exit(1);
}
```

## JSH module system

- https://docs.machbase.com/neo/jsh/modules/index.md

## Built-in Modules

- `archive`: Archive module group for TAR and ZIP handling in JSH applications, covering in-memory helpers, callback-style async wrappers, stream-style readers and writers, and file-based archive classes. Open this when you need the overview of built-in archive support or need to choose between `archive/tar` and `archive/zip`. https://docs.machbase.com/neo/jsh/modules/archive.md
- `archive/tar`: Create and extract TAR archives using in-memory helpers, callback wrappers, stream-style APIs, or the file-oriented `Tar` class. Open this when working with `.tar` bytes, saving archives to disk, or extracting selected entries. https://docs.machbase.com/neo/jsh/modules/archive/tar.md
- `archive/zip`: Create and extract ZIP archives using in-memory helpers, callback wrappers, stream-style APIs, or the file-oriented `Zip` class. Open this when working with `.zip` bytes, saving archives to disk, or extracting selected entries. https://docs.machbase.com/neo/jsh/modules/archive/zip.md
- `events`: Lightweight `EventEmitter` APIs for event-driven JSH code, including listener registration, one-time handlers, removal, emission, and listener inspection helpers. Open this when you need custom event emitters or need to understand modules built on JSH event patterns. https://docs.machbase.com/neo/jsh/modules/events.md
- `fs`: Synchronous Node.js-compatible filesystem APIs for reading, writing, copying, renaming, deleting, listing, streaming, and inspecting files or directories. Open this for path, file descriptor, or directory operations. https://docs.machbase.com/neo/jsh/modules/fs.md
- `http`: Node.js-compatible HTTP client and server APIs. Open this for requests, response parsing, route handlers, REST endpoints, static files, redirects, or HTML template responses. https://docs.machbase.com/neo/jsh/modules/http.md
- `machcli`: Machbase database client APIs. Open this for connecting, running `query()` or `queryRow()`, executing DDL or DML with `exec()`, explaining SQL, bulk append, or inspecting Machbase table and column metadata helpers. https://docs.machbase.com/neo/jsh/modules/machcli.md
- `mathx`: General numeric and statistical helpers such as array generation, sorting, descriptive statistics, correlation, regression, quantiles, entropy, and FFT. Open this when basic `Math` is not enough and you need dataset-oriented numeric functions. https://docs.machbase.com/neo/jsh/modules/mathx/index.md
- `mathx/filter`: Stateful filters for sampled numeric data, including running average, moving average, low-pass filtering, Kalman filtering, and Kalman smoothing. Open this when transforming noisy sequential values. https://docs.machbase.com/neo/jsh/modules/mathx/filter.md
- `mathx/interp`: Interpolation models including piecewise constant, piecewise linear, Akima spline, Fritsch-Butland, linear regression, and several cubic spline variants. Open this when fitting sample points and predicting intermediate values. https://docs.machbase.com/neo/jsh/modules/mathx/interp.md
- `mathx/mat`: Matrix and vector APIs centered on `Dense`, `VecDense`, QR factorization, solving linear systems, matrix arithmetic, and formatted matrix output. Open this for linear algebra work rather than scalar statistics. https://docs.machbase.com/neo/jsh/modules/mathx/mat.md
- `mathx/simplex`: Seeded Simplex noise generator with 1D to 4D `eval()` methods. Open this when deterministic noise values are needed from numeric coordinates. https://docs.machbase.com/neo/jsh/modules/mathx/simplex.md
- `mathx/spatial`: Spatial math helper currently documented for `haversine()` great-circle distance calculation between latitude and longitude points. Open this specifically for earth-distance calculations. https://docs.machbase.com/neo/jsh/modules/mathx/spatial.md
- `mqtt`: Event-driven MQTT client with broker connection options plus `publish()`, `subscribe()`, `unsubscribe()`, and MQTT v5 properties support. Open this for broker messaging flows and message event handling. https://docs.machbase.com/neo/jsh/modules/mqtt.md
- `nats`: Event-driven NATS client APIs with connection options, `publish()`, `subscribe()`, request-reply support, and lifecycle events such as `open`, `message`, and `close`. Open this for NATS broker integration, pub/sub, or request-reply messaging. https://docs.machbase.com/neo/jsh/modules/nats.md
- `net`: Node.js-compatible TCP APIs with `createServer()`, `createConnection()` / `connect()`, socket events, and IP validation helpers. Open this for raw TCP client or server code. https://docs.machbase.com/neo/jsh/modules/net.md
- `opcua`: OPC UA client APIs with node read, write, browse, paginated browse, and children lookup operations plus related request and result types. Open this for OPC UA device or server integration. https://docs.machbase.com/neo/jsh/modules/opcua.md
- `os`: Node.js-compatible operating system information APIs covering platform, memory, uptime, CPUs, interfaces, disks, host info, user info, and signal or priority constants. Open this for host inspection and runtime environment data. https://docs.machbase.com/neo/jsh/modules/os.md
- `parser`: Streaming CSV and NDJSON parser APIs for JSH streams, including factory helpers and parser classes that emit parsed records through events. Open this when reading structured text streams into row or object events. https://docs.machbase.com/neo/jsh/modules/parser.md
- `path`: Node.js-like path manipulation helpers, with POSIX behavior by default plus `path.posix` and `path.win32` namespaces. Open this for joining, normalizing, resolving, parsing, or formatting filesystem paths. https://docs.machbase.com/neo/jsh/modules/path.md
- `pretty`: Terminal output helpers for table rendering, progress indicators, byte or integer or duration formatting, row helpers, alignment constants, and terminal utilities. Open this when output formatting matters more than raw `console.println()` text. https://docs.machbase.com/neo/jsh/modules/pretty.md
- `process`: JSH process APIs for argv and env access, cwd changes, command execution, shutdown hooks, timers and event-loop helpers, signals, stdio, runtime metadata, and process lifecycle control. Open this for CLI behavior, process control, or script execution concerns. https://docs.machbase.com/neo/jsh/modules/process.md
- `readline`: Interactive line input APIs centered on the `ReadLine` class, including prompts, history, multi-line entry, and simulated input keys. Open this when building REPL-style tools or reading terminal input synchronously. https://docs.machbase.com/neo/jsh/modules/readline.md
- `semver`: Semantic version comparison helpers such as `satisfies()`, `maxSatisfying()`, and `compare()`. Open this when validating version ranges or choosing the best matching version string. https://docs.machbase.com/neo/jsh/modules/semver.md
- `service`: Client APIs for the Machbase Neo service controller JSON-RPC interface, including status queries, reloads, installs, start and stop actions, and service detail updates. Open this when inspecting or controlling Neo-managed services from JSH. https://docs.machbase.com/neo/jsh/modules/service.md
- `util`: Small helper APIs for JSH applications and built-in commands, currently centered on `parseArgs` and `splitFields`, available from the root module or submodules. Open this for structured CLI argument parsing or shell-like field splitting with quote handling. https://docs.machbase.com/neo/jsh/modules/util.md
- `ws`: WebSocket client APIs centered on the `WebSocket` class, with connection state, `send()`, `close()`, and event-driven message handling. Open this when connecting to WebSocket servers from JSH. https://docs.machbase.com/neo/jsh/modules/ws.md
- `zlib`: Node.js-style compression and decompression APIs for gzip, deflate, raw deflate, unzip, synchronous helpers, callback-based async helpers, and stream-style processing. Open this when compressing or decompressing binary or text payloads. https://docs.machbase.com/neo/jsh/modules/zlib.md