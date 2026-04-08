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
//   2. /proc/share/db.json — default shared location
//   3. Built-in defaults { host:'127.0.0.1', port:5656, user:'sys', password:'manager' }
//
// Shared DB layer decision (Phase A): machcli.js Client/Connection is reused
// directly. agent.js wraps it with limit enforcement and plain-object conversion.

const { Client } = require('machcli');
const fs = require('fs');
const _http = require('@jsh/http');
const vizspec = require('vizspec');

const DEFAULT_CONFIG_PATH = '/proc/share/db.json';
const DEFAULT_CONFIG = {
    host: '127.0.0.1',
    port: 5656,
    user: 'sys',
    password: 'manager',
};

// Default resource limits enforced by agent helpers.
const DEFAULT_MAX_ROWS = 1000;
const MODULE_DOCS_HOST = 'https://docs.machbase.com';
const MODULE_DOCS_BASE_PATH = '/neo/jsh/modules/';
const MODULE_DOCS_INDEX_URL = MODULE_DOCS_HOST + MODULE_DOCS_BASE_PATH + 'index.md';
const SQL_REFERENCE_HOST = 'https://docs.machbase.com';
const SQL_REFERENCE_BASE_PATH = '/dbms/sql-reference/';
const SQL_REFERENCE_INDEX_URL = SQL_REFERENCE_HOST + SQL_REFERENCE_BASE_PATH + 'index.md';

const KNOWN_MODULE_DOCS = {
    'archive': { summary: 'Archive module group for TAR and ZIP handling in JSH applications.' },
    'archive/tar': { summary: 'Create and extract TAR archives with memory, stream, and file APIs.' },
    'archive/zip': { summary: 'Create and extract ZIP archives with memory, stream, and file APIs.' },
    'events': { summary: 'EventEmitter utilities for event-driven JSH code.' },
    'fs': { summary: 'Filesystem APIs for file and directory operations.' },
    'http': { summary: 'HTTP client and server APIs.' },
    'machcli': { summary: 'Machbase database client APIs.' },
    'mathx/index': { summary: 'General numeric and statistical helpers.' },
    'mathx/filter': { summary: 'Stateful filters for sampled numeric data.' },
    'mathx/interp': { summary: 'Interpolation models for sample points.' },
    'mathx/mat': { summary: 'Matrix and vector APIs for linear algebra.' },
    'mathx/simplex': { summary: 'Seeded Simplex noise generator APIs.' },
    'mathx/spatial': { summary: 'Spatial helpers such as haversine distance.' },
    'mqtt': { summary: 'Event-driven MQTT client APIs.' },
    'nats': { summary: 'Event-driven NATS client APIs.' },
    'net': { summary: 'TCP client and server APIs.' },
    'opcua': { summary: 'OPC UA client APIs.' },
    'os': { summary: 'Operating system information APIs.' },
    'parser': { summary: 'Streaming CSV and NDJSON parser APIs.' },
    'path': { summary: 'Path manipulation helper APIs.' },
    'pretty': { summary: 'Terminal output formatting helpers.' },
    'process': { summary: 'Process, runtime, and lifecycle APIs.' },
    'readline': { summary: 'Interactive line input APIs.' },
    'semver': { summary: 'Semantic version comparison helpers.' },
    'service': { summary: 'Machbase Neo service controller client APIs.' },
    'util': { summary: 'Utility helpers including parseArgs and splitFields.' },
    'ws': { summary: 'WebSocket client APIs.' },
    'zlib': { summary: 'Compression and decompression APIs.' },
};

const KNOWN_SQL_REFERENCE_DOCS = {
    'datatypes': {
        title: 'Datatypes',
        summary: 'Machbase SQL datatype reference.',
        path: 'index.md',
    },
    'ddl': {
        title: 'DDL',
        summary: 'Machbase SQL DDL reference.',
        path: 'ddl.md',
    },
    'dml': {
        title: 'DML',
        summary: 'Machbase SQL DML reference.',
        path: 'ddl.md',
    },
    'math-functions': {
        title: 'Math Functions',
        summary: 'Machbase SQL math function reference.',
        path: 'ddl.md',
    },
    'functions': {
        title: 'Functions',
        summary: 'Machbase SQL function reference.',
        path: 'functions.md',
    },
};

