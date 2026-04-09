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
        "        return '(no rows)';",
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
        '    return rendered;',
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
    executeJsh,
    executeBlock,
    formatResults,
    isRenderEnvelope,
    collectRenderEnvelopes,
};
