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
const path = require('path');
const process = require('process');
const splitFields = require('util/splitFields');
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

const READ_ONLY_EXEC_ALLOW = {
    ls: true,
    cat: true,
    pwd: true,
    echo: true,
    wc: true,
    head: true,
    tail: true,
};

function _normalizeWorkspaceRoot(candidate) {
    const fallback = '/';
    if (candidate === undefined || candidate === null) {
        return fallback;
    }
    const text = String(candidate).trim();
    if (!text) {
        return fallback;
    }
    const resolved = path.resolve(text);
    if (!resolved) {
        return fallback;
    }
    return resolved;
}

function _detectWorkspaceRoot() {
    if (_agentConfig && typeof _agentConfig.workspaceRoot === 'string' && _agentConfig.workspaceRoot.trim() !== '') {
        return _normalizeWorkspaceRoot(_agentConfig.workspaceRoot);
    }
    if (_agentConfig && typeof _agentConfig.cwd === 'string' && _agentConfig.cwd.trim() !== '') {
        return _normalizeWorkspaceRoot(_agentConfig.cwd);
    }
    try {
        return _normalizeWorkspaceRoot(process.cwd());
    } catch (_) {
        return '/';
    }
}

const _workspaceRoot = _detectWorkspaceRoot();

function _isWithinWorkspace(targetPath) {
    if (_workspaceRoot === '/') {
        return true;
    }
    return targetPath === _workspaceRoot || targetPath.indexOf(_workspaceRoot + '/') === 0;
}

function _resolveWorkspacePath(inputPath) {
    if (inputPath === undefined || inputPath === null) {
        throw new Error('path is required');
    }
    const raw = String(inputPath).trim();
    if (!raw) {
        throw new Error('path is required');
    }
    const resolved = path.resolve(process.cwd(), raw);
    if (!_isWithinWorkspace(resolved)) {
        throw new Error('workspace boundary violation: ' + resolved);
    }
    return resolved;
}

function _mkdirParentIfNeeded(targetPath, mkdirp) {
    if (!mkdirp) {
        return;
    }
    const parent = path.dirname(targetPath);
    if (!parent || parent === '.' || parent === '/') {
        return;
    }
    fs.mkdirSync(parent, { recursive: true });
}

function _countLines(text) {
    const src = String(text);
    if (src.length === 0) {
        return 0;
    }
    return src.split(/\r?\n/).length;
}

function _toInteger(value, defaultValue) {
    if (value === undefined || value === null || value === '') {
        return defaultValue;
    }
    const num = Number(value);
    if (!Number.isFinite(num)) {
        return defaultValue;
    }
    return Math.floor(num);
}

function _resolveExecPolicy(opts) {
    const requestedTimeoutMs = _toInteger(opts && opts.timeoutMs, 0);
    const requestedMaxOutputBytes = _toInteger(opts && opts.maxOutputBytes, 0);
    const effectiveTimeoutMs = requestedTimeoutMs > 0 ? requestedTimeoutMs : 0;
    let effectiveMaxOutputBytes = _maxOutputBytes;
    if (requestedMaxOutputBytes > 0) {
        effectiveMaxOutputBytes = Math.min(requestedMaxOutputBytes, _maxOutputBytes);
    }
    return {
        requestedTimeoutMs: requestedTimeoutMs,
        requestedMaxOutputBytes: requestedMaxOutputBytes,
        timeoutMs: effectiveTimeoutMs,
        maxOutputBytes: effectiveMaxOutputBytes,
    };
}

function _resolveRetryCount(opts) {
    const retryCount = _toInteger(opts && opts.retryCount, 0);
    return retryCount > 0 ? retryCount : 0;
}

function _buildEditStats(opType, lang) {
    const byLang = {};
    if (lang) {
        byLang[String(lang)] = 1;
    }
    return {
        totalOps: 1,
        runOps: opType === 'run' ? 1 : 0,
        createOps: (opType === 'create' || opType === 'write') ? 1 : 0,
        patchOps: opType === 'patch' ? 1 : 0,
        byLang: byLang,
    };
}

function _writeTextAtomic(targetPath, content, encoding) {
    const tempPath = targetPath + '.agenttmp.' + String(Date.now()) + '.' + String(Math.floor(Math.random() * 100000));
    try {
        fs.writeFileSync(tempPath, content, encoding);
        fs.renameSync(tempPath, targetPath);
    } catch (err) {
        try {
            if (fs.existsSync(tempPath)) {
                fs.rmSync(tempPath, { force: true });
            }
        } catch (_) {
        }
        throw err;
    }
}

