'use strict';

const { command, root } = require('help/config');

module.exports = root('timer', 'Manage timers', 'Usage: timer <command> [options]', {
    list: command('timer list', 'List all registered timers', { table: true }),
    add: command('timer add [options] <name> <spec> <tql-path>', 'Add a new timer', {
        options: { autostart: { type: 'boolean', description: 'Enable autostart for the timer', default: false } },
        positionals: [
            { name: 'name', description: 'Name of the timer' },
            { name: 'spec', description: 'Timer specification in cron format' },
            { name: 'tql-path', description: 'Path to the TQL file to execute' },
        ],
        longDescription: `
    ex)
        timer add --autostart my_sched '@every 10s' /hello.tql
    `,
    }),
    del: command('timer del <id>', 'Delete an existing timer', { positionals: [{ name: 'id', description: 'ID of the timer to delete' }] }),
    start: command('timer start <id>', 'Start a timer', { positionals: [{ name: 'id', description: 'ID of the timer to start' }] }),
    stop: command('timer stop <id>', 'Stop a timer', { positionals: [{ name: 'id', description: 'ID of the timer to stop' }] }),
});