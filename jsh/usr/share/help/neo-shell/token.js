'use strict';

const { command, root } = require('help/config');

module.exports = root('token', 'Manage API tokens', 'Usage: token <command> [options]', {
    list: command('token list', 'List your API tokens', { table: true }),
    gen: command('token gen <name> [--not-after <date>]', 'Generate an API token', {
        options: { notAfter: { type: 'string', description: 'Expiration date in ISO-8601 format', default: '' } },
        positionals: [{ name: 'name', description: 'A label for the token' }],
    }),
    del: command('token del <id>', 'Delete one of your API tokens', {
        positionals: [{ name: 'id', description: 'Token ID' }],
    }),
});