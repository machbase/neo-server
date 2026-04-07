'use strict';

// repl/profiles/user — Human operator helper namespace for the JSH REPL.
//
// Provides ergonomic wrappers around machcli for interactive use.
// Connections are managed lazily: the first call to any user.db.* method
// opens the connection; subsequent calls reuse it; user.db.disconnect()
// closes it explicitly.
//
// Connection config search order:
//   1. Path passed to user.db.connect(path)
//   2. /proc/share/db.json  (default shared location)
//   3. Built-in defaults { host:'127.0.0.1', port:5656, user:'sys', password:'manager' }

const { Client } = require('machcli');
const fs = require('fs');

const DEFAULT_CONFIG_PATH = '/proc/share/db.json';
const DEFAULT_CONFIG = {
    host: '127.0.0.1',
    port: 5656,
    user: 'sys',
    password: 'manager',
};

// _loadConfig reads a JSON config file, falling back to DEFAULT_CONFIG.
function _loadConfig(path) {
    try {
        const raw = fs.readFileSync(path, 'utf8');
        return Object.assign({}, DEFAULT_CONFIG, JSON.parse(raw));
    } catch (_) {
        return Object.assign({}, DEFAULT_CONFIG);
    }
}

function _isPlainObject(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function _resolveConfig(configPath, pathOrConfig) {
    if (_isPlainObject(pathOrConfig)) {
        return Object.assign({}, _loadConfig(configPath), pathOrConfig);
    }
    if (pathOrConfig !== undefined) {
        return _loadConfig(String(pathOrConfig));
    }
    return _loadConfig(configPath);
}

// DbHelper manages a lazy machcli Client + Connection pair.
class DbHelper {
    constructor() {
        this._configPath = DEFAULT_CONFIG_PATH;
        this._client = null;
        this._conn = null;
    }

    // connect(pathOrConfig?) — (re-)connect using config at path or merged override fields.
    // Safe to call multiple times: closes any existing connection first.
    connect(pathOrConfig) {
        this.disconnect();
        if (typeof pathOrConfig === 'string') {
            this._configPath = pathOrConfig;
        }
        const cfg = _resolveConfig(this._configPath, pathOrConfig);
        this._client = new Client(cfg);
        this._conn = this._client.connect();
        return this; // chainable
    }

    // _ensureConn — opens the connection lazily on first use.
    _ensureConn() {
        if (!this._conn) {
            this.connect();
        }
        return this._conn;
    }

    // disconnect() — close connection and client, reset state.
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

    // query(sql, ...params) — execute SELECT, return an iterable Rows object.
    query(sql) {
        const args = Array.prototype.slice.call(arguments);
        return this._ensureConn().query.apply(this._conn, args);
    }

    // queryRow(sql, ...params) — execute SELECT, return the first row as an object.
    queryRow(sql) {
        const args = Array.prototype.slice.call(arguments);
        return this._ensureConn().queryRow.apply(this._conn, args);
    }

    // exec(sql, ...params) — execute DDL/DML, return { rowsAffected, message }.
    exec(sql) {
        const args = Array.prototype.slice.call(arguments);
        return this._ensureConn().exec.apply(this._conn, args);
    }

    // describe(tableName) — return column metadata for a table.
    // Handles V$/M$ virtual/meta tables using V$TABLES/V$COLUMNS or M$TABLES/M$COLUMNS.
    // Follows the same dispatch pattern as show.js:showTable().
    describe(tableName) {
        const conn = this._ensureConn();
        const name = tableName.toUpperCase();
        if (name.startsWith('V$') || name.startsWith('M$')) {
            return this._describeMVTable(conn, name);
        }
        return this._describeTable(conn, name);
    }

    _describeTable(conn, name) {
        // Step 1: resolve TABLE_ID.
        const tblRow = conn.queryRow(
            'SELECT ID FROM M$SYS_TABLES WHERE NAME = ? AND DATABASE_ID = -1',
            name
        );
        if (!tblRow || !tblRow.ID) {
            throw new Error('Table not found: ' + name);
        }
        const tableId = tblRow.ID;

        // Step 2: fetch columns, exclude internal hidden columns (_xxx).
        const rows = conn.query(
            'SELECT NAME, TYPE, LENGTH, FLAG FROM M$SYS_COLUMNS' +
            ' WHERE TABLE_ID = ? AND DATABASE_ID = -1 AND SUBSTR(NAME, 1, 1) <> \'_\'' +
            ' ORDER BY ID',
            tableId
        );
        const result = [];
        for (const row of rows) {
            result.push({ name: row.NAME, type: row.TYPE, length: row.LENGTH, flag: row.FLAG });
        }
        rows.close();
        return result;
    }

    _describeMVTable(conn, name) {
        const tablesView = name.startsWith('M$') ? 'M$TABLES' : 'V$TABLES';
        const columnsView = name.startsWith('M$') ? 'M$COLUMNS' : 'V$COLUMNS';

        const tblRow = conn.queryRow(
            'SELECT ID FROM ' + tablesView + ' WHERE NAME = ?', name
        );
        if (!tblRow || !tblRow.ID) {
            throw new Error('Table not found: ' + name);
        }
        const tableId = tblRow.ID;

        const rows = conn.query(
            'SELECT NAME, TYPE, LENGTH FROM ' + columnsView +
            ' WHERE TABLE_ID = ? AND SUBSTR(NAME, 1, 1) <> \'_\'' +
            ' ORDER BY ID',
            tableId
        );
        const result = [];
        for (const row of rows) {
            result.push({ name: row.NAME, type: row.TYPE, length: row.LENGTH });
        }
        rows.close();
        return result;
    }

    // tables(pattern?) — list tables, optionally filtered by LIKE pattern.
    // Only returns tables in the local database (DATABASE_ID = -1).
    tables(pattern) {
        const conn = this._ensureConn();
        let sql = 'SELECT NAME, TYPE, FLAG FROM M$SYS_TABLES WHERE DATABASE_ID = -1';
        const params = [];
        if (pattern !== undefined) {
            sql += ' AND NAME LIKE ?';
            params.push(pattern.toUpperCase());
        }
        sql += ' ORDER BY NAME';
        const rows = conn.query.apply(conn, [sql].concat(params));
        const result = [];
        for (const row of rows) {
            result.push({ name: row.NAME, type: row.TYPE, flag: row.FLAG });
        }
        rows.close();
        return result;
    }
}

// _helpText is the built-in help string printed by user.help().
const _helpText = {
    '': [
        'user — Machbase Neo helper namespace',
        '',
        '  user.db.query(sql, ...params)       Execute SELECT, returns iterable Rows',
        '  user.db.queryRow(sql, ...params)    Execute SELECT, returns first Row',
        '  user.db.exec(sql, ...params)        Execute DDL/DML',
        '  user.db.describe(tableName)         List columns of a table',
        '  user.db.tables([pattern])           List tables (optional LIKE pattern)',
        '  user.db.connect([configPathOrConfig])',
        '                                     (Re-)connect, override config file or fields',
        '  user.db.disconnect()                Close connection',
        '',
        '  user.help()                         Show this help',
        '  user.help("db")                     Show db sub-command help',
        '',
        'Connection config: /proc/share/db.json  (or override path/fields with connect())',
    ].join('\n'),
    'db': [
        'user.db — database helper',
        '',
        '  query(sql, ...params)    → Rows (iterable, call rows.close() when done)',
        '  queryRow(sql, ...params) → Row object with column properties',
        '  exec(sql, ...params)     → { rowsAffected, message }',
        '  describe(tableName)      → [{ name, type, length, nullable }, ...]',
        '  tables([pattern])        → [{ name, type, flag }, ...]',
        '  connect([configPathOrConfig])',
        '                         → reconnect; reads JSON config file and merges override fields',
        '  disconnect()             → close connection and client',
        '',
        'Example:',
        '  const rows = user.db.query("SELECT NAME, TIME, VALUE FROM TAG LIMIT ?", 5)',
        '  for (const row of rows) { console.println(row.NAME, row.TIME, row.VALUE) }',
        '  rows.close()',
    ].join('\n'),
};

// user — the exported namespace object.
const user = {
    db: new DbHelper(),

    // help(topic?) — print help. topic: '' (default) or 'db'.
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

module.exports = user;
