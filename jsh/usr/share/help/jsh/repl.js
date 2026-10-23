'use strict';
const { simple } = require('help/config');
module.exports = simple('repl', 'Interactive JavaScript console', 'Usage: repl [options]', {
    strict: false,
    options: {
        profile: { type: 'string', short: 'p', description: 'Profile to use (default, user)', default: 'default' },
        load: { type: 'string', short: 'l', description: 'Execute a file at startup (may be specified multiple times)', multiple: true },
        require: { type: 'string', short: 'r', description: 'Require a module at startup (may be specified multiple times)', multiple: true },
        eval: { type: 'string', short: 'e', description: 'Evaluate JavaScript code and exit' },
        print: { type: 'string', description: 'Evaluate JavaScript code, print the result, and exit' },
        noHistory: { type: 'boolean', description: 'Disable history persistence', default: false },
        json: { type: 'boolean', short: 'j', description: 'Output evaluation results as structured JSON', default: false },
        timeout: { type: 'string', description: 'Per-evaluation timeout in milliseconds, e.g. 5000' },
        maxOutputBytes: { type: 'string', description: 'Maximum JSON output value size in bytes, e.g. 32768 (default: 65536)' },
        transcript: { type: 'string', short: 't', description: 'Path to write a newline-delimited JSON transcript of all inputs and results' },
        historyName: { type: 'string', description: 'Custom name for the history file (default: repl_history)' },
    },
    longDescription: `
Constraints:
  no 'await'  — async APIs are wrapped as synchronous calls
  no 'import' — use require('module') instead
  Buffer and URL are available implicitly`,
});