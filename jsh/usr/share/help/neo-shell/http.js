'use strict';

const { command, root } = require('help/config');

module.exports = root('http', 'Manage HTTP server settings', 'Usage: http <command> [options]', {
    debug: command('http debug', 'Show or set HTTP debug mode configuration', {
        table: true,
        options: {
            enable: { type: 'string', description: 'Set debug mode (true/false)', default: '' },
            logLatency: { type: 'string', description: 'Log requests that take longer than the specified duration (e.g., 100ms)', default: '-1' },
        },
    }),
});