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
//   2. /share/database/machcli.json  (default shared location)
//   3. Built-in defaults { host:'127.0.0.1', port:5656, user:'sys', password:'manager' }

const { Client } = require('machcli');
const fs = require('fs');

const DEFAULT_CONFIG_PATH = '/share/database/machcli.json';
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

// DbHelper manages a lazy machcli Client + Connection pair.
class DbHelper {
    constructor() {
        this._configPath = DEFAULT_CONFIG_PATH;
        this._client = null;
        this._conn = null;
    }

    // connect(path?) — (re-)connect using config at path, or the default location.
    // Safe to call multiple times: closes any existing connection first.
    connect(path) {
        this.disconnect();
        if (path !== undefined) {
            this._configPath = path;
        }
        const cfg = _loadConfig(this._configPath);
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
    describe(tableName) {
        const conn = this._ensureConn();
        const rows = conn.query(
            'SELECT COLUMN_NAME, DATA_TYPE, LENGTH, IS_NULLABLE' +
            '  FROM M$SYS_COLUMNS c' +
            '  JOIN M$SYS_TABLES t ON c.TABLE_ID = t.ID' +
            ' WHERE t.NAME = ?',
            tableName.toUpperCase()
        );
        const result = [];
        for (const row of rows) {
            result.push({
                name: row.COLUMN_NAME,
                type: row.DATA_TYPE,
                length: row.LENGTH,
                nullable: row.IS_NULLABLE === 1,
            });
        }
        rows.close();
        return result;
    }

    // tables(pattern?) — list tables, optionally filtered by LIKE pattern.
    tables(pattern) {
        const conn = this._ensureConn();
        let sql = 'SELECT NAME, TYPE, FLAG FROM M$SYS_TABLES';
        const params = [];
        if (pattern !== undefined) {
            sql += ' WHERE NAME LIKE ?';
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
        '  user.db.connect([configPath])       (Re-)connect, override config file',
        '  user.db.disconnect()                Close connection',
        '',
        '  user.help()                         Show this help',
        '  user.help("db")                     Show db sub-command help',
        '',
        'Connection config: /share/database/machcli.json  (or override with connect())',
    ].join('\n'),
    'db': [
        'user.db — database helper',
        '',
        '  query(sql, ...params)    → Rows (iterable, call rows.close() when done)',
        '  queryRow(sql, ...params) → Row object with column properties',
        '  exec(sql, ...params)     → { rowsAffected, message }',
        '  describe(tableName)      → [{ name, type, length, nullable }, ...]',
        '  tables([pattern])        → [{ name, type, flag }, ...]',
        '  connect([configPath])    → reconnect; reads JSON config file',
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
