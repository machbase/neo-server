'use strict';

// ai/executor.js — executable jsh-run block extractor and agent REPL runner.
//
// Extracts ```jsh-run ... ``` code blocks from LLM responses and executes them
// via ai.exec(), which runs the code in the agent REPL profile and returns
// structured JSON results from AgentRenderer.
//
// Usage (from ai.js):
//   const { extractCodeBlocks, executeJsh } = require('ai/executor');
//
//   var blocks = extractCodeBlocks(llmResponseText);
//   blocks.forEach(function(block) {
//       var results = executeJsh(block.code, { readOnly: true, maxRows: 500 });
//       results.forEach(function(r) { console.println(JSON.stringify(r)); });
//   });

var { ai } = require('@jsh/shell');
var fs = require('fs');
var path = require('path');
var process = require('process');

var RUNNABLE_LANGS = {
    'jsh-run': true,
    'jsh-shell': true,
    'jsh-sql': true,
};

function buildShellExecCode(command) {
    var raw = JSON.stringify(String(command || ''));
    return [
        '(function () {',
        "    'use strict';",
        "    const process = require('process');",
        "    const splitFields = require('util/splitFields');",
        '    const cmdline = ' + raw + ';',
        '    const line = String(cmdline || \"\").trim();',
        "    if (!line) { throw new Error('jsh-shell: empty command'); }",
        '    const fields = splitFields(line);',
        "    if (!fields || fields.length === 0) { throw new Error('jsh-shell: invalid command'); }",
        '    const command = fields[0];',
        '    const args = fields.slice(1);',
        '    const readOnly = !!(agent && agent.runtime && agent.runtime.limits && agent.runtime.limits().readOnly);',
        "    const allow = { ls: true, cat: true, pwd: true, echo: true, wc: true, head: true, tail: true };",
        "    if (readOnly && !allow[command]) { throw new Error('jsh-shell: command denied in read-only mode: ' + command); }",
        '    const exitCode = process.exec(command, ...args);',
        '    return { command: line, args: args, exitCode: exitCode };',
        '}());',
    ].join('\n');
}

function buildSqlExecCode(sqlText) {
    var raw = JSON.stringify(String(sqlText || ''));
    return [
        '(function () {',
        "    'use strict';",
        "    const pretty = require('pretty');",
        '    const sql = ' + raw + ';',
        '    const text = String(sql || \"\").trim();',
        "    if (!text) { throw new Error('jsh-sql: empty SQL'); }",
        "    if (text.indexOf(';') >= 0) { throw new Error('jsh-sql: only single statement is allowed'); }",
        "    const lowered = text.toLowerCase();",
        "    const readOnly = !!(agent && agent.runtime && agent.runtime.limits && agent.runtime.limits().readOnly);",
        "    const allowed = /^(select|show|describe|desc|explain)\\b/.test(lowered);",
        "    if (readOnly && !allowed) { throw new Error('jsh-sql: write statements are denied in read-only mode'); }",
        '    const result = agent.db.query(text);',
        "    const box = pretty.Table({ rownum: false, footer: false, format: 'box' });",
        '    const rows = (result && Array.isArray(result.rows)) ? result.rows : [];',
        '    if (rows.length === 0) {',
        '        return {',
        "            __agentSql: true,",
        "            schema: 'agent-sql/v1',",
        '            sql: text,',
        '            columns: [],',
        '            rows: [],',
        '            rowCount: 0,',
        '            truncated: !!(result && result.truncated),',
        "            rendered: '(no rows)'",
        '        };',
        '    }',
        '    const columns = Object.keys(rows[0]);',
        '    box.appendHeader(columns);',
        '    for (let i = 0; i < rows.length; i++) {',
        '        const row = rows[i] || {};',
        '        const values = [];',
        '        for (let c = 0; c < columns.length; c++) {',
        '            values.push(row[columns[c]]);',
        '        }',
        '        box.append(values);',
        '    }',
        '    let rendered = box.render();',
        '    if (result && result.truncated) {',
        "        rendered += '\\n[truncated at ' + result.count + ' rows]';",
        '    }',
        '    return {',
        "        __agentSql: true,",
        "        schema: 'agent-sql/v1',",
        '        sql: text,',
        '        columns: columns,',
        '        rows: rows,',
        '        rowCount: rows.length,',
        '        truncated: !!(result && result.truncated),',
        '        rendered: rendered',
        '    };',
        '}());',
    ].join('\n');
}

