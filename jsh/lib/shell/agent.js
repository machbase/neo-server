'use strict';

// repl/profiles/agent — Machine-readable helper namespace for LLM agent use.
//
// Key differences from user.*:
//   - agent.db.query() returns { rows: [...], truncated: bool, count: N }
//     All rows are plain objects (no Rows iterator), safe to serialize directly.
//   - agent.schema.*  provides schema inspection under a dedicated namespace.
//   - agent.runtime.* exposes capability and resource limit metadata.
//   - exec() is available but annotated as a write operation.
//
// Connection config resolution (same order as user.*):
//   1. agent.db.connect(path) — explicit path
//   2. /share/database/machcli.json — default shared location
//   3. Built-in defaults { host:'127.0.0.1', port:5656, user:'sys', password:'manager' }
//
// Shared DB layer decision (Phase A): machcli.js Client/Connection is reused
// directly. agent.js wraps it with limit enforcement and plain-object conversion.

const { Client } = require('machcli');
const fs = require('fs');

const DEFAULT_CONFIG_PATH = '/share/database/machcli.json';
const DEFAULT_CONFIG = {
    host: '127.0.0.1',
    port: 5656,
    user: 'sys',
    password: 'manager',
};

// Default resource limits enforced by agent helpers.
const DEFAULT_MAX_ROWS = 1000;

// Phase C: read limits and capability config injected by Go at profile startup.
// Go sets globalThis.__agentConfig before require('repl/profiles/agent') is called.
const _agentConfig = (typeof globalThis.__agentConfig !== 'undefined' && globalThis.__agentConfig)
    ? globalThis.__agentConfig : null;
const _readOnly = _agentConfig ? Boolean(_agentConfig.readOnly) : false;
const _maxRows = (_agentConfig && _agentConfig.maxRows > 0) ? _agentConfig.maxRows : DEFAULT_MAX_ROWS;
const _maxOutputBytes = (_agentConfig && _agentConfig.maxOutputBytes > 0) ? _agentConfig.maxOutputBytes : 65536;

// _loadConfig reads a JSON config file, falling back to DEFAULT_CONFIG.
function _loadConfig(path) {
    try {
        const raw = fs.readFileSync(path, 'utf8');
        return Object.assign({}, DEFAULT_CONFIG, JSON.parse(raw));
    } catch (_) {
        return Object.assign({}, DEFAULT_CONFIG);
    }
}

// _rowToObject converts a machcli Row instance to a plain serializable object.
// Row sets column values directly as properties (row[name] = value),
// so we copy them via row.names rather than iterating Symbol.iterator
// (which yields raw values, not {key,value} pairs).
function _rowToObject(row) {
    const obj = {};
    const names = row.names;
    for (let i = 0; i < names.length; i++) {
        obj[names[i]] = row[names[i]];
    }
    return obj;
}

// AgentDbHelper wraps machcli Connection with limit enforcement.
class AgentDbHelper {
    constructor(maxRows) {
        this._configPath = DEFAULT_CONFIG_PATH;
        this._client = null;
        this._conn = null;
        this._maxRows = (maxRows !== undefined) ? maxRows : _maxRows;
    }

    // connect(path?) — (re-)connect, optionally overriding config file path.
    connect(path) {
        this.disconnect();
        if (path !== undefined) {
            this._configPath = path;
        }
        const cfg = _loadConfig(this._configPath);
        this._client = new Client(cfg);
        this._conn = this._client.connect();
        return this;
    }

    _ensureConn() {
        if (!this._conn) { this.connect(); }
        return this._conn;
    }

    // disconnect() — close connection and client.
    disconnect() {
        if (this._conn) {
            try { this._conn.close(); } catch (_) { }
            this._conn = null;
        }
        if (this._client) {
            try { this._client.close(); } catch (_) { }
            this._client = null;
        }
    }

