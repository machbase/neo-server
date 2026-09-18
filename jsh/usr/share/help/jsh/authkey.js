'use strict';
const { command, root } = require('help/config');
module.exports = root('authkey', 'Generate auth key files for Machbase challenge authentication', 'Usage: authkey <command> [options]', {
    gen: command('Usage: authkey gen -t [rsa|ecdsa] -o OUTPUT_PATH', 'Generate auth private/public key files', {
        strict: false,
        options: {
            type: { type: 'string', short: 't', description: 'key type: rsa or ecdsa', default: 'ecdsa' },
            output: { type: 'string', short: 'o', description: 'output base path (prefix with @ for host OS path)' },
        },
    }),
});