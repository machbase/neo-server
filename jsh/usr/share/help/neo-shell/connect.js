'use strict';

module.exports = {
    name: 'connect',
    description: 'Connect to a database',
    usage: 'Usage: connect [options] [user:password@]host[:port]',
    options: {
        help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    },
    positionals: [
        { name: 'connection', type: 'string', optional: true, description: 'Connection or credentials' },
    ],
};