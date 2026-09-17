'use strict';

const { simple } = require('help/config');

module.exports = simple('explain', 'Explain a query plan', 'Usage: explain [options] <sql>', {
    allowNegative: true,
    options: { full: { type: 'boolean', short: 'f', description: 'Show full explain plan', default: false } },
    positionals: [{ name: 'sql', type: 'string', variadic: true, description: 'SQL query to explain' }],
});