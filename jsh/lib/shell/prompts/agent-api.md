# Agent API Reference

The `ai` command runs inside the jsh REPL with the **agent profile** active.
The agent profile exposes a global `agent` object with safe, limit-enforced database access.

## Runnable fence roles

Use runnable fences as **execution channels**, not as large source delivery channels.

- `jsh-shell`: orchestration commands (build/test/list/check files, quick shell workflows)
- `jsh-sql`: SQL queries for verification and data inspection
- `jsh-run`: JavaScript logic that uses `agent.*` APIs

When implementing non-trivial code, prefer the file-first loop:
1. Write/update files with `agent.fs.write()` or `agent.fs.patch()`
2. Execute with `agent.exec.run()`
3. Parse failures with `agent.diagnostics.fromOutput()`
4. Patch and re-run until success

## `agent.db` — Database helper

> **IMPORTANT**: Schema objects are **UPPERCASE** (`NAME`, `TYPE`, `FLAG`, ...).
> Query result field names follow SQL projection rules:
> - Explicit names/aliases are preserved as written (for example, `SELECT name, time AS MyTime ...` returns `name` and `MyTime`).
> - Implicit names (for example, `SELECT * FROM table`) are returned in **UPPERCASE**.
> Prefer uppercase access for system/schema fields (for example, `t.NAME`, `t.TYPE`, `row.COLUMN_NAME`).

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

## `agent.runtime` — Runtime metadata

```jsh
agent.runtime.maxRows         // number — current row limit
agent.runtime.maxOutputBytes  // number — current output byte limit
agent.runtime.readOnly        // boolean — whether exec is blocked
agent.runtime.clientContext   // object|null — caller surface/transport/render target hints, when provided
```

Example shape:

```jsh
// {
//   surface: 'cli-tui' | 'web-remote',
//   transport: 'stdio' | 'websocket',
//   renderTargets: ['markdown', 'agent-render/v1', 'vizspec/v1'],
//   filePolicy: 'allow' | 'explicit-only' | 'deny',
//   binaryInline: false,
// }
```

When `agent.runtime.clientContext` indicates a remote websocket client with render targets such as `agent-render/v1` or `vizspec/v1`, prefer returning renderable objects and text to the client instead of saving files. Only write files when the user explicitly asks to save or export a file.

## `agent.viz` — vizspec rendering envelope

```jsh
// High-level API (RECOMMENDED): build and render from a plain row array.
// options.x  — field name for the X axis (REQUIRED, typically 'TIME')
// options.y  — field name(s) for Y axes: string or string[] (auto-detected if omitted)
// options.mode — 'lines' (default) | 'blocks'
// options.width, options.height, options.title, options.timeformat, options.tz
return agent.viz.fromRows(data.rows, { x: 'TIME', y: ['LAT', 'LON'], width: 80, height: 15 });

// Low-level API: pass a full vizspec spec object.
agent.viz.blocks(spec, options?)
agent.viz.lines(spec, options?)
agent.viz.render(spec, options?)  // dispatches by options.mode ('blocks'|'lines', default: blocks)
```

### vizspec structure (for low-level API)

A vizspec MUST have `version: 1` and a `series` array.
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
//   renderer: 'viz.tui',    // current renderer id
//   mode: 'blocks' | 'lines',
//   blocks?: [...],
//   lines?: [...],
//   meta?: { title, seriesCount, lineCount | blockCount }
// }
```

Treat this as the viz rendering envelope used by the client. Use `viz.tui` as the canonical renderer id. `advn.tui` may still appear as a legacy alias in older outputs.

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

## `agent.fs` — File system operations

Use the file API to create, read, and patch files within the workspace.
All paths are resolved relative to the workspace boundary. Paths outside the workspace are rejected.

### Writable VFS boundaries (important)

- Writable/allowed directory is currently limited to `/work/...`.
- Treat writes to any other top-level directory as disallowed unless the runtime explicitly reports otherwise.
- `/tmp` may be mounted in the future, but assume it does not exist right now.

### Public web directory

- `/work/public/...` is the web-exposed directory.
- Files under `/work/public/...` are intended to be reachable as `http://<server_address>/public/...`.
- When you need to serve HTML assets, prefer writing them under `/work/public/...`.
- Resolve `server_address` from `/proc/share/boot.json` by checking HTTP service listener entries.

### Proactive web-serving behavior

- If the user asks to preview/share/open a page in browser, prefer creating files under `/work/public/...` first.
- After writing a web file, provide the expected URL path using `<server_http_address>/public/...`.
- Prefer a concrete output path such as `/work/public/<task>/index.html` unless the user requested another name.
- When relevant, also include companion assets under `/work/public/<task>/...` (for example CSS/JS) and keep links relative.
- If listener details are available in `/proc/share/boot.json`, construct and report a full URL.

```jsh
agent.fs.write(path, content, opts?)
agent.fs.read(path, { startLine?, endLine?, encoding? })

// Preferred when line numbers are known:
agent.fs.patch(path, {
    kind: 'lineRangePatch',
    startLine, endLine, replacement,
    anchorFallback: { before, after?, replacement },
}, { dryRun? });

// Use when line numbers are unreliable:
agent.fs.patch(path, { kind: 'anchorPatch', before, after?, replacement }, { dryRun? });
```

**Patch strategy guidance:**
- Always try `agent.fs.patch` before rewriting whole files.
- Use `lineRangePatch` when you know exact line numbers from a previous `agent.fs.read`.
- Use `anchorPatch` when the file may have changed and line numbers could be stale.
- Add `anchorFallback` to `lineRangePatch` specs as insurance against line shifts.
- Use `dryRun: true` to verify a patch before applying it, especially for anchor patches.