// Phase C: read limits and capability config injected by Go at profile startup.
// Go sets globalThis.__agentConfig before require('repl/profiles/agent') is called.
const _agentConfig = (typeof globalThis.__agentConfig !== 'undefined' && globalThis.__agentConfig)
    ? globalThis.__agentConfig : null;
const _readOnly = _agentConfig ? Boolean(_agentConfig.readOnly) : false;
const _maxRows = (_agentConfig && _agentConfig.maxRows > 0) ? _agentConfig.maxRows : DEFAULT_MAX_ROWS;
const _maxOutputBytes = (_agentConfig && _agentConfig.maxOutputBytes > 0) ? _agentConfig.maxOutputBytes : 65536;
const _clientContext = (_agentConfig && _isPlainObject(_agentConfig.clientContext))
    ? JSON.parse(JSON.stringify(_agentConfig.clientContext)) : null;

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

    // connect(pathOrConfig?) — (re-)connect using a config path or override object.
    connect(pathOrConfig) {
        this.disconnect();
        if (typeof pathOrConfig === 'string') {
            this._configPath = pathOrConfig;
        }
        const cfg = _resolveConfig(this._configPath, pathOrConfig);
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

function _nowISOString() {
    return (new Date()).toISOString();
}

function _moduleNameToDocURL(name) {
    const mod = String(name);
    if (mod === 'index' || mod === '') {
        return MODULE_DOCS_INDEX_URL;
    }
    return MODULE_DOCS_HOST + MODULE_DOCS_BASE_PATH + mod + '.md';
}

function _sqlReferenceNameToDocURL(name) {
    const ref = _normalizeSQLReferenceName(name);
    if (ref === '' || ref === 'index' || ref === 'datatypes') {
        return SQL_REFERENCE_INDEX_URL;
    }
    const known = KNOWN_SQL_REFERENCE_DOCS[ref];
    if (known && known.path) {
        return SQL_REFERENCE_HOST + SQL_REFERENCE_BASE_PATH + known.path;
    }
    return SQL_REFERENCE_HOST + SQL_REFERENCE_BASE_PATH + ref + '.md';
}

function _normalizeModuleName(input) {
    if (input === undefined || input === null) {
        return '';
    }
    let name = String(input).trim();
    if (name.length === 0) {
        return '';
    }
    const idx = name.indexOf(MODULE_DOCS_BASE_PATH);
    if (idx >= 0) {
        name = name.slice(idx + MODULE_DOCS_BASE_PATH.length);
    }
    if (name.startsWith('/')) {
        name = name.slice(1);
    }
    if (name.endsWith('.md')) {
        name = name.slice(0, -3);
    }
    if (name === '' || name === 'index') {
        return 'index';
    }
    if (name === 'mathx') {
        return 'mathx/index';
    }
    return name;
}

function _normalizeSQLReferenceName(input) {
    if (input === undefined || input === null) {
        return '';
    }
    let name = String(input).trim().toLowerCase();
    if (name.length === 0) {
        return '';
    }
    const idx = name.indexOf(SQL_REFERENCE_BASE_PATH);
    if (idx >= 0) {
        name = name.slice(idx + SQL_REFERENCE_BASE_PATH.length);
    }
    if (name.startsWith('/')) {
        name = name.slice(1);
    }
    if (name.endsWith('.md')) {
        name = name.slice(0, -3);
    }
    if (name === '' || name === 'index') {
        return 'datatypes';
    }
    if (name === 'math function' || name === 'math functions' || name === 'math-function') {
        return 'math-functions';
    }
    return name;
}

function _httpGetText(url) {
    const client = _http.NewClient();
    const req = _http.NewRequest('GET', url);
    req.header.set('Accept', 'text/markdown, text/plain;q=0.9, */*;q=0.8');
    let resp = null;
    try {
        resp = client.do(req);
        const statusCode = resp.statusCode || 0;
        const ok = !!resp.ok;
        const markdown = resp.string();
        return {
            ok: ok,
            statusCode: statusCode,
            markdown: String(markdown || ''),
            url: url,
            fetchedAt: _nowISOString(),
        };
    } finally {
        if (resp && resp.close) {
            try { resp.close(); } catch (_) { }
        }
        if (client && client.close) {
            try { client.close(); } catch (_) { }
        }
    }
}

function _toPositiveIntOrNull(value) {
    if (value === undefined || value === null) {
        return null;
    }
    const n = Number(value);
    if (!isFinite(n)) {
        return null;
    }
    const i = Math.floor(n);
    if (i <= 0) {
        return null;
    }
    return i;
}

function _applyMarkdownOutputOptions(markdown, options) {
    const opts = options || {};
    const full = String(markdown || '');
    const maxBytes = _toPositiveIntOrNull(opts.maxBytes);
    const omitMarkdown = opts.omitMarkdown === true;

    let out = full;
    let truncated = false;
    if (maxBytes !== null && out.length > maxBytes) {
        out = out.slice(0, maxBytes);
        truncated = true;
    }

    const result = {
        bytes: out.length,
        originalBytes: full.length,
        truncated: truncated,
        omitMarkdown: omitMarkdown,
    };
    if (!omitMarkdown) {
        result.markdown = out;
    }
    return result;
}

function _parseModuleNamesFromIndex(markdown) {
    const text = String(markdown || '');
    const names = {};
    const re = /\[[^\]]+\]\(([^)]+)\)/g;
    let m;
    while ((m = re.exec(text)) !== null) {
        let href = m[1] || '';
        const hashPos = href.indexOf('#');
        if (hashPos >= 0) {
            href = href.slice(0, hashPos);
        }
        const qPos = href.indexOf('?');
        if (qPos >= 0) {
            href = href.slice(0, qPos);
        }
        if (href.startsWith(MODULE_DOCS_HOST + MODULE_DOCS_BASE_PATH)) {
            href = href.slice((MODULE_DOCS_HOST + MODULE_DOCS_BASE_PATH).length);
        } else if (href.startsWith(MODULE_DOCS_BASE_PATH)) {
            href = href.slice(MODULE_DOCS_BASE_PATH.length);
        } else {
            continue;
        }
        if (!href.endsWith('.md')) {
            continue;
        }
        const name = _normalizeModuleName(href);
        if (name.length > 0 && name !== 'index') {
            names[name] = true;
        }
    }
    return Object.keys(names).sort();
}

