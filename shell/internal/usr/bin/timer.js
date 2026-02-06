'use strict';

const process = require('process');
const neoapi = require('/usr/lib/neoapi');
const pretty = require('/usr/lib/pretty');
const { parseAndRun } = require('/usr/lib/opts');

const optionHelp = { type: 'boolean', short: 'h', description: 'Show this help message', default: false }

const defaultConfig = {
    usage: 'Usage: timer <command> [options]',
    options: {
        help: optionHelp,
    }
};

const listConfig = {
    func: doList,
    command: 'list',
    usage: 'timer list',
    description: 'List all registered timers',
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    }
}

const addConfig = {
    func: doAdd,
    command: 'add',
    usage: 'timer add [options] <name> <spec> <tql-path>',
    description: 'Add a new timer',
    options: {
        help: optionHelp,
        autoStart: { type: 'boolean', description: 'Enable autostart for the timer', default: false },
    },
    positionals: [
        { name: 'name', description: 'Name of the timer' },
        { name: 'spec', description: 'Timer specification in cron format' },
        { name: 'tql-path', description: 'Path to the TQL file to execute' },
    ],
    longDescription: `
    ex)
        timer add --auto-start my_sched '@every 10s' /hello.tql
    `,
}

const delConfig = {
    func: doDel,
    command: 'del',
    usage: 'timer del <name>',
    description: 'Delete an existing timer',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'name', description: 'Name of the timer to delete' },
    ],
}

const startConfig = {
    func: doStart,
    command: 'start',
    usage: 'timer start <name>',
    description: 'Start a timer',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'name', description: 'Name of the timer to start' },
    ],
}

const stopConfig = {
    func: doStop,
    command: 'stop',
    usage: 'timer stop <name>',
    description: 'Stop a timer',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'name', description: 'Name of the timer to stop' },
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
    client.listSchedules()
        .then((lst) => {
            let box = pretty.Table(config);
            box.appendHeader(["NAME", "SPEC", "TQL", "AUTOSTART", "STATE"]);
            for (const timer of lst) {
                if (timer.type !== 'TIMER') {
                    continue;
                }
                box.append([
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
    const autoStart = args.autoStart || false;
    client.addSchedule({ name: name, type: 'TIMER', spec: spec, task: tqlPath, autoStart: autoStart })
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
    client.deleteSchedule(args.name)
        .then(() => {
            console.println(`Timer '${args.name}' deleted successfully.`);
        })
        .catch((err) => {
            console.println('Error deleting timer:', err.message);
        });
}

function doStart(config, args) {
    const client = new neoapi.Client();
    client.startSchedule(args.name)
        .then(() => {
            console.println(`Timer '${args.name}' started successfully.`);
        })
        .catch((err) => {
            console.println('Error starting timer:', err.message);
        });
}

function doStop(config, args) {
    const client = new neoapi.Client();
    client.stopSchedule(args.name)
        .then(() => {
            console.println(`Timer '${args.name}' stopped successfully.`);
        })
        .catch((err) => {
            console.println('Error stopping timer:', err.message);
        });
}