'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const { Repl } = require('@jsh/shell');

const options = {
    profile: {
        type: 'string',
        short: 'p',
        description: 'Profile to use (default, user, agent)',
        default: 'default',
    },
    load: {
        type: 'string',
        short: 'l',
        description: 'Execute a file at startup (may be specified multiple times)',
        multiple: true,
    },
    require: {
        type: 'string',
        short: 'r',
        description: 'Require a module at startup (may be specified multiple times)',
        multiple: true,
    },
    eval: {
        type: 'string',
        short: 'e',
        description: 'Evaluate JavaScript code and exit',
    },
    print: {
        type: 'string',
        description: 'Evaluate JavaScript code, print the result, and exit',
    },
    noHistory: {
        type: 'boolean',
        description: 'Disable history persistence',
        default: false,
    },
    json: {
        type: 'boolean',
        short: 'j',
        description: 'Output evaluation results as structured JSON (agent mode)',
        default: false,
    },
    readOnly: {
        type: 'boolean',
        description: 'Deny write operations such as agent.db.exec (agent mode)',
        default: false,
    },
    timeout: {
        type: 'string',
        description: 'Per-evaluation timeout in milliseconds, e.g. 5000 (agent mode)',
    },
    maxRows: {
        type: 'string',
        description: 'Maximum rows per query, e.g. 500 (agent mode, default: 1000)',
    },
    maxOutputBytes: {
        type: 'string',
        description: 'Maximum serialized output bytes, e.g. 32768 (agent mode, default: 65536)',
    },
    transcript: {
        type: 'string',
        short: 't',
        description: 'Path to write a newline-delimited JSON transcript of all inputs and results',
    },
    historyName: {
        type: 'string',
        description: 'Custom name for the history file (default: repl_history)',
    },
    help: {
        type: 'boolean',
        short: 'h',
        description: 'Show this help message',
        default: false,
    },
};

let values = {};
let parseError = null;
try {
    const parsed = parseArgs(process.argv.slice(2), { options, strict: false });
    values = parsed.values;
} catch (err) {
    parseError = err;
}

if (parseError || values.help) {
    if (parseError) {
        console.println('Error:', parseError.message);
    }
    console.println(parseArgs.formatHelp({
        usage: 'Usage: repl [options]',
        description: 'JSH JavaScript REPL — interactive JavaScript console',
        options,
    }));
    console.println('');
    console.println('Constraints:');
    console.println("  no 'await'  — async APIs are wrapped as synchronous calls");
    console.println("  no 'import' — use require('module') instead");
    console.println('  Buffer and URL are available implicitly');
    process.exit(parseError ? 1 : 0);
}

const opts = {};

if (values.eval !== undefined) {
    opts.eval = values.eval;
    opts.print = false;
} else if (values.print !== undefined) {
    opts.eval = values.print;
    opts.print = true;
}

if (values.load !== undefined) {
    opts.load = Array.isArray(values.load) ? values.load : [values.load];
}
if (values.require !== undefined) {
    opts.require = Array.isArray(values.require) ? values.require : [values.require];
}
if (values.noHistory) {
    opts.noHistory = true;
}
if (values.historyName) {
    opts.historyName = values.historyName;
}
if (values.profile && values.profile !== 'default') {
    opts.profile = values.profile; // 'user' / 'agent' selects the profile
}
if (values.json) {
    opts.json = true;
}
if (values.readOnly) {
    opts.readOnly = true;
}
if (values.timeout !== undefined) {
    const n = parseInt(values.timeout, 10);
    if (!isNaN(n) && n > 0) { opts.timeoutMs = n; }
}
if (values.maxRows !== undefined) {
    const n = parseInt(values.maxRows, 10);
    if (!isNaN(n) && n > 0) { opts.maxRows = n; }
}
if (values.maxOutputBytes !== undefined) {
    const n = parseInt(values.maxOutputBytes, 10);
    if (!isNaN(n) && n > 0) { opts.maxOutputBytes = n; }
}
if (values.transcript !== undefined) {
    opts.transcript = values.transcript;
}

const r = new Repl();
r.loop(opts);
