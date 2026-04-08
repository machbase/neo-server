# Agent API Reference

The `ai` command runs inside the jsh REPL with the **agent profile** active.
The agent profile exposes a global `agent` object with safe, limit-enforced database access.

## `agent.db` — Database helper

```jsh
// Lazy-connects on first use. Reads connection config from /proc/share/db.json
// or falls back to 127.0.0.1:5656 sys/manager.

agent.db.connect(pathOrConfig?) // (Re-)connect, optionally path to config JSON or override object.
agent.db.disconnect()       // Close connection and client.

// query — always returns a plain serializable object, never a cursor.
// Automatically enforces maxRows limit (default 1000).
const result = agent.db.query('SELECT name, value FROM mytable LIMIT 10');
// result: { rows: [{NAME: val, VALUE: val, ...}, ...], truncated: bool, count: N }
// NOTE: Column names in rows are ALWAYS UPPERCASE, matching Machbase convention.

// exec — DDL/DML. Blocked when readOnly mode is active.
agent.db.exec('CREATE TAG TABLE t (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE)');
```

## `agent.schema` — Schema inspection

```jsh
agent.schema.tables()          // → [{NAME, TYPE, FLAG}, ...] — list all tables
agent.schema.describe(table)   // → [{NAME, TYPE, LENGTH, FLAG}, ...] — table columns
                               //   TYPE and FLAG are integer codes from M$SYS_COLUMNS
```

> **IMPORTANT**: Schema objects are **UPPERCASE** (`NAME`, `TYPE`, `FLAG`, ...).
> Query result field names follow SQL projection rules:
> - Explicit names/aliases are preserved as written (for example, `SELECT name, time AS MyTime ...` returns `name` and `MyTime`).
> - Implicit names (for example, `SELECT * FROM table`) are returned in **UPPERCASE**.
> Prefer uppercase access for system/schema fields (for example, `t.NAME`, `t.TYPE`, `row.COLUMN_NAME`).

## `agent.runtime` — Runtime metadata

```jsh
agent.runtime.maxRows         // number — current row limit
agent.runtime.maxOutputBytes  // number — current output byte limit
agent.runtime.readOnly        // boolean — whether exec is blocked
agent.runtime.provider        // string — active LLM provider name
agent.runtime.model           // string — active LLM model name
```

## `agent.viz` — ADVN TUI rendering envelope

```jsh
// High-level API (RECOMMENDED): build and render from a plain row array.
// options.x  — field name for the X axis (REQUIRED, typically 'TIME')
// options.y  — field name(s) for Y axes: string or string[] (auto-detected if omitted)
// options.mode — 'lines' (default) | 'blocks'
// options.width, options.height, options.title, options.timeformat, options.tz
return agent.viz.fromRows(data.rows, { x: 'TIME', y: ['LAT', 'LON'], width: 80, height: 15 });

// Low-level API: pass a full ADVN spec object.
agent.viz.blocks(spec, options?)
agent.viz.lines(spec, options?)
agent.viz.render(spec, options?)  // dispatches by options.mode ('blocks'|'lines', default: blocks)
```

### ADVN spec structure (for low-level API)

An ADVN spec MUST have `version: 1` and a `series` array.
Each series MUST have an `id` and `representation.kind`.

**Valid `representation.kind` values** (the ONLY allowed values):
- `"raw-point"` — raw time/value pairs: `fields: ['xField', 'yField']`
- `"time-bucket-value"` — bucketed metrics: `fields: ['time', 'value']`
- `"time-bucket-band"` — min/avg/max band: `fields: ['time', 'min', 'avg', 'max']` (any 2 of min/max/avg)
- `"distribution-histogram"` — histogram bars: `fields: ['binStart', 'binEnd', 'count']`
- `"distribution-boxplot"` — box plot: `fields: ['category', 'low', 'q1', 'median', 'q3', 'high']`
- `"event-point"` — event markers: `fields: ['time', 'label']`
- `"event-range"` — time ranges: `fields: ['from', 'to', 'label']`

> **IMPORTANT**: `"line"`, `"bar"`, `"scatter"` are NOT valid kinds — they do not exist.

**Data lives inside each series object, NOT at the spec top level:**

```jsh
// WRONG — do not put data at spec level:
// { data: rows, series: [{ id: 'x', field: 'lat' }] }

// CORRECT — data goes inside each series:
const spec = vizspec.createSpec({
    series: [{
        id: 'lat',
        name: 'Latitude',
        representation: { kind: 'raw-point', fields: ['TIME', 'LAT'] },
        data: data.rows.map(r => [r.TIME, r.LAT]),
    }, {
        id: 'lon',
        name: 'Longitude',
        representation: { kind: 'raw-point', fields: ['TIME', 'LON'] },
        data: data.rows.map(r => [r.TIME, r.LON]),
    }],
});
return agent.viz.lines(spec, { width: 80, height: 15, title: 'GPS Track' });
```

However, **prefer `agent.viz.fromRows()`** which handles spec construction automatically:

```jsh
const data = agent.db.query('SELECT time, lat, lon FROM demo WHERE name=\'firenze\' ORDER BY time LIMIT 50');
return agent.viz.fromRows(data.rows, {
    x: 'TIME',
    y: ['LAT', 'LON'],
    width: 80,
    height: 15,
    title: 'Firenze GPS Track',
});
```

