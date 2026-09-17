'use strict';

const tableOptions = require('help/table_options');

module.exports = {
    name: 'sql',
    description: 'Execute an SQL command',
    usage: 'Usage: sql [options] <sql>',
    options: {
        help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
        output: { type: 'string', short: 'o', description: "output file (default:'-' stdout)", default: '-' },
        compress: { type: 'string', description: "compression type (none, gzip)", default: 'none' },
        timing: { type: 'boolean', short: 'T', description: "print elapsed time", default: false },
        showTz: { type: 'boolean', short: 'Z', description: "show time zone in datetime column header", default: false },
        progress: { type: 'integer', description: "the expected maximum progress value (0: unknown, -1: disable)", default: 0 },
        ...tableOptions,
    },
    positionals: [
        { name: 'sql', type: 'string', variadic: true, description: 'SQL query to execute' },
    ],
};