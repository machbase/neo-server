# JSH Built-in Modules Reference

## Online Full Manuals (Markdown)

- Index: https://docs.machbase.com/neo/jsh/modules/index.md
- In agent profile, use `agent.modules.list()`, `agent.modules.fetch(name, options)`, and `agent.modules.fetchAll(options)` to load latest online markdown manuals.
- Use `maxBytes` to limit markdown payload size and `omitMarkdown: true` when only metadata is needed.
- Fetch only the modules needed for the current task. Avoid bulk fetch unless the user explicitly asks for a full sweep.
- Prefer this order: `list()` -> `fetch(name, { omitMarkdown: true })` -> `fetch(name)` only when full text is required.

### Frequently used modules

- `fs`, `path`, `process`, `os`
- `http`, `net`, `ws`
- `machcli`, `pretty`, `parser`, `util`
- For all module names and URLs, call `agent.modules.list()`.

## `fs` — Virtual Filesystem

```jsh
const fs = require('fs');

fs.readFileSync(path, encoding?)   // Read file. encoding='utf8' returns string.
fs.writeFileSync(path, data)       // Write file.
fs.existsSync(path)                // Returns boolean.
fs.readdirSync(path)               // Returns string[] of entry names.
fs.mkdirSync(path, options?)       // Create directory. {recursive: true} for parents.
fs.statSync(path)                  // Returns {size, mtime, isFile(), isDirectory()}.
```

## `path` — Path utilities

```jsh
const path = require('path');

path.join(...parts)     // Join path segments.
path.dirname(p)         // Parent directory.
path.basename(p, ext?)  // File name, optionally without extension.
path.extname(p)         // Extension including dot.
path.resolve(...parts)  // Absolute path resolved from cwd.
```

## `http` — HTTP Client

```jsh
const http = require('http');

// Simple GET
const res = http.get('https://example.com');
// res.statusCode, res.ok, res.text(), res.json()

// POST with body
const req = http.request({
    method: 'POST',
    url:    'https://api.example.com/data',
    headers: { 'Content-Type': 'application/json' },
});
req.write(JSON.stringify({ key: 'value' }));
req.end(function(res) {
    console.log(res.statusCode, res.json());
});
```

## `process` — Process info

```jsh
const process = require('process');   // or require('@jsh/process')

process.argv         // string[] — command-line arguments
process.env          // object  — environment variables
process.exit(code)   // terminate
process.cwd()        // current working directory
process.chdir(path)  // change working directory
```

## `os` — OS utilities

```jsh
const os = require('os');

os.hostname()     // string
os.platform()     // 'darwin' | 'linux' | 'windows'
os.arch()         // 'amd64' | 'arm64' | ...
os.homedir()      // user home directory path
```

## `stream` — Base stream classes

```jsh
const { Readable, Writable, Transform } = require('stream');
// Node.js-compatible stream base classes for building pipelines.
```

## `util/parseArgs` — CLI argument parser

```jsh
const parseArgs = require('util/parseArgs');

const options = {
    name: { type: 'string', short: 'n', description: 'Name', default: '' },
    flag: { type: 'boolean', short: 'f', description: 'Flag', default: false },
    list: { type: 'string', multiple: true, description: 'Repeatable' },
};

const { values, positionals } = parseArgs(process.argv.slice(2), { options });

// Format usage text:
const help = parseArgs.formatHelp({ usage: 'Usage: cmd [options]', description: '...', options });
```

## `machcli` — Machbase database client

```jsh
const { Client } = require('machcli');

const client = new Client({ host: '127.0.0.1', port: 5656, user: 'sys', password: 'manager' });
const conn = client.connect();

// Query
const rows = conn.query('SELECT * FROM mytable WHERE time > ?', someTime);
for (const row of rows) {
    // row is an iterable of {key, value} entries
    const obj = {};
    for (const {key, value} of row) obj[key] = value;
    console.log(obj);
}
rows.close();

// Execute (DDL / DML)
conn.exec('INSERT INTO mytable VALUES (?, ?, ?)', t, name, val);

conn.close();
client.close();
```

## `@jsh/shell` — Shell/REPL native module

```jsh
const { Shell, Repl, ai } = require('@jsh/shell');

// ai sub-module (LLM integration)
ai.send(messages, systemPrompt)       // → {content, inputTokens, outputTokens, provider, model}
ai.stream(messages, systemPrompt)     // → EventEmitter {data(token), end(response), error(err)}
ai.setProvider(name)                  // Switch provider: 'claude' | 'openai' | 'ollama'
ai.providerInfo()                     // → {name, model, maxTokens}
ai.listSegments()                     // → string[] of available prompt segment names
ai.loadSegment(name)                  // → string (markdown content of named segment)
ai.editConfig()                       // Open config in host editor → 'saved'|'cancelled'|'no-editor'|'invalid-json'
ai.config.load()                      // Reload config from disk → config object
ai.config.save(configObj)             // Persist config object to disk
ai.config.set(dotKey, value)          // Set a single value by dot-notation key
ai.config.path()                      // → string path to config file
```

## Prompt usage rule

- Keep module examples short and runnable.
- For detailed behavior, fetch the exact module manual at runtime instead of embedding long excerpts.
