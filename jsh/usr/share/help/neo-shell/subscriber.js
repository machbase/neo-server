'use strict';

const { command, root } = require('help/config');

module.exports = root('subscriber', 'Manage subscribers', 'Usage: subscriber <command> [options]', {
    list: command('subscriber list', 'List all registered subscribers', { table: true }),
    add: command('subscriber add [options] <name> <bridge> <topic> <destination>', 'Add a new subscriber to the topic via pre-defined bridge', {
        options: {
            autostart: { type: 'boolean', description: 'Enable autostart for the subscriber', default: false },
            qos: { type: 'integer', description: 'QoS level for MQTT bridge (0, 1, or 2)', default: 0 },
        },
        allowNegative: false,
        positionals: [
            { name: 'name', description: 'Name of the subscriber' },
            { name: 'bridge', description: 'Name of the pre-defined bridge to use' },
            { name: 'topic', description: 'Topic to subscribe to' },
            { name: 'destination', description: 'Destination to forward messages to (e.g., tql path, writing path descriptor)' },
        ],
        longDescription: `  ex)
    subscriber add --autostart --qos=1 my_lsnr my_mqtt outer/events /my_event.tql
    subscriber add my_append nats_bridge stream.in db/append/EXAMPLE:json
    subscriber add my_writer nats_bridge topic.in  db/write/EXAMPLE:csv:gzip
    `,
    }),
    del: command('subscriber del <id>', 'Delete a subscriber by name', { positionals: [{ name: 'id', description: 'ID of the subscriber to delete' }] }),
    start: command('subscriber start <id>', 'Start a subscriber by name', { positionals: [{ name: 'id', description: 'ID of the subscriber to start' }] }),
    stop: command('subscriber stop <id>', 'Stop a subscriber by name', { positionals: [{ name: 'id', description: 'ID of the subscriber to stop' }] }),
});