    // query(sql, ...params) — execute SELECT.
    // Returns { rows: [...], truncated: bool, count: N }.
    // All row values are plain objects. Automatically enforces maxRows limit.
    query(sql) {
        const args = Array.prototype.slice.call(arguments);
        const dbRows = this._ensureConn().query.apply(this._conn, args);
        const rows = [];
        let truncated = false;
        try {
            for (const row of dbRows) {
                if (rows.length >= this._maxRows) {
                    truncated = true;
                    break;
                }
                rows.push(_rowToObject(row));
            }
        } finally {
            dbRows.close();
        }
        return { rows: rows, truncated: truncated, count: rows.length };
    }

    // queryRow(sql, ...params) — execute SELECT, return first row as plain object.
    // Returns null if no rows available.
    queryRow(sql) {
        const args = Array.prototype.slice.call(arguments);
        try {
            const row = this._ensureConn().queryRow.apply(this._conn, args);
            return _rowToObject(row);
        } catch (_) {
            return null;
        }
    }

    // exec(sql, ...params) — execute DDL/DML.
    // Returns { rowsAffected, message }.
    // Write operations are denied when read-only mode is active (Phase C).
    exec(sql) {
        if (_readOnly) {
            throw new Error('capability denied: db.write is not allowed in read-only mode');
        }
        const args = Array.prototype.slice.call(arguments);
        return this._ensureConn().exec.apply(this._conn, args);
    }
}

// AgentSchemaHelper provides schema inspection under agent.schema.*.
// It shares the DB connection with AgentDbHelper.
class AgentSchemaHelper {
    constructor(dbHelper) {
        this._db = dbHelper;
    }

    // tables([pattern]) — list tables, optionally filtered by LIKE pattern.
    // Returns array of plain row objects with NAME, TYPE, FLAG fields.
    // Only returns tables in the local database (DATABASE_ID = -1).
    tables(pattern) {
        let sql = 'SELECT NAME, TYPE, FLAG FROM M$SYS_TABLES WHERE DATABASE_ID = -1';
        const params = [];
        if (pattern !== undefined) {
            sql += ' AND NAME LIKE ?';
            params.push(String(pattern).toUpperCase());
        }
        sql += ' ORDER BY NAME';
        const dbRows = this._db._ensureConn().query.apply(this._db._conn, [sql].concat(params));
        const result = [];
        try {
            for (const row of dbRows) {
                result.push(_rowToObject(row));
            }
        } finally {
            dbRows.close();
        }
        return result;
    }

    // describe(tableName) — return column metadata for a table.
    // Returns array of plain row objects: { NAME, TYPE, LENGTH, FLAG }.
    // TYPE and FLAG are integer codes from M$SYS_COLUMNS.
    // Handles V$/M$ virtual/meta tables using V$TABLES/V$COLUMNS or M$TABLES/M$COLUMNS.
    // Follows the same dispatch pattern as show.js:showTable().
    describe(tableName) {
        const conn = this._db._ensureConn();
        const name = String(tableName).toUpperCase();

        if (name.startsWith('V$') || name.startsWith('M$')) {
            return this._describeMVTable(conn, name);
        }
        return this._describeTable(conn, name);
    }

    // _describeTable handles regular user tables via M$SYS_TABLES + M$SYS_COLUMNS.
    _describeTable(conn, name) {
        // Step 1: resolve TABLE_ID from M$SYS_TABLES (DATABASE_ID = -1 = local DB).
        const tblRow = conn.queryRow(
            'SELECT ID FROM M$SYS_TABLES WHERE NAME = ? AND DATABASE_ID = -1',
            name
        );
        if (!tblRow || !tblRow.ID) {
            throw new Error('Table not found: ' + name);
        }
        const tableId = tblRow.ID;

        // Step 2: fetch columns by TABLE_ID, exclude internal hidden columns (_xxx).
        const dbRows = conn.query(
            'SELECT NAME, TYPE, LENGTH, FLAG FROM M$SYS_COLUMNS' +
            ' WHERE TABLE_ID = ? AND DATABASE_ID = -1 AND SUBSTR(NAME, 1, 1) <> \'_\'' +
            ' ORDER BY ID',
            tableId
        );
        const result = [];
        try {
            for (const row of dbRows) {
                result.push(_rowToObject(row));
            }
        } finally {
            dbRows.close();
        }
        return result;
    }

