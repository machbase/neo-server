'use strict';

const { simple } = require('help/config');

module.exports = simple('import', 'Import data into a table', 'Usage: import [options] <table>', {
    allowNegative: true,
    options: {
        input: { type: 'string', short: 'i', description: "input file (default:'-' stdin)", default: '-' },
        compress: { type: 'string', description: 'compression type (none, gzip)', default: 'none' },
        format: { type: 'string', short: 'f', description: 'input format (csv, tsv, ndjson)', default: 'csv' },
        timeformat: { type: 'string', short: 't', description: 'time format [ns|us|ms|s|<timeformat>]', default: 'ns' },
        tz: { type: 'string', description: 'time zone for handling datetime (default: time zone)', default: 'local' },
        header: { type: 'string', description: 'header option [skip|columns|none]', default: 'none' },
        nullValue: { type: 'string', description: 'string to represent null values', default: 'NULL' },
        dryRun: { type: 'boolean', description: 'run in dry mode', default: false },
        verbose: { type: 'boolean', description: 'verbose mode, it works only with --dry-run', default: false },
    },
    positionals: [{ name: 'table', type: 'string', description: 'table name to read' }],
});