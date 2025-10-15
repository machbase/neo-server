# Node.js / TypeScript

## Overview

The Machbase TypeScript client (`@machbase/ts-client`) is a pure TypeScript implementation of the Machbase CMI protocol. It allows Node.js applications to connect to a Machbase (standard edition) server, execute SQL statements, fetch results, work with prepared statements, and append batches of log data without native bindings.

This document covers installation, core APIs, practical examples, and important behavioural notes.

## Installation

### Requirements

- Node.js 18 or newer (LTS recommended)
- A reachable Machbase server (standard edition)

### Install from npm

Install via your package manager:

```bash
npm install @machbase/ts-client
# or
yarn add @machbase/ts-client
# or
pnpm add @machbase/ts-client
```

### Offline Installation

If you received a `.tgz` file from Machbase:

```bash
# example file name; your version may differ
npm install ./machbase-ts-client-0.9.0.tgz
```

### Verify Installation

```bash
python3 - <<'PY'
from machbaseAPI.machbaseAPI import machbase
print('machbaseAPI import ok')
cli = machbase()
print('isConnected():', cli.isConnected())
PY
```

> **Note**: This client targets Node.js and uses TCP sockets; it is not a browser library (no WebSocket transport).

## Quick Start

The snippet below connects to a local server, creates a sample table, inserts rows, reads them back, and closes the session.

```typescript
// src/example.ts
import { createConnection } from '@machbase/ts-client';

const conn = createConnection({
  host: process.env.MACH_HOST ?? '127.0.0.1',
  port: +(process.env.MACH_PORT ?? 5656),
  user: process.env.MACH_USER ?? 'SYS',
  password: process.env.MACH_PASS ?? 'MANAGER',
});

await conn.connect();
const [rows] = await conn.query('SELECT NAME FROM V$TABLES ORDER BY NAME LIMIT ?', [5]);
console.log(rows);
await conn.end();
```

### CommonJS Example

```javascript
// quickstart.js (CommonJS, Node 18+)
const { createConnection } = require('@machbase/ts-client');

async function main() {
  const conn = createConnection({
    host: '127.0.0.1',
    user: 'SYS',
    password: 'MANAGER',
    port: 5656
  });
  await conn.connect();

  const [rows] = await conn.query('SELECT * FROM v$tables ORDER BY NAME LIMIT ?', [10]);
  console.table(rows);

  await conn.end();
}

main().catch(err => console.error('Unexpected failure:', err));
```

> **Transaction notice:** Machbase autocommits every statement. Commands such as `BEGIN`, `COMMIT`, or `ROLLBACK` always return an error and should only be used to detect the lack of transaction support.

## Common Issues

- **ECONNREFUSED** – Verify the server is started (`machadmin -u`) and the host/port are correct, and that firewalls permit TCP to the listener port (default 5656).
- **Authentication failed** – Check user/password and that the target database is created (`machadmin -c`).

## API Reference

### Connection Management

#### createConnection(config)

Establishes a network session to the Machbase listener and completes the CMI handshake.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `host` | string | `127.0.0.1` | IP or hostname of the Machbase server |
| `port` | number | `5656` | Listener port |
| `user` | string | – | Database user (commonly `SYS`) |
| `password` | string | – | Password (commonly `MANAGER`) |
| `database` | string | `data` | Database name |
| `clientId` | string | `CLI` | Client identifier shown in server logs |
| `showHiddenColumns` | boolean | `false` | Include hidden columns in metadata |
| `timezone` | string | empty | Optional time zone identifier |
| `connectTimeout` | number | 5000 | Socket connect timeout (ms) |
| `queryTimeout` | number | 60000 | Per-command timeout (ms) |

```javascript
const conn = createConnection({ host: '192.168.1.10', user: 'SYS', password: 'MANAGER' });
await conn.connect();
```

The promise rejects if the socket fails, authentication fails, or the handshake response is unexpected.

#### connect()

Opens the connection to the server.