// AgentModuleDocsHelper provides online JSH module manual lookup.
// It fetches markdown pages from docs.machbase.com and can merge local
// known module metadata with links parsed from the latest index.md.
class AgentModuleDocsHelper {
    constructor() {
        this._indexCache = null;
    }

    _buildKnownList() {
        return Object.keys(KNOWN_MODULE_DOCS).sort().map(function (name) {
            return {
                name: name,
                summary: KNOWN_MODULE_DOCS[name].summary,
                url: _moduleNameToDocURL(name),
                source: 'builtin',
            };
        });
    }

    list(options) {
        const opts = options || {};
        const online = opts.online !== false;
        const force = !!opts.force;
        const map = {};
        this._buildKnownList().forEach(function (item) {
            map[item.name] = item;
        });

        let onlineError = null;
        let indexURL = MODULE_DOCS_INDEX_URL;
        if (online) {
            try {
                const idx = this.index(force);
                indexURL = idx.url;
                for (let i = 0; i < idx.modules.length; i++) {
                    const name = idx.modules[i];
                    if (!map[name]) {
                        map[name] = {
                            name: name,
                            summary: '',
                            url: _moduleNameToDocURL(name),
                            source: 'index',
                        };
                    }
                }
            } catch (err) {
                onlineError = String(err && err.message ? err.message : err);
            }
        }

        const modules = Object.keys(map).sort().map(function (name) { return map[name]; });
        return {
            modules: modules,
            count: modules.length,
            indexURL: indexURL,
            online: online,
            onlineError: onlineError,
            fetchedAt: _nowISOString(),
        };
    }