function _normalizeLineRangePatchSpec(spec) {
    const startLine = _toInteger(spec.startLine, NaN);
    const endLine = _toInteger(spec.endLine, NaN);
    if (!Number.isFinite(startLine) || startLine < 1) {
        throw new Error('lineRangePatch.startLine must be >= 1');
    }
    if (!Number.isFinite(endLine) || endLine < startLine) {
        throw new Error('lineRangePatch.endLine must be >= startLine');
    }
    const replacement = String(spec.replacement === undefined || spec.replacement === null ? '' : spec.replacement);
    const normalized = {
        kind: 'lineRangePatch',
        startLine: startLine,
        endLine: endLine,
        replacement: replacement,
    };
    // anchorFallback: optional. If provided, used when lineRangePatch fails (e.g. out-of-range).
    if (_isPlainObject(spec.anchorFallback)) {
        const fb = spec.anchorFallback;
        normalized.anchorFallback = _normalizeAnchorPatchSpec({
            before: fb.before,
            after: fb.after,
            // fallback replacement defaults to the parent replacement if not specified
            replacement: (fb.replacement !== undefined && fb.replacement !== null) ? fb.replacement : replacement,
        });
    }
    return normalized;
}

function _normalizeAnchorPatchSpec(spec) {
    const before = String(spec.before === undefined || spec.before === null ? '' : spec.before);
    if (!before) {
        throw new Error('anchorPatch.before is required');
    }
    return {
        kind: 'anchorPatch',
        before: before,
        after: (spec.after === undefined || spec.after === null) ? null : String(spec.after),
        replacement: String(spec.replacement === undefined || spec.replacement === null ? '' : spec.replacement),
    };
}

function _normalizePatchSpec(spec) {
    if (!_isPlainObject(spec)) {
        throw new Error('patchSpec must be an object');
    }
    const kind = (spec.kind === undefined || spec.kind === null) ? '' : String(spec.kind);
    if (kind === 'lineRangePatch' || (!kind && spec.startLine !== undefined && spec.endLine !== undefined)) {
        return _normalizeLineRangePatchSpec(spec);
    }
    if (kind === 'anchorPatch' || (!kind && spec.before !== undefined)) {
        return _normalizeAnchorPatchSpec(spec);
    }
    throw new Error('patchSpec.kind must be lineRangePatch or anchorPatch');
}

function _applyLineRangePatch(content, spec) {
    const lines = String(content).split('\n');
    if (spec.endLine > lines.length) {
        throw new Error('lineRangePatch is out of file range: endLine=' + spec.endLine + ', lineCount=' + lines.length);
    }
    const replacementLines = spec.replacement.split('\n');
    const startIndex = spec.startLine - 1;
    const nextIndex = spec.endLine;
    const patched = lines.slice(0, startIndex).concat(replacementLines, lines.slice(nextIndex));
    return {
        kind: spec.kind,
        content: patched.join('\n'),
        startLine: spec.startLine,
        endLine: spec.endLine,
    };
}

function _applyAnchorPatch(content, spec) {
    const source = String(content);
    const beforeIndex = source.indexOf(spec.before);
    if (beforeIndex < 0) {
        throw new Error('anchorPatch.before not found');
    }
    const insertFrom = beforeIndex + spec.before.length;
    let insertTo = insertFrom;
    if (spec.after !== null && spec.after !== '') {
        const afterIndex = source.indexOf(spec.after, insertFrom);
        if (afterIndex < 0) {
            throw new Error('anchorPatch.after not found');
        }
        insertTo = afterIndex;
    }
    return {
        kind: spec.kind,
        content: source.slice(0, insertFrom) + spec.replacement + source.slice(insertTo),
        startLine: null,
        endLine: null,
    };
}

function _applyPatch(content, patchSpec) {
    const spec = _normalizePatchSpec(patchSpec);
    if (spec.kind === 'lineRangePatch') {
        // If anchorFallback is provided, check for out-of-range before attempting.
        if (spec.anchorFallback) {
            const v = _validatePatch(content, spec);
            if (v.usedFallback) {
                // lineRangePatch was out-of-range; anchorFallback validation was attempted.
                if (v.ok) {
                    const fallback = _applyAnchorPatch(content, spec.anchorFallback);
                    return {
                        kind: fallback.kind,
                        content: fallback.content,
                        startLine: fallback.startLine,
                        endLine: fallback.endLine,
                        usedFallback: true,
                        fallbackKind: 'anchorPatch',
                        fallbackReason: v.fallbackReason,
                    };
                }
                // anchorFallback also failed — throw its reason.
                throw new Error(v.detail || v.fallbackReason);
            }
        }
        return _applyLineRangePatch(content, spec);
    }
    return _applyAnchorPatch(content, spec);
}

// _countStringOccurrences counts non-overlapping occurrences of needle in source.
function _countStringOccurrences(source, needle) {
    if (!needle || needle.length === 0) { return 0; }
    let count = 0;
    let pos = 0;
    while ((pos = source.indexOf(needle, pos)) >= 0) {
        count++;
        pos += needle.length;
    }
    return count;
}

