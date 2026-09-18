'use strict';

const { command, root } = require('help/config');

module.exports = root('statz', 'Inspect server statistics', 'Usage: statz <command> [options]', {
    list: command('statz list <name>', 'List available statz metrics matching the given pattern', {
        table: true,
        positionals: [{ name: 'names', variadic: true, optional: true, description: 'The names of the statz metrics to list' }],
    }),
    get: command('statz get [name]', 'Get the specified statz metrics', {
        table: true,
        options: { nrow: { type: 'integer', short: 'n', description: 'number of rows to retrieve', default: 1 } },
        positionals: [{ name: 'names', variadic: true, description: 'The names of the statz metrics to retrieve' }],
    }),
    viz: command('statz viz [name]', 'Get the specified statz metrics and render them as a visualization', {
        table: true,
        positionals: [{ name: 'names', variadic: true, description: 'The names of the statz metrics to retrieve' }],
    }),
});