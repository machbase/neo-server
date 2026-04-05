# Agent API Reference

The `ai` command runs inside the jsh REPL with the **agent profile** active.
The agent profile exposes a global `agent` object with safe, limit-enforced database access.

## `agent.db` — Database helper

```javascript
// Lazy-connects on first use. Reads connection config from /share/database/machcli.json
// or falls back to 127.0.0.1:5656 sys/manager.

agent.db.connect(path?)     // (Re-)connect, optionally path to config JSON.
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

```javascript
agent.schema.tables()          // → [{NAME, TYPE, FLAG}, ...] — list all tables
agent.schema.describe(table)   // → [{NAME, TYPE, LENGTH, FLAG}, ...] — table columns
                               //   TYPE and FLAG are integer codes from M$SYS_COLUMNS
```

> **IMPORTANT**: All field names in query results and schema objects are **UPPERCASE**.
> Use `t.NAME`, `t.TYPE`, `row.COLUMN_NAME`, etc. — never lowercase.

## `agent.runtime` — Runtime metadata

```javascript
agent.runtime.maxRows         // number — current row limit
agent.runtime.maxOutputBytes  // number — current output byte limit
agent.runtime.readOnly        // boolean — whether exec is blocked
agent.runtime.provider        // string — active LLM provider name
agent.runtime.model           // string — active LLM model name
```

## Output format

When the user asks you to query data, write jsh code that:
1. Uses `agent.db.query()` for SELECT statements.
2. Returns results as `JSON.stringify(result)` or prints them using `console.log`.
3. Handles `result.truncated === true` by noting that more rows exist.

Example:
```javascript
'use strict';
const result = agent.db.query('SELECT name, value FROM sensors ORDER BY time DESC LIMIT 20');
if (result.truncated) {
    console.error('Note: results were truncated at ' + result.count + ' rows');
}
console.log(JSON.stringify(result.rows, null, 2));
```

## Code block convention

When generating executable jsh code, wrap it in a fenced code block with the `jsh` language tag:

```jsh
'use strict';
// ... your code here
```

The `ai` command detects ` ```jsh ` blocks and offers to execute them automatically.