function buildExecutableCode(block) {
    if (!block || !block.lang || !block.code) {
        throw new Error('invalid runnable block');
    }
    if (block.lang === 'jsh-run') {
        return block.code;
    }
    if (block.lang === 'jsh-shell') {
        return buildShellExecCode(block.code);
    }
    if (block.lang === 'jsh-sql') {
        return buildSqlExecCode(block.code);
    }
    throw new Error('unsupported runnable language: ' + block.lang);
}

/**
 * Extract fenced code blocks tagged as runnable fence types from an LLM response.
 * Plain fences and explanatory examples are ignored so only explicit
 * execution candidates are considered runnable.
 *
 * Returns an array of { lang, code } objects in document order.
 *
 * @param {string} text  LLM response text
 * @returns {{ lang: string, code: string }[]}
 */
function extractCodeBlocks(text) {
    var blocks = [];
    // Match ```<lang>\n<body>```. The closing ``` may be followed by nothing
    // or a newline.
    var re = /```([a-z0-9_-]+)\n([\s\S]*?)```/g;
    var m;
    while ((m = re.exec(text)) !== null) {
        var lang = String(m[1] || '').toLowerCase();
        if (!RUNNABLE_LANGS[lang]) {
            continue;
        }
        var code = m[2];
        if (code && code.trim()) {
            blocks.push({ lang: lang, code: code });
        }
    }
    return blocks;
}

function _extractAllFencedBlocks(text) {
    var blocks = [];
    var re = /```([a-z0-9_-]+)\n([\s\S]*?)```/g;
    var m;
    while ((m = re.exec(String(text || ''))) !== null) {
        blocks.push({
            lang: String(m[1] || '').toLowerCase(),
            code: m[2],
        });
    }
    return blocks;
}

function _isSingleSafeSelect(sqlText) {
    var text = String(sqlText || '').trim();
    if (!text) {
        return false;
    }
    if (text.indexOf(';') >= 0) {
        return false;
    }
    return /^(select|show|describe|desc|explain)\b/i.test(text);
}

function tryPromotePlainSqlBlock(block) {
    if (!block || !block.code) {
        return null;
    }
    var lang = String(block.lang || '').toLowerCase();
    if (lang !== 'sql') {
        return null;
    }
    if (!_isSingleSafeSelect(block.code)) {
        return null;
    }
    return {
        lang: 'jsh-sql',
        code: String(block.code || '').trim(),
        promoted: true,
        promotedFrom: lang,
    };
}

function tryPromotePlainJsBlock(block) {
    if (!block || !block.code) {
        return null;
    }
    var lang = String(block.lang || '').toLowerCase();
    if (lang !== 'js' && lang !== 'javascript') {
        return null;
    }
    var code = String(block.code || '').trim();
    if (!code) {
        return null;
    }
    if (code.split(/\r?\n/).length > 40) {
        return null;
    }
    if (code.indexOf('agent.') < 0) {
        return null;
    }
    if (/process\.exec|agent\.exec\.run|agent\.fs\.write|agent\.fs\.patch|agent\.db\.exec/.test(code)) {
        return null;
    }
    return {
        lang: 'jsh-run',
        code: code,
        promoted: true,
        promotedFrom: lang,
    };
}