    index(forceOrOptions) {
        let forceRefresh = false;
        let outputOptions = {};
        if (typeof forceOrOptions === 'boolean') {
            forceRefresh = forceOrOptions;
        } else if (forceOrOptions && typeof forceOrOptions === 'object') {
            forceRefresh = !!forceOrOptions.force;
            outputOptions = forceOrOptions;
        }

        if (!forceRefresh && this._indexCache) {
            return this._indexCache;
        }
        const res = _httpGetText(MODULE_DOCS_INDEX_URL);
        if (!res.ok || res.statusCode < 200 || res.statusCode >= 300) {
            throw new Error('Failed to load module index: HTTP ' + res.statusCode);
        }
        const modules = _parseModuleNamesFromIndex(res.markdown);
        const output = _applyMarkdownOutputOptions(res.markdown, outputOptions);
        const payload = {
            url: res.url,
            statusCode: res.statusCode,
            fetchedAt: res.fetchedAt,
            modules: modules,
            count: modules.length,
            bytes: output.bytes,
            originalBytes: output.originalBytes,
            truncated: output.truncated,
            omitMarkdown: output.omitMarkdown,
        };
        if (output.markdown !== undefined) {
            payload.markdown = output.markdown;
        }
        this._indexCache = payload;
        return payload;
    }

    resolve(name) {
        const moduleName = _normalizeModuleName(name);
        const isIndex = (moduleName === '' || moduleName === 'index');
        const normalized = isIndex ? 'index' : moduleName;
        const url = _moduleNameToDocURL(normalized);
        const known = KNOWN_MODULE_DOCS[normalized];
        return {
            name: normalized,
            url: url,
            summary: known ? known.summary : '',
        };
    }

    fetch(name, options) {
        const ref = this.resolve(name);
        const res = _httpGetText(ref.url);
        if (!res.ok || res.statusCode < 200 || res.statusCode >= 300) {
            throw new Error('Failed to load module manual: ' + ref.name + ' (HTTP ' + res.statusCode + ')');
        }
        const output = _applyMarkdownOutputOptions(res.markdown, options);
        const doc = {
            name: ref.name,
            summary: ref.summary,
            url: ref.url,
            statusCode: res.statusCode,
            fetchedAt: res.fetchedAt,
            bytes: output.bytes,
            originalBytes: output.originalBytes,
            truncated: output.truncated,
            omitMarkdown: output.omitMarkdown,
        };
        if (output.markdown !== undefined) {
            doc.markdown = output.markdown;
        }
        return doc;
    }

    fetchAll(options) {
        const opts = options || {};
        let names = [];
        if (opts.modules && opts.modules.length) {
            names = opts.modules.map(_normalizeModuleName).filter(function (name) {
                return name.length > 0 && name !== 'index';
            });
        } else {
            const listing = this.list({
                online: opts.online !== false,
                force: !!opts.force,
            });
            names = listing.modules.map(function (m) { return m.name; });
        }

        const result = [];
        for (let i = 0; i < names.length; i++) {
            result.push(this.fetch(names[i], {
                maxBytes: opts.maxBytes,
                omitMarkdown: opts.omitMarkdown,
            }));
        }
        return {
            count: result.length,
            docs: result,
            fetchedAt: _nowISOString(),
        };
    }
}

class AgentSQLReferenceDocsHelper {
    list() {
        const docs = Object.keys(KNOWN_SQL_REFERENCE_DOCS).sort().map(function (name) {
            const item = KNOWN_SQL_REFERENCE_DOCS[name];
            return {
                name: name,
                title: item.title,
                summary: item.summary,
                url: _sqlReferenceNameToDocURL(name),
                source: 'builtin',
            };
        });
        return {
            docs: docs,
            count: docs.length,
            indexURL: SQL_REFERENCE_INDEX_URL,
            fetchedAt: _nowISOString(),
        };
    }

