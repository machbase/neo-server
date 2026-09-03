'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
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
}
const delConfig = {
    func: doDel,
    command: 'del',
    usage: 'subscriber del <id>',
    description: 'Delete a subscriber by name',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'id', description: 'ID of the subscriber to delete' },
    ],
}

const startConfig = {
    func: doStart,
    command: 'start',
    usage: 'subscriber start <id>',
    description: 'Start a subscriber by name',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'id', description: 'ID of the subscriber to start' },
    ],
}

const stopConfig = {
    func: doStop,
    command: 'stop',
    usage: 'subscriber stop <id>',
    description: 'Stop a subscriber by name',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'id', description: 'ID of the subscriber to stop' },
    ],
}

parseAndRun(process.argv.slice(2), defaultConfig, [
    listConfig,
    addConfig,
    delConfig,
    startConfig,
    stopConfig,
]);

function doList(config, args) {
    const client = new neoapi.Client(config);
    client.listSubscribers()
        .then((lst) => {
            let box = pretty.Table(config);
            box.appendHeader(["ID", "NAME", "BRIDGE", "TOPIC", "DESTINATION", "AUTOSTART", "STATE"]);
            for (const subs of lst) {
                box.append([
                    subs.id,
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
    const autostart = config.autostart || false;
    const qos = config.qos || 0;
    client.addSubscriber({ name: name, bridge: bridge, command: destination, autoStart: autostart, topic: topic, qos: qos })
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
    client.deleteSubscriber(Number(args.id))
        .then(() => {
            console.println(`Subscriber '${args.id}' deleted successfully.`);
        })
        .catch((err) => {
            console.println('Error deleting subscriber:', err.message);
        });
}

function doStart(config, args) {
    const client = new neoapi.Client();
    client.startSubscriber(Number(args.id))
        .then(() => {
            console.println(`Subscriber '${args.id}' started successfully.`);
        })
        .catch((err) => {
            console.println('Error starting subscriber:', err.message);
        });
}

function doStop(config, args) {
    const client = new neoapi.Client();
    client.stopSubscriber(Number(args.id))
        .then(() => {
            console.println(`Subscriber '${args.id}' stopped successfully.`);
        })
        .catch((err) => {
            console.println('Error stopping subscriber:', err.message);
        });
}