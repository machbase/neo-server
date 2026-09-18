'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const { newMachCliClient } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/export');

let showHelp = true;
let config = {};
let tableName = '';
try {
    const parsed = parseArgs(process.argv.slice(2), help);
    config = parsed.values;
    tableName = parsed.namedPositionals.table;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp || (!tableName) || tableName.length === 0) {
    console.println(parseArgs.formatHelp(help));
    process.exit(showHelp ? 0 : 1);
}

let args = [
    '--output', config.output,
    '--compress', config.compress,
    '--format', config.format,
    '--timeformat', config.timeformat,
    '--tz', config.tz,
    '--precision', config.precision,
    '--null-value', config.nullValue,
    '--no-rownum',
    '--no-pause',
    '--no-footer',
];

if (config.header) {
    args.push('--header');
} else {
    args.push('--no-header');
}

if (config.silent) {
    args.push('--progress', '-1');
} else {
    let db, conn;
    try {
        db = newMachCliClient(config);
        conn = db.connect();
        const result = conn.queryRow(`SELECT COUNT(*) AS count FROM ${tableName}`);
        const rowCount = result.count || 0;
        args.push('--progress', rowCount);
    } catch (err) {
        console.println(`Failed to get row count: ${err.message}`);
        process.exit(1);
    } finally {
        conn && conn.close();
        db && db.close();
    }
}

args.push(`SELECT * FROM ${tableName}`);

process.exec('sql', ...args);