### Return shape:
```jsh
// {
//   __agentRender: true,
//   schema: 'agent-render/v1',
//   renderer: 'advn.tui',
//   mode: 'blocks' | 'lines',
//   blocks?: [...],
//   lines?: [...],
//   meta?: { title, seriesCount, lineCount | blockCount }
// }
```

## `agent.modules` — Online JSH module manuals

```jsh
// List module references with markdown URLs.
// By default it tries to merge builtin catalog + online index.md.
const listing = agent.modules.list();
// listing: { modules:[{name, summary, url, source}], count, indexURL, online, onlineError, fetchedAt }

// Force refresh from online index markdown.
const index = agent.modules.index(true);
// index: { url, statusCode, fetchedAt, modules:[name...], count, bytes, originalBytes, truncated, omitMarkdown, markdown? }
const lightIndex = agent.modules.index({ force: true, omitMarkdown: true });

// Normalize and resolve a module reference.
agent.modules.resolve('fs');                                      // { name:'fs', url:'.../fs.md', summary:'...' }
agent.modules.resolve('https://docs.machbase.com/neo/jsh/modules/path.md');

// Fetch full markdown manual for one module.
const fsDoc = agent.modules.fetch('fs');
// fsDoc: { name, summary, url, statusCode, fetchedAt, bytes, originalBytes, truncated, omitMarkdown, markdown? }
const fsDocPreview = agent.modules.fetch('fs', { maxBytes: 12000 });
const fsDocMeta = agent.modules.fetch('fs', { omitMarkdown: true });

// Fetch all module manuals (or selected ones).
const allDocs = agent.modules.fetchAll();
const picked = agent.modules.fetchAll({ modules: ['fs', 'http', 'machcli'] });
const pickedPreview = agent.modules.fetchAll({
    modules: ['fs', 'http', 'machcli'],
    maxBytes: 8000,
    omitMarkdown: false,
});
```

## `agent.sqlref` — Online Machbase SQL reference manuals

```jsh
// List configured SQL reference pages with markdown URLs.
const sqlListing = agent.sqlref.list();
// sqlListing: { docs:[{name, title, summary, url, source}], count, indexURL, fetchedAt }

// Fetch the SQL reference index page (currently used for Datatypes).
const sqlIndex = agent.sqlref.index();
// sqlIndex: { name, title, summary, url, statusCode, fetchedAt, bytes, originalBytes, truncated, omitMarkdown, markdown? }
const sqlIndexMeta = agent.sqlref.index({ omitMarkdown: true });

// Normalize and resolve a SQL reference.
agent.sqlref.resolve('datatypes');                                // { name:'datatypes', title:'Datatypes', url:'.../index.md', summary:'...' }
agent.sqlref.resolve('https://docs.machbase.com/dbms/sql-reference/functions.md');

// Fetch full markdown manual for one SQL reference page.
const ddlDoc = agent.sqlref.fetch('ddl');
// ddlDoc: { name, title, summary, url, statusCode, fetchedAt, bytes, originalBytes, truncated, omitMarkdown, markdown? }
const fnDocPreview = agent.sqlref.fetch('functions', { maxBytes: 12000 });
const dmlDocMeta = agent.sqlref.fetch('dml', { omitMarkdown: true });

// Fetch all configured SQL reference pages (or selected ones).
const allSqlDocs = agent.sqlref.fetchAll();
const pickedSqlDocs = agent.sqlref.fetchAll({ docs: ['datatypes', 'ddl', 'functions'] });
const pickedSqlPreview = agent.sqlref.fetchAll({
    docs: ['datatypes', 'ddl', 'functions'],
    maxBytes: 8000,
    omitMarkdown: false,
});
```

## Output format

When the user asks you to query data, write jsh code that:
1. Uses `agent.db.query()` for SELECT statements.
2. Returns results as `JSON.stringify(result)` or prints them using `console.log`.
3. Handles `result.truncated === true` by noting that more rows exist.
4. Wraps executable code in an IIFE so repeated execution does not redeclare top-level variables.
5. Avoids creating top-level `const`/`let`/`class` declarations unless persistent global state is explicitly required.
6. When visualization is requested, prefer `agent.viz.fromRows(data.rows, { x: 'FIELD', y: [...] })` for simple time-series data. For advanced specs use `agent.viz.blocks(spec)` or `agent.viz.lines(spec)` with a properly constructed ADVN spec (see `agent.viz` section above for valid `representation.kind` values).

Example:
```jsh-run
(function () {
    'use strict';

    const result = agent.db.query('SELECT name, value FROM sensors ORDER BY time DESC LIMIT 20');
    if (result.truncated) {
        console.error('Note: results were truncated at ' + result.count + ' rows');
    }
    console.log(JSON.stringify(result.rows, null, 2));
}());
```

## Code block convention

When generating executable jsh code, wrap it in a fenced code block with the `jsh-run` language tag:

```jsh-run
(function () {
    'use strict';
    // ... your code here
}());
```

Use the `js` language tag only for explanatory examples that must not be executed automatically.
Do not use `javascript` or `jsh` or `js` fences for executable content.

The `ai` command may execute multiple generated scripts in the same runtime.
Prefer function-local variables inside the IIFE instead of top-level declarations.
Only write to `globalThis` when the user explicitly asks for persistent state across executions.

The `ai` command detects ` ```jsh-run ` blocks and offers to execute them automatically.
