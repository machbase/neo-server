'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/shell');

const commandFunctions = { list: listShells, add: addShell, del: deleteShell };
const commandConfigs = Object.keys(help.commands).map((name) => ({
    ...help.commands[name],
    command: name,
    func: commandFunctions[name],
}));

parseAndRun(process.argv.slice(2), help, commandConfigs);

function listShells(config, args) {
    const client = new neoapi.Client(config);
    client.listShells()
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
    const label = args.name;
    const command = [args.binPath, ...(args.args || [])].join(' ');
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
    const id = args.id;
    client.deleteShell(id)
        .then((res) => {
            console.println(`Shell deleted`);
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}