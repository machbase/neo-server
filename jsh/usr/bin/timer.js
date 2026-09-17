'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/timer');

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
    client.listTimers()
        .then((lst) => {
            let box = pretty.Table(config);
            box.appendHeader(["ID", "NAME", "SPEC", "TQL", "AUTOSTART", "STATE"]);
            for (const timer of lst) {
                box.append([
                    timer.id,
                    timer.name,
                    timer.schedule,
                    timer.task,
                    timer.autoStart ? 'YES' : 'NO',
                    timer.state,
                ]);
            }
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function doAdd(config, args) {
    const client = new neoapi.Client();
    const name = args.name;
    const spec = args.spec;
    const tqlPath = args.tqlPath;
    const autostart = config.autostart || false;
    client.addTimer({ name: name, spec: spec, command: tqlPath, autoStart: autostart })
        .then(() => {
            console.println(`Timer '${name}' added successfully.`);
        })
        .catch((err) => {
            let message = err.message;
            //trim 'JSON-RPC error: ' prefix if exists
            if (message.startsWith('JSON-RPC error: ')) {
                message = message.substring('JSON-RPC error: '.length);
            }
            console.println('Error adding timer:', message);
        });
}

function doDel(config, args) {
    const client = new neoapi.Client();
    client.deleteTimer(Number(args.id))
        .then(() => {
            console.println(`Timer '${args.id}' deleted successfully.`);
        })
        .catch((err) => {
            console.println('Error deleting timer:', err.message);
        });
}

function doStart(config, args) {
    const client = new neoapi.Client();
    client.startTimer(Number(args.id))
        .then(() => {
            console.println(`Timer '${args.id}' started successfully.`);
        })
        .catch((err) => {
            console.println('Error starting timer:', err.message);
        });
}

function doStop(config, args) {
    const client = new neoapi.Client();
    client.stopTimer(Number(args.id))
        .then(() => {
            console.println(`Timer '${args.id}' stopped successfully.`);
        })
        .catch((err) => {
            console.println('Error stopping timer:', err.message);
        });
}