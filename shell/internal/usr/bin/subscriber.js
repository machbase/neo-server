'use strict';

const process = require('process');
const neoapi = require('/usr/lib/neoapi');
const pretty = require('/usr/lib/pretty');
const { parseAndRun } = require('/usr/lib/opts');

const optionHelp = { type: 'boolean', short: 'h', description: 'Show this help message', default: false }

const defaultConfig = {
    usage: 'Usage: subscriber <command> [options]',
    options: {
        help: optionHelp,
    }
};

const listConfig = {
    func: doList,
    command: 'list',
    usage: 'subscriber list',
    description: 'List all registered subscribers',
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    }
}

const addConfig = {
    func: doAdd,
    command: 'add',
    usage: 'subscriber add [options] <name> <bridge> <topic> <destination>',
    description: 'Add a new subscriber to the topic via pre-defined bridge',
    options: {
        help: optionHelp,
        autoStart: { type: 'boolean', description: 'Enable autostart for the subscriber', default: false },

    },
    allowNegative: false,
    positionals: [
        { name: 'name', description: 'Name of the subscriber' },
        { name: 'bridge', description: 'Name of the pre-defined bridge to use' },
        { name: 'topic', description: 'Topic to subscribe to' },
        { name: 'destination', description: 'Destination to forward messages to (e.g., tql path, writing path descriptor)' },
    ],
    longDescription: `  ex)
    subscriber add --auto-start --qos=1 my_lsnr my_mqtt outer/events /my_event.tql
    subscriber add my_append nats_bridge stream.in db/append/EXAMPLE:json
    subscriber add my_writer nats_bridge topic.in  db/write/EXAMPLE:csv:gzip
    `,
}
const deleteConfig = {
    func: doDel,
    command: 'delete',
    usage: 'subscriber del <name>',
    description: 'Delete a subscriber by name',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'name', description: 'name of the subscriber to delete' },
    ],
}

const startConfig = {
    func: doStart,
    command: 'start',
    usage: 'subscriber start <name>',
    description: 'Start a subscriber by name',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'name', description: 'name of the subscriber to start' },
    ],
}

const stopConfig = {
    func: doStop,
    command: 'stop',
    usage: 'subscriber stop <name>',
    description: 'Stop a subscriber by name',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'name', description: 'name of the subscriber to stop' },
    ],
}

parseAndRun(process.argv.slice(2), defaultConfig, [
    listConfig,
    addConfig,
    deleteConfig,
    startConfig,
    stopConfig,
]);

function doList(config, args) {
    const client = new neoapi.Client(config);
    client.listSchedules()
        .then((lst) => {
            let box = pretty.Table(config);
            box.appendHeader(["NAME", "BRIDGE", "TOPIC", "DESTINATION", "AUTOSTART", "STATE"]);
            for (const subs of lst) {
                if (subs.type !== 'SUBSCRIBER') {
                    continue;
                }
                box.append([
                    subs.name,
                    subs.bridge,
                    subs.topic,
                    subs.task,
                    subs.autoStart ? 'YES' : 'NO',
                    subs.state,
                ]);
            }
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function doAdd(config, args) {
    // subscriber add [options] <name> <bridge> <topic> <destination>
    const client = new neoapi.Client();
    const name = args.name;
    const bridge = args.bridge;
    const topic = args.topic;
    const destination = args.destination;
    const autoStart = args.autoStart || false;
    client.addSchedule({ name: name, type: 'SUBSCRIBER', bridge: bridge, topic: topic, task: destination, autoStart: autoStart })
        .then(() => {
            console.println(`Subscriber '${name}' added successfully.`);
        })
        .catch((err) => {
            let message = err.message;
            //trim 'JSON-RPC error: ' prefix if exists
            if (message.startsWith('JSON-RPC error: ')) {
                message = message.substring('JSON-RPC error: '.length);
            }
            console.println('Error adding subscriber:', message);
        });
}

function doDel(config, args) {
    const client = new neoapi.Client();
    client.deleteSchedule(args.name)
        .then(() => {
            console.println(`Subscriber '${args.name}' deleted successfully.`);
        })
        .catch((err) => {
            console.println('Error deleting subscriber:', err.message);
        });
}

function doStart(config, args) {
    const client = new neoapi.Client();
    client.startSchedule(args.name)
        .then(() => {
            console.println(`Subscriber '${args.name}' started successfully.`);
        })
        .catch((err) => {
            console.println('Error starting subscriber:', err.message);
        });
}

function doStop(config, args) {
    const client = new neoapi.Client();
    client.stopSchedule(args.name)
        .then(() => {
            console.println(`Subscriber '${args.name}' stopped successfully.`);
        })
        .catch((err) => {
            console.println('Error stopping subscriber:', err.message);
        });
}