```javascript
await conn.connect();
```

#### end()

Closes the underlying socket. After calling `end()`, any further operation will throw an error.

```javascript
await conn.end();
```

### Executing SQL

#### execute(sql, values?)

Runs a statement that does not necessarily return rows. Use it for DDL (`CREATE`, `ALTER`, `DROP`) or data modification (`INSERT`, `UPDATE`, `DELETE`).

```javascript
const [create] = await conn.execute('CREATE TABLE demo (ID INTEGER, NAME VARCHAR(32))');
console.log('Rows affected:', create.affectedRows); // -> 0 for DDL

const [insert] = await conn.execute("INSERT INTO demo VALUES (1, 'alpha')");
console.log('Rows affected:', insert.affectedRows); // -> 1
```

#### query(sql, values?)

Executes a statement that returns rows. The facade resolves with a two‑element tuple `[rows, fields]`.

```javascript
const [rows, fields] = await conn.query('SELECT ID, NAME FROM demo ORDER BY ID');
console.table(rows);
```

### Prepared Statements

#### prepare(sql)

Creates a prepared statement on the server:

```javascript
const stmt = await conn.prepare('SELECT NAME FROM demo WHERE ID = ?');
try {
  const [rows] = await stmt.execute([1]);
  console.log(rows); // -> [ { NAME: 'alpha' } ]
} finally {
  await stmt.close();
}
```

Methods on the returned object:

- `execute(parameters?)` – runs the statement and resolves to `[rowsOrPacket, fields]`.
- `getColumns()` – returns cached column metadata.
- `getLastMessage()` – surfaces the last server message for the statement.
- `getStatementId()` – exposes the internal statement identifier.
- `close()` – frees the server resource; safe to call more than once.

#### Prepared Statement Examples

**Reusing a prepared SELECT:**

```javascript
const select = await conn.prepare('SELECT DEVICE_ID, SENSOR_VALUE FROM sensors WHERE DEVICE_ID = ?');
for (const { id } of samples) {
  const [rows] = await select.execute([id]);
  console.log(`selected ${id}:`, rows);
}
await select.close();
```

**Prepared upsert:**

```javascript
const upsert = await conn.prepare('INSERT INTO devices VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE');
const [result] = await upsert.execute([deviceId, firstValue, firstValue]);
console.log('Affected rows:', result.affectedRows);
await upsert.close();
```

**Typed parameters and NULL values:**

```javascript
await update.execute([
  { value: null, type: 'varchar' },
  { value: new Date(), type: 'varchar' },
  { value: 'sensor-200', type: 'varchar' },
]);
```

### Append Protocol

#### appendBatch(table, columns, rows, options?)

Appends rows to a **log table** using the `CMI_APPEND_BATCH_PROTOCOL`. Provide only user-visible columns (log tables implicitly contain `_arrival_time` and `_rid`).

```javascript
const appendResult = await conn.appendBatch(
  'sensor_log',
  [
    { name: 'ID', type: 'int32' },
    { name: 'NAME', type: 'varchar' },
    { name: 'VALUE', type: 'float64' },
  ],
  [
    [1, 'alpha', 0.5],
    { values: [2, 'bravo', 1.25], arrivalTime: Date.now() * 1_000_000 },
  ],
);
console.log('Appended rows:', appendResult.rowsAppended);
```

Supported column types: `int32`, `int64`, `float64`, `varchar`.

The promise resolves to `{ table, rowsAppended, rowsFailed, message }`.

> **Tip**: A "column count does not match" error usually means the target table is not a log table or the provided columns are not in schema order.

#### appendOpen(table, columns, options?)

Opens a lightweight append session. By default native APPEND open/data/close is enabled.

```javascript
const stream = await conn.appendOpen('sensor_log', [
  { name: 'ID', type: 'int32' },
  { name: 'NAME', type: 'varchar' },
  { name: 'VALUE', type: 'float64' },
]);

await stream.append([
  [1, 'alpha', 0.5],
  [2, 'bravo', 1.25],
]);

await stream.append({ values: [3, 'charlie', 2.5] });
await stream.close();
```