function extractRunnableCandidates(text, options) {
    var opts = options || {};
    var blocks = extractCodeBlocks(text);
    if (blocks.length > 0 || opts.autoRepair !== true) {
        return blocks;
    }
    var allBlocks = _extractAllFencedBlocks(text);
    var promoted = [];
    for (var i = 0; i < allBlocks.length; i++) {
        var block = allBlocks[i];
        var candidate = tryPromotePlainSqlBlock(block);
        if (!candidate) {
            candidate = tryPromotePlainJsBlock(block);
        }
        if (candidate) {
            promoted.push(candidate);
        }
    }
    return promoted;
}

/**
 * Execute jsh code via the agent REPL profile.
 *
 * Delegates to ai.exec() (Go: ai.go jsExecJsh), which:
 *   1. Initialises globalThis.agent (agent.db, agent.schema, agent.runtime)
 *   2. Runs the code via evalWithTimeout with a fresh AgentRenderer
 *   3. Returns the NDJSON lines parsed into an array of result objects
 *
 * Each result object has the shape from AgentRenderer:
 *   { ok: true,  type: string, value: any, elapsedMs: number }
 *   { ok: false, error: string, elapsedMs: number }
 *   { ok: true,  ..., truncated: true }  — when output exceeds maxOutputBytes
 *
 * @param {string} code            jsh code to evaluate
 * @param {object} [options]
 * @param {boolean} [options.readOnly=true]        deny agent.db.exec write ops
 * @param {number}  [options.maxRows=1000]         max rows per query
 * @param {number}  [options.timeoutMs=30000]      execution timeout in ms
 * @param {number}  [options.maxOutputBytes=65536] max serialised output bytes
 * @returns {object[]} array of AgentRenderer result objects
 */
function executeJsh(code, options) {
    var opts = options || {};
    return ai.exec(code, {
        readOnly: opts.readOnly !== false,   // default true
        maxRows: opts.maxRows || 1000,
        timeoutMs: opts.timeoutMs || 30000,
        maxOutputBytes: opts.maxOutputBytes || 65536,
    });
}

function executeBlock(block, options) {
    var code = buildExecutableCode(block);
    return executeJsh(code, options);
}

function isRenderEnvelope(value) {
    return !!value &&
        typeof value === 'object' &&
        value.__agentRender === true &&
        value.schema === 'agent-render/v1' &&
        (value.renderer === 'viz.tui' || value.renderer === 'advn.tui') &&
        (value.mode === 'blocks' || value.mode === 'lines');
}

function isSqlEvidence(value) {
    return !!value &&
        typeof value === 'object' &&
        value.__agentSql === true &&
        value.schema === 'agent-sql/v1';
}

function collectRenderEnvelopes(results) {
    var envelopes = [];
    if (!results || results.length === 0) {
        return envelopes;
    }
    for (var i = 0; i < results.length; i++) {
        var r = results[i] || {};
        if (!r.ok || !isRenderEnvelope(r.value)) {
            continue;
        }
        envelopes.push(r.value);
    }
    return envelopes;
}

function collectExecutionEvidence(results, block) {
    var evidence = [];
    if (!results || results.length === 0) {
        return evidence;
    }
    var source = {
        lang: block && block.lang ? String(block.lang) : '',
        promoted: !!(block && block.promoted),
        promotedFrom: block && block.promotedFrom ? String(block.promotedFrom) : '',
    };
    for (var i = 0; i < results.length; i++) {
        var r = results[i] || {};
        if (r.ok !== true) {
            continue;
        }
        if (isSqlEvidence(r.value)) {
            evidence.push({
                kind: 'sql',
                source: source,
                sql: r.value.sql || '',
                columns: Array.isArray(r.value.columns) ? r.value.columns : [],
                rows: Array.isArray(r.value.rows) ? r.value.rows : [],
                rowCount: Number(r.value.rowCount || 0),
                truncated: !!r.value.truncated,
                rendered: r.value.rendered || '',
            });
            continue;
        }
        if (isRenderEnvelope(r.value)) {
            evidence.push({
                kind: 'viz',
                source: source,
                renderer: String(r.value.renderer || ''),
                mode: String(r.value.mode || ''),
                meta: r.value.meta || {},
            });
            continue;
        }
        if (r.type === 'print') {
            evidence.push({
                kind: 'print',
                source: source,
                text: String(r.value || ''),
            });
            continue;
        }
        if (r.value !== undefined && r.value !== null && r.type !== 'undefined') {
            evidence.push({
                kind: 'value',
                source: source,
                type: String(r.type || ''),
                value: r.value,
            });
        }
    }
    return evidence;
}