// _validatePatch checks whether a patch can be applied without modifying the file.
// Returns { ok: true, kind, startLine, endLine } on success, or
// { ok: false, reason: 'out-of-range'|'anchor-not-found'|'ambiguous', detail: string } on failure.
// For lineRangePatch with anchorFallback: if lineRangePatch fails with out-of-range,
// tries validating the anchorFallback and returns { ok, usedFallback: true, ... } accordingly.
function _validatePatch(content, patchSpec) {
    const spec = _normalizePatchSpec(patchSpec);
    if (spec.kind === 'lineRangePatch') {
        const lines = String(content).split('\n');
        if (spec.endLine > lines.length) {
            // Try anchorFallback if available
            if (spec.anchorFallback) {
                const fbResult = _validateAnchorPatch(content, spec.anchorFallback);
                const fallbackReason = 'lineRangePatch is out of file range: endLine=' + spec.endLine + ', lineCount=' + lines.length;
                if (fbResult.ok) {
                    return {
                        ok: true,
                        kind: fbResult.kind,
                        startLine: fbResult.startLine,
                        endLine: fbResult.endLine,
                        usedFallback: true,
                        fallbackKind: 'anchorPatch',
                        fallbackReason: fallbackReason,
                    };
                }
                return {
                    ok: false,
                    reason: fbResult.reason,
                    detail: fbResult.detail,
                    usedFallback: true,
                    fallbackKind: 'anchorPatch',
                    fallbackReason: fallbackReason,
                };
            }
            return {
                ok: false,
                reason: 'out-of-range',
                detail: 'lineRangePatch is out of file range: endLine=' + spec.endLine + ', lineCount=' + lines.length,
            };
        }
        return { ok: true, kind: spec.kind, startLine: spec.startLine, endLine: spec.endLine };
    }
    // anchorPatch
    return _validateAnchorPatch(content, spec);
}

// _validateAnchorPatch validates an already-normalized anchorPatch spec.
function _validateAnchorPatch(content, spec) {
    const source = String(content);
    const beforeCount = _countStringOccurrences(source, spec.before);
    if (beforeCount === 0) {
        return { ok: false, reason: 'anchor-not-found', detail: 'anchorPatch.before not found' };
    }
    if (beforeCount > 1) {
        return { ok: false, reason: 'ambiguous', detail: 'anchorPatch.before matches ' + beforeCount + ' locations' };
    }
    if (spec.after !== null && spec.after !== '') {
        const beforeIndex = source.indexOf(spec.before);
        const insertFrom = beforeIndex + spec.before.length;
        const afterIndex = source.indexOf(spec.after, insertFrom);
        if (afterIndex < 0) {
            return { ok: false, reason: 'anchor-not-found', detail: 'anchorPatch.after not found' };
        }
    }
    return { ok: true, kind: spec.kind, startLine: null, endLine: null };
}

function _parseCommand(command) {
    if (Array.isArray(command)) {
        if (command.length === 0) {
            throw new Error('command is required');
        }
        const cmd = String(command[0] || '').trim();
        if (!cmd) {
            throw new Error('command is required');
        }
        return {
            command: cmd,
            args: command.slice(1).map(function (one) { return String(one); }),
            commandLine: [cmd].concat(command.slice(1).map(function (one) { return String(one); })).join(' '),
        };
    }
    const line = String(command === undefined || command === null ? '' : command).trim();
    if (!line) {
        throw new Error('command is required');
    }
    const fields = splitFields(line);
    if (!fields || fields.length === 0) {
        throw new Error('command is required');
    }
    return {
        command: String(fields[0]),
        args: fields.slice(1).map(function (one) { return String(one); }),
        commandLine: line,
    };
}

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

function _normalizeStringList(values) {
    if (!Array.isArray(values)) {
        return [];
    }
    const seen = {};
    const out = [];
    for (let i = 0; i < values.length; i++) {
        const value = String(values[i] || '').trim().toLowerCase();
        if (!value || seen[value]) {
            continue;
        }
        seen[value] = true;
        out.push(value);
    }
    return out;
}

function _clientRenderTargets() {
    if (!_clientContext || !_isPlainObject(_clientContext)) {
        return [];
    }
    return _normalizeStringList(_clientContext.renderTargets);
}

function _clientPreferredVizFormats() {
    if (!_clientContext || !_isPlainObject(_clientContext)) {
        return [];
    }
    const allowed = {
        echarts: true,
        svg: true,
        png: true,
    };
    return _normalizeStringList(_clientContext.preferredVizFormats).filter(function (value) {
        return allowed[value] === true;
    });
}

function _shouldReturnRawVizspec(options) {
    const target = _toStringOption(options, 'target', 'auto').toLowerCase();
    if (target === 'tui') {
        return false;
    }
    if (target === 'vizspec' || target === 'raw') {
        return true;
    }
    return _clientRenderTargets().indexOf('vizspec/v1') >= 0;
}

