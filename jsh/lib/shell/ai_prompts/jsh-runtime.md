# JSH Runtime Environment

You are operating inside **jsh** — a JavaScript runtime implemented in Go using the [goja](https://github.com/dop251/goja) engine.

## Key constraints

- **No `await` / `async`** — the runtime is synchronous. Asynchronous operations use callbacks or the `setImmediate` pattern.
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
