'use strict';

const { simple } = require('help/config');

module.exports = simple('export', 'Export a table', 'Usage: export [options] <table>', {
    allowNegative: true,
    options: {
        output: { type: 'string', short: 'o', description: "output file (default:'-' stdout)", default: '-' },
        compress: { type: 'string', description: 'compression type (none, gzip)', default: 'none' },
        format: { type: 'string', short: 'f', description: 'output format (box, csv, tsv, json, ndjson)', default: 'csv' },
        timeformat: { type: 'string', short: 't', description: 'time format [ns|us|ms|s|<timeformat>]', default: 'ns' },
        tz: { type: 'string', description: 'time zone for handling datetime (default: time zone)', default: 'local' },
        precision: { type: 'integer', short: 'p', description: 'set precision of float value to force round', default: -1 },
        header: { type: 'boolean', description: 'print header', default: false },
        nullValue: { type: 'string', description: 'string to represent null values', default: '' },
        silent: { type: 'boolean', description: 'suppress progress output', default: false },
    },
    positionals: [{ name: 'table', type: 'string', description: 'table name to read' }],
});