function _buildRawVizspec(spec, options, meta) {
    const normalized = vizspec.normalize(spec);
    const preferred = _clientPreferredVizFormats();
    const mergedMeta = {};
    if (_isPlainObject(normalized.meta)) {
        Object.assign(mergedMeta, normalized.meta);
    }
    if (_isPlainObject(meta)) {
        Object.assign(mergedMeta, meta);
    }
    if (preferred.length > 0) {
        mergedMeta.preferred = preferred;
    }
    const out = Object.assign({
        schema: 'advn/v1',
    }, normalized);
    if (Object.keys(mergedMeta).length > 0) {
        out.meta = mergedMeta;
    }
    return out;
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
            if (_shouldReturnRawVizspec(opts)) {
                return _buildRawVizspec(spec, opts, {
                    title: _toStringOption(opts, 'title', ''),
                    mode: 'blocks',
                });
            }
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
            if (_shouldReturnRawVizspec(opts)) {
                return _buildRawVizspec(spec, opts, {
                    title: _toStringOption(opts, 'title', ''),
                    mode: 'lines',
                });
            }
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
            'agent.viz.render(spec[, options])           Render VIZSPEC for the current client context',
            'agent.viz.blocks(spec[, options])           Render VIZSPEC as TUI blocks or raw vizspec',
            'agent.viz.lines(spec[, options])            Render VIZSPEC as TUI lines or raw vizspec',
            'agent.viz.fromRows(rows, options)           High-level: build spec from row array and render',
            '  required: options.x (x-axis field name)',
            '  optional: options.y (string or string[] of y field names — auto-detected if omitted)',
            '  optional: options.mode ("blocks"|"lines", default "lines")',
            '  optional: options.target ("auto"|"tui"|"vizspec", default "auto")',
            'options: mode, compact, rows, width, height, series, timeformat, tz, title',
            'Returns: a TUI envelope or { schema:"advn/v1", ... } depending on clientContext/target',
        ].join('\n');
    }
}

const _db = new AgentDbHelper();
const _schema = new AgentSchemaHelper(_db);
const _modules = new AgentModuleDocsHelper();
const _sqlref = new AgentSQLReferenceDocsHelper();
const _viz = new AgentVizHelper();

// AgentFsHelper provides file operations with policy enforcement.
class AgentFsHelper {
    // write(path, content, opts?) — write file (create or overwrite).
    // Denied when read-only mode is active.
    write(path, content, opts) {
        if (_readOnly) {
            throw new Error('capability denied: fs.write is not allowed in read-only mode');
        }
        const options = _isPlainObject(opts) ? opts : {};
        const targetPath = _resolveWorkspacePath(path);
        const mkdirp = options.mkdirp !== false;
        const atomic = options.atomic !== false;
        const encoding = options.encoding === undefined ? 'utf8' : String(options.encoding);
        const body = String(content === undefined || content === null ? '' : content);
        const existed = fs.existsSync(targetPath);
        const opType = existed ? 'write' : 'create';
        const retryCount = _resolveRetryCount(options);

        _mkdirParentIfNeeded(targetPath, mkdirp);

        if (atomic) {
            _writeTextAtomic(targetPath, body, encoding);
        } else {
            fs.writeFileSync(targetPath, body, encoding);
        }

        return {
            path: targetPath,
            bytes: Buffer.from(body, 'utf8').length,
            lines: _countLines(body),
            opType: opType,
            retryCount: retryCount,
            editStats: _buildEditStats(opType),
        };
    }

    // read(path, opts?) — read file (with optional startLine, endLine for ranges).
    // readOnly mode: allowed, but may be limited by workspace boundary.
    read(path, opts) {
        const options = _isPlainObject(opts) ? opts : {};
        const targetPath = _resolveWorkspacePath(path);
        const encoding = options.encoding === undefined ? 'utf8' : String(options.encoding);
        const raw = fs.readFileSync(targetPath, encoding);
        const text = (typeof raw === 'string') ? raw : String(raw);
        const hasRange = options.startLine !== undefined || options.endLine !== undefined;

        if (!hasRange) {
            return {
                path: targetPath,
                content: text,
                lineCount: _countLines(text),
            };
        }

        const lines = text.split(/\r?\n/);
        const startLine = Math.max(1, _toInteger(options.startLine, 1));
        const endLine = Math.min(lines.length, _toInteger(options.endLine, lines.length));
        if (endLine < startLine) {
            throw new Error('invalid read range: endLine must be >= startLine');
        }
        const chunk = lines.slice(startLine - 1, endLine).join('\n');
        return {
            path: targetPath,
            startLine: startLine,
            endLine: endLine,
            content: chunk,
            lineCount: lines.length,
        };
    }

