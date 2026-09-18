'use strict';

const { command, root } = require('help/config');

module.exports = root('key', 'Manage X.509 keys and auth-tokens', 'Usage: key <command> [options]', {
    list: command('key list', 'List all registered keys', { table: true }),
    gen: command('key gen [options]', 'Generate new key with the given name', {
        allowNegative: true,
        options: {
            output: { type: 'string', short: 'o', description: 'Output directory for the new key files', default: '-' },
            type: { type: 'string', short: 't', description: 'Type of key to generate (RSA or ECDSA)', default: 'ECDSA' },
            store: { type: 'boolean', short: 's', description: 'Whether to store the generated key in the server', default: true },
        },
        positionals: [{ name: 'name', description: 'The CommonName for the new key; duplicates are allowed' }],
    }),
    del: command('key del <id>', 'Delete an existing key', {
        positionals: [{ name: 'id', description: 'The management id of the key to delete, as shown by key list' }],
    }),
    'server-cert': command('key server-cert', 'Retrieve server certificate', {
        options: { output: { type: 'string', short: 'o', description: 'Output file for the server certificate', default: '-' } },
    }),
});