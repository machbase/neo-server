'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const { Repl } = require('@jsh/shell');
const help = require('help');
const metadata = require('/usr/share/help/jsh/repl');

let values = {};
let parseError = null;
try {
    const parsed = parseArgs(process.argv.slice(2), metadata);
    values = parsed.values;
} catch (err) {
    parseError = err;
}

if (parseError || values.help) {
    if (parseError) {
        console.println('Error:', parseError.message);
    }
    console.println(help.format(metadata));
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
