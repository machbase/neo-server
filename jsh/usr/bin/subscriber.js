'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/subscriber');

const commandFunctions = {
    list: doList,
    add: doAdd,
    del: doDel,
    start: doStart,
    stop: doStop,
};
const commandConfigs = Object.keys(help.commands).map((name) => ({
    ...help.commands[name],
    command: name,
    func: commandFunctions[name],
}));

parseAndRun(process.argv.slice(2), help, commandConfigs);

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
    client.listBridges()
        .then((bridges) => {
            const found = (bridges || []).find((item) => {
                const bridgeName = item.name || item.Name;
                return bridgeName === bridge;
            });
            const bridgeType = ((found && (found.type || found.Type)) || '').toLowerCase();
            const request = { name: name, bridge: bridge, command: destination, autoStart: autostart };
            if (bridgeType === 'nats') {
                request.nats = { subject: topic };
            } else {
                request.mqtt = { topic: topic, qos: qos };
            }
            return client.addSubscriber(request);
        })
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