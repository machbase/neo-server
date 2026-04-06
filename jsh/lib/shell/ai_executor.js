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

/**
 * Extract fenced code blocks tagged as jsh-run from an LLM response.
 * Plain fences and explanatory jsh examples are ignored so only explicit
 * execution candidates are considered runnable.
 *
 * Returns an array of { lang, code } objects in document order.
 *
 * @param {string} text  LLM response text
 * @returns {{ lang: string, code: string }[]}
 */
function extractCodeBlocks(text) {
    var blocks = [];
    // Match ```jsh-run\n<body>```. The closing ``` may be followed by nothing
    // or a newline.
    var re = /```(jsh-run)\n([\s\S]*?)```/g;
    var m;
    while ((m = re.exec(text)) !== null) {
        var code = m[2];
        if (code && code.trim()) {
            blocks.push({ lang: m[1], code: code });
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

module.exports = { extractCodeBlocks, executeJsh, formatResults };