function formatEvidencePrompt(evidence, options) {
    var opts = options || {};
    var maxItems = Number(opts.maxItems || 3);
    if (!Number.isFinite(maxItems) || maxItems < 1) {
        maxItems = 3;
    }
    var items = Array.isArray(evidence) ? evidence.slice(0, maxItems) : [];
    if (items.length === 0) {
        return '';
    }
    return [
        'Structured execution evidence:',
        'Use this evidence directly when writing the next analysis or report.',
        '```json',
        JSON.stringify(items, null, 2),
        '```',
    ].join('\n');
}

/**
 * Collect edit statistics from LLM execution blocks.
 * Tracks operation types and count by language.
 *
 * @param {object[]} results
 * @returns {object} { totalOps, runOps, createOps, patchOps, byLang }
 */
function collectEditStats(results) {
    var stats = {
        totalOps: 0,
        runOps: 0,
        createOps: 0,
        patchOps: 0,
        byLang: {},
    };
    if (!results || results.length === 0) {
        return stats;
    }
    for (var i = 0; i < results.length; i++) {
        var r = results[i];
        if (!r || r.ok !== true || r.type === 'undefined') {
            continue;
        }
        // If the result value carries editStats (from agent.fs.* / agent.exec.*),
        // aggregate those counts directly.
        var es = r.value && typeof r.value === 'object' ? r.value.editStats : null;
        if (es && typeof es === 'object') {
            stats.totalOps += Number(es.totalOps || 0);
            stats.runOps += Number(es.runOps || 0);
            stats.createOps += Number(es.createOps || 0);
            stats.patchOps += Number(es.patchOps || 0);
            var bl = es.byLang;
            if (bl && typeof bl === 'object') {
                var keys = Object.keys(bl);
                for (var k = 0; k < keys.length; k++) {
                    var lang = keys[k];
                    stats.byLang[lang] = (stats.byLang[lang] || 0) + Number(bl[lang] || 0);
                }
            }
        } else {
            // Plain code-block execution (jsh-run/jsh-shell/jsh-sql without agent.* calls).
            stats.totalOps++;
            stats.runOps++;
        }
    }
    return stats;
}

/**
 * Extract error location (path:line:col) from stderr/error messages.
 * Supports go/js/shell common error patterns.
 *
 * @param {string} errMsg
 * @returns {object|null} { path, line, col } or null if not found
 */
function extractErrorLocation(errMsg) {
    if (!errMsg || typeof errMsg !== 'string') {
        return null;
    }
    // Go error: "filename.go:123:1: message"
    var goMatch = /^([^\s:]+\.go):([0-9]+):([0-9]+):/m.exec(errMsg);
    if (goMatch) {
        return { path: goMatch[1], line: parseInt(goMatch[2], 10), col: parseInt(goMatch[3], 10) };
    }
    // JS runtime: "Error at line 123, col 45 in script"
    var jsMatch = /line\s+([0-9]+).*col\s+([0-9]+)/i.exec(errMsg);
    if (jsMatch) {
        return { path: 'script', line: parseInt(jsMatch[1], 10), col: parseInt(jsMatch[2], 10) };
    }
    // Unix/shell: "file.js:15: error message"
    var unixMatch = /^([^\s:]+):([0-9]+):/m.exec(errMsg);
    if (unixMatch) {
        return { path: unixMatch[1], line: parseInt(unixMatch[2], 10), col: 1 };
    }
    return null;
}