    index(options) {
        const res = _httpGetText(SQL_REFERENCE_INDEX_URL);
        if (!res.ok || res.statusCode < 200 || res.statusCode >= 300) {
            throw new Error('Failed to load SQL reference index: HTTP ' + res.statusCode);
        }
        const output = _applyMarkdownOutputOptions(res.markdown, options);
        const payload = {
            name: 'datatypes',
            title: KNOWN_SQL_REFERENCE_DOCS.datatypes.title,
            summary: KNOWN_SQL_REFERENCE_DOCS.datatypes.summary,
            url: res.url,
            statusCode: res.statusCode,
            fetchedAt: res.fetchedAt,
            bytes: output.bytes,
            originalBytes: output.originalBytes,
            truncated: output.truncated,
            omitMarkdown: output.omitMarkdown,
        };
        if (output.markdown !== undefined) {
            payload.markdown = output.markdown;
        }
        return payload;
    }

    resolve(name) {
        const refName = _normalizeSQLReferenceName(name);
        const known = KNOWN_SQL_REFERENCE_DOCS[refName];
        return {
            name: refName,
            title: known ? known.title : '',
            summary: known ? known.summary : '',
            url: _sqlReferenceNameToDocURL(refName),
        };
    }

    fetch(name, options) {
        const ref = this.resolve(name);
        const res = _httpGetText(ref.url);
        if (!res.ok || res.statusCode < 200 || res.statusCode >= 300) {
            throw new Error('Failed to load SQL reference: ' + ref.name + ' (HTTP ' + res.statusCode + ')');
        }
        const output = _applyMarkdownOutputOptions(res.markdown, options);
        const doc = {
            name: ref.name,
            title: ref.title,
            summary: ref.summary,
            url: ref.url,
            statusCode: res.statusCode,
            fetchedAt: res.fetchedAt,
            bytes: output.bytes,
            originalBytes: output.originalBytes,
            truncated: output.truncated,
            omitMarkdown: output.omitMarkdown,
        };
        if (output.markdown !== undefined) {
            doc.markdown = output.markdown;
        }
        return doc;
    }

    fetchAll(options) {
        const opts = options || {};
        let names = [];
        if (opts.docs && opts.docs.length) {
            names = opts.docs.map(_normalizeSQLReferenceName).filter(function (name) {
                return name.length > 0;
            });
        } else {
            names = this.list().docs.map(function (doc) { return doc.name; });
        }

        const result = [];
        for (let i = 0; i < names.length; i++) {
            result.push(this.fetch(names[i], {
                maxBytes: opts.maxBytes,
                omitMarkdown: opts.omitMarkdown,
            }));
        }
        return {
            count: result.length,
            docs: result,
            fetchedAt: _nowISOString(),
        };
    }
}

function _toPositiveIntOption(options, key, defaultValue) {
    if (!options || options[key] === undefined || options[key] === null || options[key] === '') {
        return defaultValue;
    }
    const n = Number(options[key]);
    if (!isFinite(n) || n <= 0) {
        throw new Error('agent.viz: option "' + key + '" must be greater than 0');
    }
    return Math.floor(n);
}

function _toBooleanOption(options, key, defaultValue) {
    if (!options || options[key] === undefined || options[key] === null || options[key] === '') {
        return defaultValue;
    }
    return Boolean(options[key]);
}

function _toStringOption(options, key, defaultValue) {
    if (!options || options[key] === undefined || options[key] === null || options[key] === '') {
        return defaultValue;
    }
    return String(options[key]);
}

function _normalizeVizOptions(options) {
    if (options === undefined || options === null) {
        return {};
    }
    if (!_isPlainObject(options)) {
        throw new Error('agent.viz: options must be an object');
    }
    return options;
}

function _buildRenderEnvelope(mode, payload, meta) {
    const envelope = {
        __agentRender: true,
        schema: 'agent-render/v1',
        renderer: 'viz.tui',
        mode: mode,
        meta: meta || {},
    };
    if (mode === 'blocks') {
        envelope.blocks = payload;
    } else {
        envelope.lines = payload;
    }
    if (meta && meta.title) {
        envelope.title = meta.title;
    }
    return envelope;
}

class AgentVizHelper {
    render(spec, options) {
        const opts = _normalizeVizOptions(options);
        const mode = _toStringOption(opts, 'mode', 'blocks').toLowerCase();
        if (mode === 'lines') {
            return this.lines(spec, opts);
        }
        if (mode !== 'blocks') {
            throw new Error('agent.viz: unsupported mode "' + mode + '"');
        }
        return this.blocks(spec, opts);
    }