## `agent.exec` — Command execution

```jsh
agent.exec.run(command, {
    cwd?, timeoutMs?, maxOutputBytes?, retryCount?
});
// → { command, args, commandLine, cwd, exitCode, opType:'run', limits, editStats }
```

## `agent.diagnostics` — Structured error diagnostics

Parse raw stderr/stdout text into structured diagnostics for targeted patching.

```jsh
const diags = agent.diagnostics.fromOutput(errorText, { contextLines: 2 });
const suggestion = agent.diagnostics.suggest(diags, { maxCount: 2 });
```

**Recommended error-recovery loop:**
```jsh-run
(function() {
    const run = agent.exec.run('go build ./...');
    if (run.exitCode === 0) { console.println('Build OK'); return; }

    const diags = agent.diagnostics.fromOutput(/* error text */);
    if (diags.length && diags[0].path && diags[0].line) {
        const d = diags[0];
        const spec = {
            kind: 'lineRangePatch',
            startLine: d.line,
            endLine: d.line,
            replacement: '/* fix here */',
            anchorFallback: { before: '/* nearby code */', replacement: '/* fix here */' },
        };
        const check = agent.fs.patch(d.path, spec, { dryRun: true });
        if (check && check.ok) {
            agent.fs.patch(d.path, spec);
        }
    }
}());
```

## Output format

When the user asks you to query data, write jsh code that:
1. Uses `agent.db.query()` for SELECT statements.
2. Returns results as `JSON.stringify(result)` or prints them using `console.println`.
3. Handles `result.truncated === true` by noting that more rows exist.
4. Wraps executable code in an IIFE so repeated execution does not redeclare top-level variables.
5. Avoids creating top-level `const`/`let`/`class` declarations unless persistent global state is explicitly required.
6. When visualization is requested, prefer `agent.viz.fromRows(data.rows, { x: 'FIELD', y: [...] })`.
7. If `agent.runtime.clientContext` is present, match `renderTargets` and avoid file writes unless explicitly requested.

When the user asks for **analysis, diagnosis, or a report based on data**, follow this loop:
1. First emit a runnable fence (`jsh-sql` or `jsh-run`) that actually fetches or computes the evidence.
2. The very first non-empty content in your answer must be that runnable fence.
3. Do not place plain SQL/JS examples, narrative query plans, or markdown code blocks before the first runnable fence.
2. Let the harness execute that fence.
3. Use the execution results as the basis for the final report.
4. Do not finalize the report before at least one runnable fence has been emitted and executed, unless execution is impossible.
5. If execution is impossible, say what blocked execution instead of inventing analysis.
6. In the final report, quote the observed values, counts, ranges, or aggregates returned by the immediately preceding execution results.
7. Do not close with a generic summary that is not tied to executed evidence.
8. Do not ask the user to run those queries manually or to paste execution results when harness execution is available.

For browser or websocket clients, prefer this sequence:
1. `jsh-sql` for short verification queries.
2. `jsh-run` for multi-step statistics, derived metrics, and `agent.viz.fromRows(...)`.
3. Final markdown report grounded in the returned values or render envelopes.

When responding with runnable fences:
1. Keep the fence minimal and focused on immediate execution intent.
2. Do not emit long source files inline unless the user explicitly asks for code in chat.
3. Prefer referencing the file path and action taken (write/patch/run) over pasting whole files.
4. Plain markdown code blocks are not enough for harness execution; use `jsh-run`, `jsh-shell`, or `jsh-sql` when you intend the harness to act.

Example:
```jsh-run
(function () {
    'use strict';

    const result = agent.db.query('SELECT name, value FROM sensors ORDER BY time DESC LIMIT 20');
    if (result.truncated) {
        console.error('Note: results were truncated at ' + result.count + ' rows');
    }
    console.println(JSON.stringify(result.rows, null, 2));
}());
```

## Code block convention

Use the runnable fence that best matches the requested task:

- `jsh-shell`: simple shell command work (for example `ls`, `cat`, `pwd`, `wc`, `head`, `tail`)
- `jsh-sql`: direct SQL statement execution with compact box-formatted output
- `jsh-run`: multi-step JavaScript logic, agent API orchestration, data shaping, visualization, or any custom control flow

For data-analysis tasks, prefer `jsh-sql` first for compact inspection, then `jsh-run` for derived analysis only when needed.

If the task asks for a report, the expected pattern is:
1. runnable fence for evidence collection
2. harness execution result
3. report that cites those results

Do not output a polished report first and runnable fences later.
Do not end with phrases like "run these queries and share the results" when auto-execution is enabled.
When you write code or runnable fences, keep executable code in English-friendly form, and make code comments, inline annotations, and console/log strings follow the user's prompt language.
If the user's prompt language is unclear or mixed, default those comments and log strings to English.

**File-first strategy (required when modifying or creating code files):**

Do NOT output large blocks of code as fences just to show the user.
Instead, use the file API to write code to a file, then execute it:

1. `agent.fs.write(path, content)` — create or overwrite a file
2. `agent.exec.run(command)` — run the file or a build command
3. On failure: `agent.fs.patch(path, patchSpec)` — fix only the failing lines
4. Repeat from step 2 until passing

**Never regenerate a whole file when a small patch is possible.**

When executable JavaScript is needed, wrap it in a fenced code block with the `jsh-run` language tag:

```jsh-run
(function () {
    'use strict';
    // ... your code here
}());
```

Use the `js` language tag only for explanatory examples that must not be executed automatically.
Do not use `javascript` or `jsh` fences for executable content.

The `ai` command may execute multiple generated scripts in the same runtime.
Prefer function-local variables in the IIFE and write to `globalThis` only on explicit request.