function resolveDiagnosticPath(filePath) {
    if (!filePath) {
        return '';
    }
    if (filePath === 'script') {
        return 'script';
    }
    if (String(filePath).charAt(0) === '/') {
        return String(filePath);
    }
    return path.resolve(process.cwd(), String(filePath));
}

function readDiagnosticContext(filePath, line, contextLines) {
    var target = resolveDiagnosticPath(filePath);
    if (!target || target === 'script') {
        return null;
    }
    try {
        var text = fs.readFileSync(target, 'utf8');
        var lines = String(text || '').split(/\r?\n/);
        var center = Number(line || 1);
        if (!Number.isFinite(center) || center < 1) {
            center = 1;
        }
        var around = Number(contextLines || 2);
        if (!Number.isFinite(around) || around < 0) {
            around = 2;
        }
        if (around > 10) {
            around = 10;
        }
        var startLine = Math.max(1, center - around);
        var endLine = Math.min(lines.length, center + around);
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

function collectErrorDiagnostics(results, options) {
    var opts = options || {};
    var contextLines = Number(opts.contextLines || 2);
    if (!Number.isFinite(contextLines) || contextLines < 0) {
        contextLines = 2;
    }
    var diagnostics = [];
    if (!results || results.length === 0) {
        return diagnostics;
    }
    for (var i = 0; i < results.length; i++) {
        var r = results[i] || {};
        if (r.ok !== false) {
            continue;
        }
        var msg = String(r.error || r.value || 'unknown execution error');
        var location = extractErrorLocation(msg);
        var diag = {
            message: msg,
            path: null,
            line: null,
            col: null,
            context: null,
        };
        if (location) {
            diag.path = location.path || null;
            diag.line = location.line || null;
            diag.col = location.col || null;
            var ctx = readDiagnosticContext(location.path, location.line, contextLines);
            if (ctx) {
                diag.context = ctx;
            }
        }
        diagnostics.push(diag);
    }
    return diagnostics;
}

function formatDiagnosticsPrompt(diagnostics, limit) {
    if (!diagnostics || diagnostics.length === 0) {
        return '';
    }
    var maxCount = Number(limit || 3);
    if (!Number.isFinite(maxCount) || maxCount < 1) {
        maxCount = 3;
    }
    var lines = [
        'Execution diagnostics:',
        'Use partial patch strategy first (agent.fs.patch) before full-file regeneration.',
    ];
    var n = Math.min(diagnostics.length, maxCount);
    for (var i = 0; i < n; i++) {
        var d = diagnostics[i] || {};
        if (d.path && d.line) {
            lines.push('- location: ' + d.path + ':' + d.line + (d.col ? ':' + d.col : ''));
        }
        lines.push('- error: ' + String(d.message || 'unknown error'));
        if (d.context && d.context.snippet) {
            lines.push('```');
            lines.push(String(d.context.snippet));
            lines.push('```');
        }
    }
    return lines.join('\n');
}

function buildPatchGuardrailPrompt(diagnostics, options) {
    if (!diagnostics || diagnostics.length === 0) {
        return '';
    }
    var opts = options || {};
    var maxCount = Number(opts.maxCount || 2);
    if (!Number.isFinite(maxCount) || maxCount < 1) {
        maxCount = 2;
    }
    var lines = [
        'Patch-first guardrail:',
        'Do not regenerate the whole file.',
        'Use agent.fs.patch(...) first and change only minimal lines around the diagnostic location.',
    ];
    var n = Math.min(diagnostics.length, maxCount);
    for (var i = 0; i < n; i++) {
        var d = diagnostics[i] || {};
        if (d.path && d.line) {
            lines.push('- target: ' + d.path + ':' + d.line + (d.col ? ':' + d.col : ''));
        }
        if (d.message) {
            lines.push('- reason: ' + String(d.message));
        }
    }
    lines.push('If patch is impossible, explain why before proposing full rewrite.');
    return lines.join('\n');
}

function buildAutoPatchSuggestionPrompt(diagnostics, options) {
    if (!diagnostics || diagnostics.length === 0) {
        return '';
    }
    var opts = options || {};
    var maxCount = Number(opts.maxCount || 2);
    if (!Number.isFinite(maxCount) || maxCount < 1) {
        maxCount = 2;
    }
    var lines = [
        'Auto patch suggestions:',
        'Apply one minimal patch candidate first, then rerun the command.',
        'Prefer agent.fs.patch with kind="lineRangePatch". Use anchorPatch only if line numbers are unreliable.',
    ];
    var n = Math.min(diagnostics.length, maxCount);
    for (var i = 0; i < n; i++) {
        var d = diagnostics[i] || {};
        var filePath = String(d.path || '').trim();
        var line = Number(d.line || 0);
        if (!Number.isFinite(line) || line < 1) {
            line = 1;
        }
        var col = Number(d.col || 0);
        if (!Number.isFinite(col) || col < 1) {
            col = 1;
        }
        lines.push('Candidate ' + (i + 1) + ':');
        if (filePath) {
            lines.push('- target: ' + filePath + ':' + line + ':' + col);
        }
        if (d.message) {
            lines.push('- error: ' + String(d.message));
        }
        lines.push('```json');
        lines.push('{');
        lines.push('  "path": "' + filePath.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '",');
        lines.push('  "patch": {');
        lines.push('    "kind": "lineRangePatch",');
        lines.push('    "startLine": ' + line + ',');
        lines.push('    "endLine": ' + line + ',');
        lines.push('    "replacement": "// TODO: minimal fix for the failing line"');
        lines.push('  }');
        lines.push('}');
        lines.push('```');
    }
    lines.push('Return only the minimal patch code/action, not full-file regeneration.');
    return lines.join('\n');
}

function detectPatchFirstViolation(responseText, diagnostics, options) {
    if (!responseText || !diagnostics || diagnostics.length === 0) {
        return false;
    }
    var opts = options || {};
    var lineThreshold = Number(opts.lineThreshold || 80);
    if (!Number.isFinite(lineThreshold) || lineThreshold < 10) {
        lineThreshold = 80;
    }
    var blocks = extractCodeBlocks(String(responseText));
    if (!blocks || blocks.length === 0) {
        return false;
    }
    for (var i = 0; i < blocks.length; i++) {
        var block = blocks[i] || {};
        var code = String(block.code || '');
        var lineCount = code ? code.split(/\r?\n/).length : 0;
        if (lineCount >= lineThreshold) {
            return true;
        }
    }
    return false;
}

function hasRunnableFence(text) {
    return extractCodeBlocks(text).length > 0;
}

function detectAnalysisIntent(promptText) {
    if (!promptText) {
        return false;
    }
    var text = String(promptText).toLowerCase();
    return /(\banaly[sz]e\b|\banalysis\b|\breport\b|\bsummarize\b|\bsummary\b|\bdiagnos(?:e|is)\b|\binsight\b|\bfindings\b|분석|리포트|보고서|요약|진단|통계|이상\s*징후)/i.test(text);
}

function buildEvidenceGatePrompt() {
    return [
        'Evidence-first retry:',
        'This request requires executed evidence before the final report.',
        'Start your next response with a runnable fence (`jsh-sql` or `jsh-run`).',
        'Do not provide the final report yet.',
        'Prefer a short bounded `jsh-sql` verification query first.',
    ].join('\n');
}

function _collectEvidenceTokens(executionSummary, options) {
    var opts = options || {};
    var maxNumbers = Number(opts.maxNumbers || 8);
    if (!Number.isFinite(maxNumbers) || maxNumbers < 1) {
        maxNumbers = 8;
    }
    var text = String(executionSummary || '');
    var tokens = [];
    var seen = {};
    var numberRe = /\b\d+(?:\.\d+)?\b/g;
    var match;
    while ((match = numberRe.exec(text)) !== null) {
        var token = match[0];
        if (!seen[token]) {
            seen[token] = true;
            tokens.push(token);
            if (tokens.length >= maxNumbers) {
                break;
            }
        }
    }
    return tokens;
}

function _collectEvidenceHints(evidence, options) {
    var opts = options || {};
    var maxHints = Number(opts.maxHints || 8);
    if (!Number.isFinite(maxHints) || maxHints < 1) {
        maxHints = 8;
    }
    var hints = [];
    var seen = {};
    var items = Array.isArray(evidence) ? evidence : [];
    for (var i = 0; i < items.length && hints.length < maxHints; i++) {
        var item = items[i] || {};
        if (item.kind === 'sql') {
            if (item.rowCount !== undefined && item.rowCount !== null) {
                var rowCountText = String(item.rowCount);
                if (!seen[rowCountText]) {
                    seen[rowCountText] = true;
                    hints.push(rowCountText);
                    if (hints.length >= maxHints) {
                        break;
                    }
                }
            }
            var columns = Array.isArray(item.columns) ? item.columns : [];
            for (var c = 0; c < columns.length && hints.length < maxHints; c++) {
                var col = String(columns[c] || '').trim();
                if (col && !seen[col]) {
                    seen[col] = true;
                    hints.push(col);
                }
            }
            var rows = Array.isArray(item.rows) ? item.rows : [];
            for (var r = 0; r < rows.length && hints.length < maxHints; r++) {
                var row = rows[r] || {};
                var keys = Object.keys(row);
                for (var k = 0; k < keys.length && hints.length < maxHints; k++) {
                    var value = row[keys[k]];
                    if (typeof value === 'number' || typeof value === 'string') {
                        var token = String(value);
                        if (token && !seen[token]) {
                            seen[token] = true;
                            hints.push(token);
                        }
                    }
                }
            }
            var rendered = String(item.rendered || '');
            var numbers = rendered.match(/\b\d+(?:\.\d+)?\b/g) || [];
            for (var rn = 0; rn < numbers.length && hints.length < maxHints; rn++) {
                var numToken = String(numbers[rn] || '');
                if (numToken && !seen[numToken]) {
                    seen[numToken] = true;
                    hints.push(numToken);
                }
            }
            continue;
        }
        if (item.kind === 'viz') {
            var vizTokens = [item.renderer, item.mode];
            for (var vt = 0; vt < vizTokens.length && hints.length < maxHints; vt++) {
                var one = String(vizTokens[vt] || '').trim();
                if (one && !seen[one]) {
                    seen[one] = true;
                    hints.push(one);
                }
            }
        }
    }
    return hints;
}

function buildGroundedReportPrompt(evidence, options) {
    var opts = options || {};
    var evidenceTokens = _collectEvidenceHints(evidence, { maxHints: opts.maxHints || 8 });
    var lines = [
        'Grounded report retry:',
        'Write the report from the executed evidence only.',
        'Explicitly cite the observed values, counts, ranges, aggregates, or render outputs from the immediately preceding execution results.',
        'Do not ask the user to run queries manually.',
        'Do not provide unsupported generic conclusions.',
    ];
    if (evidenceTokens.length > 0) {
        lines.push('Observed value hints: ' + evidenceTokens.join(', '));
    }
    return lines.join('\n');
}

function detectUngroundedReport(responseText, evidence, options) {
    if (!responseText || !evidence || !Array.isArray(evidence) || evidence.length === 0) {
        return false;
    }
    if (hasRunnableFence(responseText)) {
        return false;
    }
    var opts = options || {};
    var minResponseLength = Number(opts.minResponseLength || 40);
    if (!Number.isFinite(minResponseLength) || minResponseLength < 1) {
        minResponseLength = 40;
    }
    var response = String(responseText || '');
    if (response.trim().length < minResponseLength) {
        return false;
    }
    if (/blocked|cannot execute|execution is impossible|unable to execute|실행할 수 없|차단되었|불가능/.test(response.toLowerCase())) {
        return false;
    }
    if (/run (these|this) quer(?:y|ies)|paste (the )?result|share the result|쿼리를 실행|결과를 붙여넣|결과를 공유/.test(response.toLowerCase())) {
        return true;
    }

    var evidenceTokens = _collectEvidenceHints(evidence, { maxHints: opts.maxHints || 8 });
    if (evidenceTokens.length > 0) {
        for (var i = 0; i < evidenceTokens.length; i++) {
            if (response.indexOf(evidenceTokens[i]) >= 0) {
                return false;
            }
        }
        return true;
    }

    for (var j = 0; j < evidence.length; j++) {
        if (evidence[j] && evidence[j].kind === 'viz') {
            if (/render|chart|plot|series|blocks|lines|시각화|차트|그래프/.test(response.toLowerCase())) {
                return false;
            }
            return true;
        }
    }
    return false;
}

/**
 * Format an array of AgentRenderer result objects into a human-readable string.
 * Used to produce the tool-result message inserted into conversation history.
 *
 * @param {object[]} results
 * @returns {string}
 */
function formatResults(results) {
    if (!results || results.length === 0) {
        return '(no output)';
    }
    var lines = [];
    for (var i = 0; i < results.length; i++) {
        var r = results[i];
        if (!r.ok) {
            lines.push('Error: ' + r.error);
        } else if (isSqlEvidence(r.value)) {
            lines.push(String(r.value.rendered || '(no rows)'));
        } else if (isRenderEnvelope(r.value)) {
            var env = r.value;
            var rendererName = String(env.renderer || 'viz.tui');
            if (env.mode === 'blocks') {
                var count = Array.isArray(env.blocks) ? env.blocks.length : 0;
                lines.push('[rendered ' + rendererName + ' blocks: ' + count + ']');
            } else {
                var lineCount = Array.isArray(env.lines) ? env.lines.length : 0;
                lines.push('[rendered ' + rendererName + ' lines: ' + lineCount + ']');
            }
        } else if (r.truncated && typeof r.value === 'string' && r.value.indexOf('[truncated:') === 0) {
            lines.push('[render payload truncated: increase maxOutputBytes to render viz output]');
            lines.push(r.value + '  [truncated]');
        } else if (r.type === 'print') {
            // console.log/println output captured during jsh execution.
            lines.push(String(r.value));
        } else if (r.truncated) {
            lines.push(JSON.stringify(r.value) + '  [truncated]');
        } else if (r.value === undefined || r.value === null || r.type === 'undefined') {
            // Skip pure undefined — these are declaration side-effects, not output.
        } else if (typeof r.value === 'object') {
            lines.push(JSON.stringify(r.value, null, 2));
        } else {
            lines.push(String(r.value));
        }
    }
    return lines.length > 0 ? lines.join('\n') : '(no output)';
}

module.exports = {
    extractCodeBlocks,
    extractRunnableCandidates,
    tryPromotePlainSqlBlock,
    tryPromotePlainJsBlock,
    hasRunnableFence,
    detectAnalysisIntent,
    buildEvidenceGatePrompt,
    buildGroundedReportPrompt,
    detectUngroundedReport,
    executeJsh,
    executeBlock,
    formatResults,
    isSqlEvidence,
    isRenderEnvelope,
    collectRenderEnvelopes,
    collectExecutionEvidence,
    formatEvidencePrompt,
    collectEditStats,
    extractErrorLocation,
    collectErrorDiagnostics,
    formatDiagnosticsPrompt,
    buildPatchGuardrailPrompt,
    buildAutoPatchSuggestionPrompt,
    detectPatchFirstViolation,
};
