'use strict';

module.exports = {
    name: 'use',
    description: 'Select the current database',
    usage: 'Usage: use <database>',
    options: {
        help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    },
    positionals: [
        { name: 'database', type: 'string', description: 'Database name' },
    ],
};