    blocks(spec, options) {
        const opts = _normalizeVizOptions(options);
        try {
            vizspec.validate(spec);
            const renderOptions = {
                compact: _toBooleanOption(opts, 'compact', true),
                rows: _toPositiveIntOption(opts, 'rows', 8),
                width: _toPositiveIntOption(opts, 'width', 40),
                timeformat: _toStringOption(opts, 'timeformat', ''),
                tz: _toStringOption(opts, 'tz', ''),
            };
            if (renderOptions.timeformat === '') {
                delete renderOptions.timeformat;
            }
            if (renderOptions.tz === '') {
                delete renderOptions.tz;
            }
            const blocks = vizspec.toTUIBlocks(spec, renderOptions);
            const listed = vizspec.listSeries(spec);
            const meta = {
                blockCount: Array.isArray(blocks) ? blocks.length : 0,
                seriesCount: Array.isArray(listed) ? listed.length : 0,
                title: _toStringOption(opts, 'title', ''),
            };
            return _buildRenderEnvelope('blocks', blocks, meta);
        } catch (err) {
            throw new Error('viz render failed: ' + (err && err.message ? err.message : String(err)));
        }
    }

    lines(spec, options) {
        const opts = _normalizeVizOptions(options);
        try {
            vizspec.validate(spec);
            const renderOptions = {
                height: _toPositiveIntOption(opts, 'height', 3),
                width: _toPositiveIntOption(opts, 'width', 40),
                timeformat: _toStringOption(opts, 'timeformat', ''),
                tz: _toStringOption(opts, 'tz', ''),
            };
            const seriesId = _toStringOption(opts, 'series', '');
            if (seriesId) {
                renderOptions.seriesId = seriesId;
            }
            if (renderOptions.timeformat === '') {
                delete renderOptions.timeformat;
            }
            if (renderOptions.tz === '') {
                delete renderOptions.tz;
            }
            const lines = vizspec.toTUILines(spec, renderOptions);
            const listed = vizspec.listSeries(spec);
            const meta = {
                lineCount: Array.isArray(lines) ? lines.length : 0,
                seriesCount: Array.isArray(listed) ? listed.length : 0,
                seriesId: renderOptions.seriesId || '',
                title: _toStringOption(opts, 'title', ''),
            };
            return _buildRenderEnvelope('lines', lines, meta);
        } catch (err) {
            throw new Error('viz render failed: ' + (err && err.message ? err.message : String(err)));
        }
    }

    // fromRows(rows, options) — convenience high-level API.
    // Builds a raw-point vizspec from a plain array of row objects and renders it.
    //
    // options:
    //   x        (string, required)  — field name for the X axis (e.g. 'TIME')
    //   y        (string|string[])   — one or more field names for Y series (e.g. ['LAT', 'LON'])
    //   mode     ('blocks'|'lines')  — render mode, default 'lines'
    //   width    (number)            — render width  (default 80)
    //   height   (number)            — render height for lines mode (default 10)
    //   rows     (number)            — row count for blocks mode (default 8)
    //   compact  (boolean)           — compact blocks (default true)
    //   timeformat (string)          — 'rfc3339'|'s'|'ms'|'us'|'ns'
    //   tz       (string)            — timezone
    //   title    (string)            — chart title
    fromRows(rows, options) {
        const opts = _normalizeVizOptions(options);
        const xField = _toStringOption(opts, 'x', '');
        if (!xField) {
            throw new Error('agent.viz.fromRows: options.x (x-axis field name) is required');
        }
        if (!Array.isArray(rows) || rows.length === 0) {
            throw new Error('agent.viz.fromRows: rows must be a non-empty array');
        }

        // Determine y fields — accept a single string or an array.
        let yFields = opts['y'];
        if (typeof yFields === 'string' && yFields.length > 0) {
            yFields = [yFields];
        }
        if (!Array.isArray(yFields) || yFields.length === 0) {
            // Auto-detect: all numeric fields except the x field.
            const sample = rows[0];
            yFields = Object.keys(sample).filter(function (k) {
                return k !== xField && typeof sample[k] === 'number';
            });
            if (yFields.length === 0) {
                throw new Error('agent.viz.fromRows: no numeric y fields found; specify options.y explicitly');
            }
        }

        // Build one raw-point series per y field.
        // Data layout: each row becomes [xValue, yValue].
        const seriesList = yFields.map(function (yField) {
            return {
                id: yField,
                name: yField,
                representation: {
                    kind: 'raw-point',
                    fields: [xField, yField],
                },
                data: rows.map(function (r) { return [r[xField], r[yField]]; }),
            };
        });

        const spec = vizspec.createSpec({ series: seriesList });

        const mode = _toStringOption(opts, 'mode', 'lines').toLowerCase();
        if (mode === 'blocks') {
            return this.blocks(spec, opts);
        }
        return this.lines(spec, opts);
    }