    // patch(path, patchSpec, opts?) — apply line range or anchor-based patch.
    // Denied when read-only mode is active (unless opts.dryRun is true).
    // opts.dryRun: if true, validate the patch without modifying the file.
    //   Returns { path, dryRun:true, ok, kind, startLine, endLine, reason?, detail? }
    patch(path, patchSpec, opts) {
        const options = _isPlainObject(opts) ? opts : {};
        const dryRun = options.dryRun === true;
        if (!dryRun && _readOnly) {
            throw new Error('capability denied: fs.patch is not allowed in read-only mode');
        }
        const targetPath = _resolveWorkspacePath(path);
        const original = fs.readFileSync(targetPath, 'utf8');

        if (dryRun) {
            const v = _validatePatch(original, patchSpec);
            const result = {
                path: targetPath,
                dryRun: true,
                ok: v.ok,
                kind: v.kind !== undefined ? v.kind : null,
                startLine: v.startLine !== undefined ? v.startLine : null,
                endLine: v.endLine !== undefined ? v.endLine : null,
            };
            if (v.usedFallback) {
                result.usedFallback = true;
                result.fallbackKind = v.fallbackKind !== undefined ? v.fallbackKind : null;
                result.fallbackReason = v.fallbackReason !== undefined ? v.fallbackReason : null;
            }
            if (!v.ok) {
                result.reason = v.reason;
                result.detail = v.detail;
            }
            return result;
        }

        const applied = _applyPatch(original, patchSpec);
        const retryCount = _resolveRetryCount(_isPlainObject(patchSpec) ? patchSpec : null);

        if (applied.content !== original) {
            _writeTextAtomic(targetPath, applied.content, 'utf8');
        }

        const patchResult = {
            path: targetPath,
            applied: applied.content !== original,
            kind: applied.kind,
            startLine: applied.startLine,
            endLine: applied.endLine,
            opType: 'patch',
            retryCount: retryCount,
            editStats: _buildEditStats('patch'),
        };
        if (applied.usedFallback) {
            patchResult.usedFallback = true;
            patchResult.fallbackKind = applied.fallbackKind !== undefined ? applied.fallbackKind : null;
            patchResult.fallbackReason = applied.fallbackReason !== undefined ? applied.fallbackReason : null;
        }
        return patchResult;
    }
}

// AgentDiagnosticsHelper provides structured error diagnostics from raw output text.
class AgentDiagnosticsHelper {
    // fromOutput(text, opts?) — parse stderr/stdout text into structured diagnostics.
    // Each diagnostic: { message, path, line, col, context? }
    // opts.contextLines: number of surrounding lines to include (default 2, max 10).
    fromOutput(text, opts) {
        const options = (_isPlainObject(opts) ? opts : {});
        let contextLines = Math.floor(Number(options.contextLines !== undefined ? options.contextLines : 2));
        if (!isFinite(contextLines) || contextLines < 0) { contextLines = 2; }
        if (contextLines > 10) { contextLines = 10; }

        const src = String(text === undefined || text === null ? '' : text);
        const errorLines = src.split(/\r?\n/).filter(function (l) { return l.trim().length > 0; });
        const diagnostics = [];
        const seen = {};

        for (let i = 0; i < errorLines.length; i++) {
            const msg = errorLines[i].trimRight();
            const loc = _extractErrorLocation(msg);
            const key = loc ? (loc.path + ':' + loc.line) : msg;
            if (seen[key]) { continue; }
            seen[key] = true;

            const diag = { message: msg, path: null, line: null, col: null, context: null };
            if (loc) {
                diag.path = loc.path;
                diag.line = loc.line;
                diag.col = loc.col;
                if (contextLines > 0) {
                    diag.context = _readDiagnosticContext(loc.path, loc.line, contextLines);
                }
            }
            diagnostics.push(diag);
        }
        return diagnostics;
    }

    // suggest(diagnostics, opts?) — build a patch suggestion prompt text from diagnostics.
    // Returns a string describing patch candidates for the AI.
    // opts.maxCount: max number of diagnostics to include (default 2).
    suggest(diagnostics, opts) {
        const options = (_isPlainObject(opts) ? opts : {});
        let maxCount = Math.floor(Number(options.maxCount !== undefined ? options.maxCount : 2));
        if (!isFinite(maxCount) || maxCount < 1) { maxCount = 2; }

        const diags = Array.isArray(diagnostics) ? diagnostics : [];
        if (diags.length === 0) { return ''; }

        const out = [
            'Patch suggestions from diagnostics:',
            'Apply one minimal patch candidate first, then rerun the command.',
            'Prefer agent.fs.patch with kind="lineRangePatch". Use anchorPatch only if line numbers are unreliable.',
        ];
        const n = Math.min(diags.length, maxCount);
        for (let i = 0; i < n; i++) {
            const d = diags[i] || {};
            const filePath = String(d.path || '').trim();
            let line = Math.floor(Number(d.line || 1));
            if (!isFinite(line) || line < 1) { line = 1; }
            let col = Math.floor(Number(d.col || 1));
            if (!isFinite(col) || col < 1) { col = 1; }

            out.push('Candidate ' + (i + 1) + ':');
            if (filePath) {
                out.push('- target: ' + filePath + ':' + line + ':' + col);
            }
            if (d.message) {
                out.push('- error: ' + String(d.message));
            }
            if (d.context && d.context.snippet) {
                out.push('```');
                out.push(String(d.context.snippet));
                out.push('```');
            }
            out.push('```json');
            out.push('{');
            out.push('  "path": "' + filePath.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '",');
            out.push('  "patch": {');
            out.push('    "kind": "lineRangePatch",');
            out.push('    "startLine": ' + line + ',');
            out.push('    "endLine": ' + line + ',');
            out.push('    "replacement": "// TODO: minimal fix for the failing line"');
            out.push('  }');
            out.push('}');
            out.push('```');
        }
        out.push('Return only the minimal patch code/action, not full-file regeneration.');
        return out.join('\n');
    }
}