#### append(rows) on an append stream

Sends one or more rows to the open append stream.

```javascript
const frames = await stream.append([
  ['S-001', new Date(), 1.0],
  ['S-002', new Date(Date.now() + 1), 2.0],
]);
console.log('frames sent:', frames);
```

### Helper Methods

#### ping()

Health check using `SELECT 1 FROM V$TABLES`.

```javascript
await conn.ping();
```

#### promise()

Produces a promise-first wrapper mirroring the familiar `.promise()` behaviour.

```javascript
const p = conn.promise();
await p.ping();
const [rows] = await p.query('SELECT NAME FROM V$TABLES ORDER BY NAME LIMIT ?', [5]);
```

#### escape, escapeId, format

Convenience utilities for building SQL strings safely.

```javascript
const safeName = conn.escapeId('table_name');
const safeValue = conn.escape('user input');
```

## Tutorials

### Quickstart (Log Table)

```javascript
// quickstart-log.js
const { createConnection } = require('@machbase/ts-client');

(async () => {
  const conn = createConnection({ host: '127.0.0.1', port: 5656, user: 'SYS', password: 'MANAGER' });
  await conn.connect();
  const table = 'JS_LOG_' + Math.random().toString(36).slice(2, 7).toUpperCase();
  try {
    await conn.execute(`CREATE LOG TABLE "${table}" (ID INTEGER, NAME VARCHAR(64), VALUE DOUBLE)`);
    await conn.execute(`INSERT INTO "${table}" VALUES (1, 'A', 0.5)`);
    const [rows] = await conn.query(`SELECT * FROM "${table}" ORDER BY ID`);
    console.table(rows);
  } finally {
    await conn.execute(`DROP TABLE "${table}"`);
    await conn.end();
  }
})();
```

### Prepared Statements (Reuse)

```javascript
// prepared-reuse.js
const { createConnection } = require('@machbase/ts-client');

(async () => {
  const conn = createConnection({ host: '127.0.0.1', user: 'SYS', password: 'MANAGER' });
  await conn.connect();
  const table = 'JS_VOL_' + Math.random().toString(36).slice(2, 7).toUpperCase();
  try {
    await conn.execute(`CREATE VOLATILE TABLE "${table}" (ID INTEGER PRIMARY KEY, NAME VARCHAR(64))`);
    for (let i = 1; i <= 3; i++) await conn.execute(`INSERT INTO "${table}" VALUES (${i}, 'N${i}')`);
    const stmt = await conn.prepare(`SELECT NAME FROM "${table}" WHERE ID = ?`);
    try {
      for (const id of [1, 2, 3]) {
        const [rows] = await stmt.execute([id]);
        console.log(id, rows[0]?.NAME);
      }
    } finally { await stmt.close(); }
  } finally {
    await conn.execute(`DROP TABLE "${table}"`);
    await conn.end();
  }
})();
```

### Batch Append to a Log Table

```javascript
// append-batch.js
const { createConnection } = require('@machbase/ts-client');

(async () => {
  const conn = createConnection({ host: '127.0.0.1', user: 'SYS', password: 'MANAGER' });
  await conn.connect();
  const table = 'JS_LOGAPP_' + Math.random().toString(36).slice(2, 7).toUpperCase();
  try {
    await conn.execute(`CREATE LOG TABLE "${table}" (ID INTEGER, NAME VARCHAR(64), VALUE DOUBLE)`);
    const result = await conn.appendBatch(
      table,
      [ { name: 'ID', type: 'int32' }, { name: 'NAME', type: 'varchar' }, { name: 'VALUE', type: 'float64' } ],
      [ [1, 'X', 0.5], [2, 'Y', 1.25] ],
    );
    console.log(result);
  } finally {
    await conn.execute(`DROP TABLE "${table}"`);
    await conn.end();
  }
})();
```