    help() {
        return [
            'agent.viz.render(spec[, options])           Render VIZSPEC into a structured TUI envelope',
            'agent.viz.blocks(spec[, options])           Render VIZSPEC as TUI blocks envelope',
            'agent.viz.lines(spec[, options])            Render VIZSPEC as TUI lines envelope',
            'agent.viz.fromRows(rows, options)           High-level: build spec from row array and render',
            '  required: options.x (x-axis field name)',
            '  optional: options.y (string or string[] of y field names — auto-detected if omitted)',
            '  optional: options.mode ("blocks"|"lines", default "lines")',
            'options: mode, compact, rows, width, height, series, timeformat, tz, title',
            'Returns: { __agentRender:true, schema:"agent-render/v1", renderer:"viz.tui", ... }',
        ].join('\n');
    }
}

const _db = new AgentDbHelper();
const _schema = new AgentSchemaHelper(_db);
const _modules = new AgentModuleDocsHelper();
const _sqlref = new AgentSQLReferenceDocsHelper();
const _viz = new AgentVizHelper();

// _helpText for agent.help().
const _helpText = {
    '': [
        'agent — Machbase Neo agent helper namespace',
        '',
        '  agent.db.query(sql, ...params)     SELECT → { rows:[...], truncated, count }',
        '  agent.db.queryRow(sql, ...params)  SELECT → first row object, or null',
        '  agent.db.exec(sql, ...params)      DDL/DML → { rowsAffected, message }',
        '                                     (denied when read-only mode is active)',
        '  agent.db.connect([configPathOrConfig])',
        '                                     (Re-)connect, override config file or fields',
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
        '  agent.runtime.clientContext       Caller surface/transport/render target hints, when provided',
        '',
        '  agent.modules.list([options])     List module manuals with URLs (online + builtin)',
        '  agent.modules.index([force|options]) Fetch index.md and parsed module names',
        '  agent.modules.resolve(name)       Resolve module name/url/summary',
        '  agent.modules.fetch(name[, options]) Fetch module markdown reference from docs site',
        '  agent.modules.fetchAll([options]) Fetch all module markdown references',
        '',
        '  agent.sqlref.list()               List Machbase SQL reference docs and URLs',
        '  agent.sqlref.index([options])     Fetch SQL reference index markdown',
        '  agent.sqlref.resolve(name)        Resolve SQL reference name/url/summary',
        '  agent.sqlref.fetch(name[, options]) Fetch one SQL reference markdown page',
        '  agent.sqlref.fetchAll([options])  Fetch all configured SQL reference markdown pages',
        '',
        '  agent.viz.render(spec[, options])        Render VIZSPEC into a TUI envelope ({__agentRender:true,...})',
        '  agent.viz.blocks(spec[, options])        Render VIZSPEC as TUI blocks envelope',
        '  agent.viz.lines(spec[, options])         Render VIZSPEC as TUI lines envelope',
        '  agent.viz.fromRows(rows, {x, y?, mode?}) High-level: build spec from row array and render',
        '',
        '  agent.help([topic])               Show this help',
        '',
        'Connection config: /proc/share/db.json  (or override path/fields with connect())',
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
    modules: _modules,
    sqlref: _sqlref,
    viz: _viz,

    runtime: {
        clientContext: _clientContext,
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