function _coerceRowsInput(input) {
    if (Array.isArray(input)) {
        return input;
    }
    if (_isPlainObject(input) && Array.isArray(input.rows)) {
        return input.rows;
    }
    throw new Error('agent.analysis: input must be an array of rows or { rows:[...] }');
}

function _resolveAnalysisYField(rows, opts) {
    const options = _isPlainObject(opts) ? opts : {};
    const explicit = options.y !== undefined && options.y !== null ? String(options.y).trim() : '';
    if (explicit) {
        return explicit;
    }
    const sample = rows[0] || {};
    const keys = Object.keys(sample);
    for (let i = 0; i < keys.length; i++) {
        const key = keys[i];
        if (typeof sample[key] === 'number') {
            return key;
        }
    }
    throw new Error('agent.analysis.timeseries.summary: no numeric y field found; specify options.y');
}

function _resolveAnalysisXField(rows, opts) {
    const options = _isPlainObject(opts) ? opts : {};
    const explicit = options.x !== undefined && options.x !== null ? String(options.x).trim() : '';
    if (explicit) {
        return explicit;
    }
    const sample = rows[0] || {};
    const keys = Object.keys(sample);
    const preferred = ['TIME', 'time', 'ts', 'timestamp', 'TIMESTAMP'];
    for (let i = 0; i < preferred.length; i++) {
        if (sample[preferred[i]] !== undefined) {
            return preferred[i];
        }
    }
    for (let j = 0; j < keys.length; j++) {
        if (sample[keys[j]] instanceof Date) {
            return keys[j];
        }
    }
    return '';
}

function _coerceTimeValue(value) {
    if (value instanceof Date) {
        return value.getTime();
    }
    if (typeof value === 'number' && Number.isFinite(value)) {
        return value;
    }
    if (typeof value === 'string' && value.trim() !== '') {
        const n = Date.parse(value);
        if (Number.isFinite(n)) {
            return n;
        }
    }
    return NaN;
}

class AgentAnalysisHelper {
    timeseriesSummary(input, opts) {
        const rows = _coerceRowsInput(input);
        if (rows.length === 0) {
            throw new Error('agent.analysis.timeseries.summary: rows must be non-empty');
        }
        const yField = _resolveAnalysisYField(rows, opts);
        const xField = _resolveAnalysisXField(rows, opts);
        const values = [];
        const timeValues = [];
        let min = Infinity;
        let max = -Infinity;
        let sum = 0;
        let sumSq = 0;
        for (let i = 0; i < rows.length; i++) {
            const row = rows[i] || {};
            const value = Number(row[yField]);
            if (!Number.isFinite(value)) {
                continue;
            }
            values.push(value);
            sum += value;
            sumSq += value * value;
            if (value < min) { min = value; }
            if (value > max) { max = value; }
            if (xField) {
                const tv = _coerceTimeValue(row[xField]);
                if (Number.isFinite(tv)) {
                    timeValues.push(tv);
                }
            }
        }
        if (values.length === 0) {
            throw new Error('agent.analysis.timeseries.summary: no numeric samples found for the selected y field');
        }
        const count = values.length;
        const avg = sum / count;
        let varianceSum = 0;
        let absPeak = 0;
        for (let j = 0; j < values.length; j++) {
            const v = values[j];
            varianceSum += Math.pow(v - avg, 2);
            if (Math.abs(v) > absPeak) {
                absPeak = Math.abs(v);
            }
        }
        const stddev = Math.sqrt(varianceSum / count);
        const rms = Math.sqrt(sumSq / count);
        const peakToPeak = max - min;
        const crestFactor = rms > 0 ? absPeak / rms : 0;
        let sampleInterval = null;
        let timeRange = null;
        if (timeValues.length >= 2) {
            timeValues.sort(function(a, b) { return a - b; });
            let diffSum = 0;
            let diffCount = 0;
            for (let k = 1; k < timeValues.length; k++) {
                const diff = timeValues[k] - timeValues[k - 1];
                if (Number.isFinite(diff) && diff >= 0) {
                    diffSum += diff;
                    diffCount += 1;
                }
            }
            sampleInterval = diffCount > 0 ? diffSum / diffCount : null;
            timeRange = {
                start: new Date(timeValues[0]).toISOString(),
                end: new Date(timeValues[timeValues.length - 1]).toISOString(),
            };
        }
        return {
            count: count,
            xField: xField || null,
            yField: yField,
            min: min,
            max: max,
            avg: avg,
            stddev: stddev,
            rms: rms,
            peakToPeak: peakToPeak,
            crestFactor: crestFactor,
            sampleInterval: sampleInterval,
            timeRange: timeRange,
        };
    }