    // _describeMVTable handles V$/M$ virtual and meta tables.
    // V$ tables use V$TABLES + V$COLUMNS; M$ tables use M$TABLES + M$COLUMNS.
    // These catalog views do not expose a FLAG column.
    _describeMVTable(conn, name) {
        const tablesView = name.startsWith('M$') ? 'M$TABLES' : 'V$TABLES';
        const columnsView = name.startsWith('M$') ? 'M$COLUMNS' : 'V$COLUMNS';

        const tblRow = conn.queryRow(
            'SELECT ID FROM ' + tablesView + ' WHERE NAME = ?',
            name
        );
        if (!tblRow || !tblRow.ID) {
            throw new Error('Table not found: ' + name);
        }
        const tableId = tblRow.ID;

        const dbRows = conn.query(
            'SELECT NAME, TYPE, LENGTH FROM ' + columnsView +
            ' WHERE TABLE_ID = ? AND SUBSTR(NAME, 1, 1) <> \'_\'' +
            ' ORDER BY ID',
            tableId
        );
        const result = [];
        try {
            for (const row of dbRows) {
                result.push(_rowToObject(row));
            }
        } finally {
            dbRows.close();
        }
        return result;
    }
}

const _db = new AgentDbHelper();
const _schema = new AgentSchemaHelper(_db);

// _helpText for agent.help().
const _helpText = {
    '': [
        'agent — Machbase Neo agent helper namespace',
        '',
        '  agent.db.query(sql, ...params)     SELECT → { rows:[...], truncated, count }',
        '  agent.db.queryRow(sql, ...params)  SELECT → first row object, or null',
        '  agent.db.exec(sql, ...params)      DDL/DML → { rowsAffected, message }',
        '                                     (denied when read-only mode is active)',
        '  agent.db.connect([configPath])     (Re-)connect, override config file',
        '  agent.db.disconnect()              Close connection',
        '',
        '  agent.schema.tables([pattern])    List tables → [{ NAME, TYPE, FLAG }]',
        '  agent.schema.describe(tableName)  Column info → [{ NAME, TYPE, LENGTH, FLAG }]',
        '',
        'NOTE: All field names in query results and schema objects are UPPERCASE.',
        'Use t.NAME, t.TYPE, row.COLUMN_NAME, etc.',
        '',
        '  agent.runtime.capabilities()      List allowed operation categories',
        '  agent.runtime.limits()            Current resource limits (maxRows, maxOutputBytes, readOnly)',
        '',
        '  agent.help([topic])               Show this help',
        '',
        'Connection config: /share/database/machcli.json  (or override with connect())',
        'All query results are plain objects, safe to serialize.',
        'query() enforces maxRows limit to prevent unbounded result sets.',
        '',
        'Capability and limits are set at session startup via CLI options:',
        '  --read-only           Deny write operations (exec)',
        '  --max-rows N          Override query row limit (default: 1000)',
        '  --max-output-bytes N  Override output size limit (default: 65536)',
        '  --timeout N           Per-evaluation timeout in milliseconds',
        '  --transcript FILE     Record inputs/results as NDJSON to file',
    ].join('\n'),
};

// agent — the exported namespace object.
const agent = {
    db: _db,
    schema: _schema,

    runtime: {
        // capabilities() — list allowed operation categories for this profile.
        capabilities: function () {
            const caps = ['db.read', 'db.schema'];
            if (!_readOnly) { caps.push('db.write'); }
            return caps;
        },
        // limits() — current resource limits enforced by this profile.
        limits: function () {
            return {
                maxRows: _maxRows,
                maxOutputBytes: _maxOutputBytes,
                readOnly: _readOnly,
            };
        },
    },

    help: function (topic) {
        const key = (topic === undefined || topic === null) ? '' : String(topic);
        const text = _helpText[key];
        if (text !== undefined) {
            console.println(text);
        } else {
            console.println('No help for topic: ' + key);
            console.println('Available topics: ' + Object.keys(_helpText).map(function (k) {
                return k === '' ? '(default)' : k;
            }).join(', '));
        }
    },
};

module.exports = agent;
