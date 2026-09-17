'use strict';

const { command, root } = require('help/config');

module.exports = root('shell', 'Manage custom shells', 'Usage: shell <command> [options]', {
    list: command('shell list', 'List all shells', { table: true }),
    add: command('shell add <name> <bin-path> [args...]', 'Add a new shell with given name and binary path', {
        positionals: [
            { name: 'name', description: 'Shell name' },
            { name: 'bin-path', description: 'Executable path' },
            { name: 'args', variadic: true, optional: true, description: 'Executable arguments' },
        ],
    }),
    del: command('shell del <id>', 'Delete a shell by ID', {
        positionals: [{ name: 'id', description: 'Shell ID' }],
    }),
});