    reportGrounding(evidence, opts) {
        const options = _isPlainObject(opts) ? opts : {};
        let maxItems = Math.floor(Number(options.maxItems !== undefined ? options.maxItems : 5));
        if (!isFinite(maxItems) || maxItems < 1) { maxItems = 5; }
        const items = Array.isArray(evidence) ? evidence : [];
        const highlights = [];
        for (let i = 0; i < items.length && highlights.length < maxItems; i++) {
            const one = items[i] || {};
            if (one.kind === 'sql') {
                highlights.push({
                    kind: 'sql',
                    sql: one.sql || '',
                    rowCount: one.rowCount || 0,
                    columns: Array.isArray(one.columns) ? one.columns : [],
                });
                continue;
            }
            if (one.kind === 'viz') {
                highlights.push({
                    kind: 'viz',
                    renderer: one.renderer || '',
                    mode: one.mode || '',
                });
                continue;
            }
            if (one.kind === 'value') {
                highlights.push({
                    kind: 'value',
                    valueType: one.valueType || '',
                });
            }
        }
        return {
            count: highlights.length,
            highlights: highlights,
            instruction: 'Use these evidence highlights directly in the final report.',
        };
    }
}

// _extractErrorLocation — parse a single error message line into { path, line, col }.
// Mirrors the logic in ai_executor.js extractErrorLocation for in-profile use.
function _extractErrorLocation(errMsg) {
    if (!errMsg || typeof errMsg !== 'string') { return null; }
    // Go: "file.go:123:1: message"
    const goMatch = /^([^\s:]+\.go):([0-9]+):([0-9]+):/m.exec(errMsg);
    if (goMatch) {
        return { path: goMatch[1], line: parseInt(goMatch[2], 10), col: parseInt(goMatch[3], 10) };
    }
    // JS runtime: "Error at line 123, col 45"
    const jsMatch = /line\s+([0-9]+).*col\s+([0-9]+)/i.exec(errMsg);
    if (jsMatch) {
        return { path: 'script', line: parseInt(jsMatch[1], 10), col: parseInt(jsMatch[2], 10) };
    }
    // goja/jsh stack: "at func (file.js:15:8)"
    const gojaMatch = /\(([^)]+\.js):([0-9]+):([0-9]+)\)/.exec(errMsg);
    if (gojaMatch) {
        return { path: gojaMatch[1], line: parseInt(gojaMatch[2], 10), col: parseInt(gojaMatch[3], 10) };
    }
    // Unix/shell: "file.js:15: error message"
    const unixMatch = /^([^\s:]+):([0-9]+):/m.exec(errMsg);
    if (unixMatch) {
        return { path: unixMatch[1], line: parseInt(unixMatch[2], 10), col: 1 };
    }
    return null;
}

// _readDiagnosticContext — read surrounding lines of a file around a given line number.
function _readDiagnosticContext(filePath, line, contextLines) {
    if (!filePath || filePath === 'script') { return null; }
    const target = String(filePath).charAt(0) === '/' ? String(filePath) : path.resolve(process.cwd(), String(filePath));
    try {
        const text = fs.readFileSync(target, 'utf8');
        const lines = String(text || '').split(/\r?\n/);
        const center = (Number.isFinite(Number(line)) && Number(line) >= 1) ? Number(line) : 1;
        const around = (Number.isFinite(Number(contextLines)) && Number(contextLines) >= 0) ? Math.min(10, Number(contextLines)) : 2;
        const startLine = Math.max(1, center - around);
        const endLine = Math.min(lines.length, center + around);
        return {
            path: target,
            startLine: startLine,
            endLine: endLine,
            snippet: lines.slice(startLine - 1, endLine).join('\n'),
        };
    } catch (_) {
        return null;
    }
}