### Streaming Append to a TAG Table

```javascript
// append-tag-stream.js
const { createConnection } = require('@machbase/ts-client');

(async () => {
  const conn = createConnection({ host: '127.0.0.1', user: 'SYS', password: 'MANAGER' });
  await conn.connect();
  const table = 'JS_TAG_' + Math.random().toString(36).slice(2, 7).toUpperCase();
  try {
    await conn.execute(`CREATE TAG TABLE "${table}" (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED)`);
    const stream = await conn.appendOpen(table, [
      { name: 'NAME', type: 'varchar' },
      { name: 'TIME', type: 'int64' },
      { name: 'VALUE', type: 'float64' },
    ]);
    const now = Date.now();
    await stream.append([
      ['T-0001', new Date(now), 1.0],
      ['T-0002', new Date(now + 1), 2.0],
    ]);
    await stream.close();
    const [rows] = await conn.query(`SELECT COUNT(*) AS CNT FROM "${table}"`);
    console.log('count', rows[0]?.CNT);
  } finally {
    await conn.execute(`DROP TABLE "${table}"`);
    await conn.end();
  }
})();
```

### Promise Wrapper + Ping

```javascript
// promise-and-ping.js
const { createConnection } = require('@machbase/ts-client');

(async () => {
  const conn = createConnection({ host: '127.0.0.1', user: 'SYS', password: 'MANAGER' });
  await conn.connect();
  try {
    const p = conn.promise();
    await p.ping(); // SELECT 1 FROM V$TABLES
    const [rows] = await p.query('SELECT NAME FROM V$TABLES ORDER BY NAME LIMIT ?', [5]);
    console.log(rows.map(r => r.NAME));
  } finally {
    await conn.end();
  }
})();
```

## Behaviour Notes & Limitations

### Transactions

Machbase autocommits every statement. Transactional keywords such as `BEGIN`, `COMMIT`, or `ROLLBACK` always fail, and the facade mirrors that behaviour by calling the supplied callback with a `QueryError` (`ERR_MACHBASE_NO_TX`).

```javascript
try {
  await conn.execute('COMMIT');
} catch (err) {
  console.log('Expected error:', err.message);
  // Error: Machbase does not support transactions
}
```

### Result Buffering & Pagination

The facade connection's `query` method buffers the entire rowset before resolving. For large tables, page manually with `ORDER BY … LIMIT` queries or primary-key ranges.

### Parameter Binding

Typed binds cover the portable scalar set (`int32`, `int64`, `float64`, and `varchar`). When supplying `null`, pair it with a concrete type:

```javascript
{ value: null, type: 'varchar' }
```

### Append Protocol

Use `appendBatch` for log tables and the streaming helper (`appendOpen` / `append`) for scenarios that need incremental ingest. When the server does not support the streaming protocol for a given table type (for example TAG tables), the facade automatically falls back to a prepared-statement loop.

### Error Handling

Errors propagate as standard `Error` objects (or `QueryError` when using the facade). Inspect `error.message` or the `QueryError` fields (`code`, `sql`) to diagnose issues.

### Table Type SQL Semantics

- **LOG and TAG tables** support `SELECT`, `INSERT`, and `DELETE`, but not `UPDATE`.
- **VOLATILE and LOOKUP tables** support all DML, but queries must include the primary key in `WHERE` clauses for correct index access and performance.

## Best Practices

1. **Always close connections**: Use try-finally blocks to ensure `conn.end()` is called.
2. **Reuse prepared statements**: Create them once and execute multiple times for better performance.
3. **Batch inserts**: Use `appendBatch` or `appendOpen` for bulk data loading instead of individual INSERT statements.
4. **Handle errors**: Wrap database operations in try-catch blocks and log errors appropriately.
5. **Use connection pooling**: For production applications, implement connection pooling to manage multiple concurrent requests.
6. **Parameterize queries**: Always use parameter binding (`?` placeholders) instead of string concatenation to prevent SQL injection.
