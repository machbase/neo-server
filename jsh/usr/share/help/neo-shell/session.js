'use strict';

const { command, root } = require('help/config');

module.exports = root('session', 'Manage sessions', 'Usage: session <command> [options]', {
    list: command('session list', 'List all sessions', {
        table: true,
        allowNegative: true,
        options: { all: { type: 'boolean', description: 'Include details' } },
    }),
    kill: command('session kill <id>', 'Force to close the session by session ID', {
        options: { force: { type: 'boolean', description: 'Force kill the session', default: false } },
        positionals: [{ name: 'id', description: 'ID of the session to kill' }],
    }),
    stat: command('session stat [options]', 'Show detailed information about sessions', {
        table: true,
        allowNegative: true,
        options: { reset: { type: 'boolean', description: 'Reset statistics after showing', default: false } },
    }),
    limit: command('session limit', 'Get session limits', { table: true, allowNegative: true }),
    'set-limit': command('session set-limit [options]', 'Set session limits', {
        options: {
            maxOpenConn: { type: 'integer', description: 'Maximum number of open connections to the database' },
            maxIdleConn: { type: 'integer', description: 'Maximum number of idle connections to the database' },
            connMaxIdletime: { type: 'string', description: 'Maximum idle time for a connection (e.g., "30s", "5m")' },
            connMaxLifetime: { type: 'string', description: 'Maximum lifetime for a connection (e.g., "1h", "24h")' },
        },
    }),
});