// AgentExecHelper provides execution run with policy enforcement.
class AgentExecHelper {
    // run(command, opts?) — execute command (shell or other).
    // readOnly mode: restricted to allowlist commands.
    run(command, opts) {
        const options = _isPlainObject(opts) ? opts : {};
        const parsed = _parseCommand(command);
        const policy = _resolveExecPolicy(options);
        const retryCount = _resolveRetryCount(options);
        if (_readOnly && !READ_ONLY_EXEC_ALLOW[parsed.command]) {
            throw new Error('capability denied: exec.run command is not allowed in read-only mode: ' + parsed.command);
        }

        const previousCwd = process.cwd();
        let changedCwd = false;
        let targetCwd = previousCwd;
        if (options.cwd !== undefined && options.cwd !== null && String(options.cwd).trim() !== '') {
            targetCwd = _resolveWorkspacePath(options.cwd);
            if (targetCwd !== previousCwd) {
                process.chdir(targetCwd);
                changedCwd = true;
            }
        }

        try {
            const exitCode = process.exec(parsed.command, ...parsed.args);
            return {
                command: parsed.command,
                args: parsed.args,
                commandLine: parsed.commandLine,
                cwd: targetCwd,
                exitCode: exitCode,
                opType: 'run',
                retryCount: retryCount,
                editStats: _buildEditStats('run', 'jsh-shell'),
                limits: {
                    timeoutMs: policy.timeoutMs,
                    maxOutputBytes: policy.maxOutputBytes,
                },
            };
        } finally {
            if (changedCwd) {
                process.chdir(previousCwd);
            }
        }
    }
}

// > **IMPORTANT**: Schema objects are **UPPERCASE** (`NAME`, `TYPE`, `FLAG`, ...).
// > Query result field names follow SQL projection rules:
// > - Explicit names/aliases are preserved as written (for example, `SELECT name, time AS MyTime ...` returns `name` and `MyTime`).
// > - Implicit names (for example, `SELECT * FROM table`) are returned in **UPPERCASE**.
// > Prefer uppercase access for system/schema fields (for example, `t.NAME`, `t.TYPE`, `row.COLUMN_NAME`).


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
        'NOTE: Schema objects are **UPPERCASE** (`NAME`, `TYPE`, `FLAG`, ...).',
        '  Query result field names follow SQL projection rules:',
        '    - Explicit names/aliases are preserved as written (for example, `SELECT name, time AS MyTime ...` returns `name` and `MyTime`).',
        '    - Implicit names (for example, `SELECT * FROM table`) are returned in **UPPERCASE**.',
        '    - Prefer uppercase access for system/schema fields (for example, t.NAME, t.TYPE, row.COLUMN_NAME, etc.).',
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
        '  agent.fs.write(path, content, opts?) Write/create file (denied in read-only mode)',
        '  agent.fs.read(path, opts?)         Read file (with optional startLine, endLine)',
        '  agent.fs.patch(path, patchSpec, opts?) Apply patch to file (denied in read-only mode)',
        '    opts.dryRun: true → validate without modifying; returns {ok, reason?, detail?}',
        '    patchSpec.anchorFallback: {before, after?, replacement?} — fallback anchor patch',
        '      when lineRangePatch fails due to out-of-range (line shift); result has usedFallback:true',
        '',
        '  agent.exec.run(command, opts?)     Execute command/shell (restricted in read-only mode)',
        '',
        '  agent.diagnostics.fromOutput(text, opts?) Parse stderr/stdout text → [{message,path,line,col,context?}]',
        '    opts.contextLines: surrounding file lines to include (default 2, max 10)',
        '  agent.diagnostics.suggest(diags, opts?)   Build patch suggestion prompt from diagnostics array',
        '    opts.maxCount: max candidates (default 2)',
        '',
        '  agent.analysis.timeseries.summary(input, opts?) Summarize numeric time-series rows',
        '    opts.x: optional x/time field, opts.y: numeric value field',
        '  agent.analysis.report.grounding(evidence, opts?) Build report grounding highlights',
        '',
        '  agent.viz.render(spec[, options])        Render VIZSPEC for the current client context',
        '  agent.viz.blocks(spec[, options])        Render VIZSPEC as TUI blocks or raw vizspec',
        '  agent.viz.lines(spec[, options])         Render VIZSPEC as TUI lines or raw vizspec',
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
const _fs = new AgentFsHelper();
const _exec = new AgentExecHelper();
const _diagnostics = new AgentDiagnosticsHelper();
const _analysisHelper = new AgentAnalysisHelper();

const agent = {
    db: _db,
    schema: _schema,
    modules: _modules,
    sqlref: _sqlref,
    viz: _viz,
    fs: _fs,
    exec: _exec,
    diagnostics: _diagnostics,
    analysis: {
        timeseries: {
            summary: function (input, opts) {
                return _analysisHelper.timeseriesSummary(input, opts);
            },
        },
        report: {
            grounding: function (evidence, opts) {
                return _analysisHelper.reportGrounding(evidence, opts);
            },
        },
    },

    runtime: {
        clientContext: _clientContext,
        // capabilities() — list allowed operation categories for this profile.
        capabilities: function () {
            const caps = ['db.read', 'db.schema', 'fs.read', 'exec.run', 'diagnostics', 'analysis'];
            if (!_readOnly) { caps.push('db.write'); }
            if (!_readOnly) {
                caps.push('fs.write');
                caps.push('fs.patch');
            } else {
                // dryRun is allowed even in read-only mode
                caps.push('fs.patch.dryRun');
            }
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
