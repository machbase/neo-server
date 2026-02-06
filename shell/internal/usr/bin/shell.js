'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const neoapi = require('/usr/lib/neoapi');
const pretty = require('/usr/lib/pretty');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    ...pretty.TableArgOptions,
}
const positionals = [
    { name: 'command', type: 'string', description: 'Command to execute' },
    { name: 'params', type: 'string', variadic: true, description: 'Arguments for the command' }
];

let showHelp = true;
let config = {};
let args = {};
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
        allowPositionals: true,
        allowNegative: true,
        positionals: positionals
    });
    config = parsed.values;
    args = parsed.namedPositionals;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

const commands = {
    'list': {
        syntax: 'list',
        description: 'List all shells',
        func: listShells
    },
    'add': {
        syntax: 'add <name> <bin_path> [args...]',
        description: 'Add a new shell with given name and binary path',
        func: addShell
    },
    'del': {
        syntax: 'del <id>',
        description: 'Delete a shell by ID',
        func: deleteShell
    },
}

function printHelp() {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: shell [options] <command> [params]',
        options,
        positionals: positionals
    }));
    console.println('\nAvailable commands:');
    for (const [cmd, info] of Object.entries(commands)) {
        console.println(`  ${info.syntax.padEnd(30)} ${info.description}`);
    }
}

if (showHelp || (!args.command) || args.command.length === 0) {
    printHelp();
    process.exit(showHelp ? 0 : 1);
}

args.command = args.command.toLowerCase();

// Validate that the provided command is in the allowed list
if (!commands.hasOwnProperty(args.command)) {
    console.println(`Error: Unknown command '${args.command}'\n`);
    printHelp();
    process.exit(1);
}

// Dispatch to appropriate handler based on command type
commands[args.command].func(config, args.params);

function listShells(config, args) {
    const client = new neoapi.Client(config);
    client.getShellList()
        .then((lst) => {
            let box = pretty.Table(config);
            box.appendHeader(['ID', 'NAME', 'COMMAND']);
            for (const shell of lst) {
                box.append([shell.id, shell.label, shell.command]);
            }
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function addShell(config, args) {
     const client = new neoapi.Client(config);
    if (args.length < 2) {
        console.println('Error: Missing parameters. Usage: add <name> <bin_path> [args...]');
        return;
    }
    const label = args[0];
    const command = args.slice(1).join(' ');
    client.addShell(label, command)
        .then((res) => {
            console.println(`Shell added with ID: ${res}`);
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function deleteShell(config, args) {
    const client = new neoapi.Client(config);
    if (args.length < 1) {
        console.println('Error: Missing parameter. Usage: del <id>');
        return;
    }
    const id = args[0];
    client.deleteShell(id)
        .then((res) => {
            console.println(`Shell deleted`);
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}