'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/ssh-key');

const commandFunctions = { list: doList, add: doAdd, del: doDel };
const commandConfigs = Object.keys(help.commands).map((name) => ({
    ...help.commands[name],
    command: name,
    func: commandFunctions[name],
}));

parseAndRun(process.argv.slice(2), help, commandConfigs);

function doList(config, args) {
    const client = new neoapi.Client(config);
    client.listSSHKeys()
        .then((rows) => {
            let box = pretty.Table(config);
            box.appendHeader(["NAME", "KEY TYPE", "FINGERPRINT"]);
            for (const row of rows) {
                box.append([row.Comment, row.KeyType, row.Fingerprint]);
            }
            console.println(box.render());
        })
}

function doAdd(config, args) {
    const keyType = args.type;
    const key = args.key;
    let comment = '';
    if (args.comment) {
        if (args.comment.length > 0)
            comment = args.comment.join(' ');
        else
            comment = args.comment;
    }
    const client = new neoapi.Client(config);
    client.addSSHKey(keyType, key, comment)
        .then(() => {
            console.println('SSH key added successfully.');
        })
        .catch((err) => {
            let message = err.message;
            //trim 'JSON-RPC error: ' prefix if exists
            if (message.startsWith('JSON-RPC error: ')) {
                message = message.substring('JSON-RPC error: '.length);
            }
            console.println('Error adding SSH key:', message);
        });
}

function doDel(config, args) {
    const fingerprint = args.fingerprint;
    const client = new neoapi.Client(config);
    client.deleteSSHKey(fingerprint)
        .then(() => {
            console.println('SSH key deleted successfully.');
        })
        .catch((err) => {
            console.println('Error deleting SSH key:', err.message);
        });
}
