'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const machcli = require('/usr/lib/machcli');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    output: { type: 'string', short: 'o', description: "output file (default:'-' stdout)", default: '-' },
    compress: { type: 'string', description: "compression type (none, gzip)", default: 'none' },
    format: { type: 'string', short: 'f', description: "output format (box, csv, tsv, json, ndjson)", default: 'csv' },
    timeformat: { type: 'string', short: 't', description: "time format [ns|us|ms|s|<timeformat>]", default: 'ns' },
    tz: { type: 'string', description: "time zone for handling datetime (default: time zone)", default: 'local' },
    precision: { type: 'integer', short: 'p', description: "set precision of float value to force round", default: -1 },
    header: { type: 'boolean', description: "print header", default: false },
    nullValue: { type: 'string', description: "string to represent null values", default: '' },
    silent: { type: 'boolean', description: "suppress progress output", default: false },
}

const positionals = [
    { name: 'table', type: 'string', description: 'table name to read' }
];

let showHelp = true;
let config = {};
let tableName = '';
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
        allowPositionals: true,
        allowNegative: true,
        positionals: positionals
    });
    config = parsed.values;
    tableName = parsed.namedPositionals.table;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp || (!tableName) || tableName.length === 0) {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: export [options] <table>',
        options,
        positionals: positionals
    }));
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
        db = new machcli.Client(config);
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