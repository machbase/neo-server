'use strict';

const process = require('process');
const neoapi = require('/usr/lib/neoapi');
const pretty = require('/usr/lib/pretty');
const { parseAndRun } = require('/usr/lib/opts');

const optionHelp = { type: 'boolean', short: 'h', description: 'Show this help message', default: false }

const defaultConfig = {
    usage: 'Usage: ssh-key <command> [options]',
    options: {
        help: optionHelp,
    }
};

const listConfig = {
    func: doList,
    command: 'list',
    usage: 'ssh-key list',
    description: 'List all registered ssh keys',
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    }
}

const addConfig = {
    func: doAdd,
    command: 'add',
    usage: 'ssh-key add <type> <key> [comment]',
    description: 'Add a new ssh key',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'type', description: 'Type of the ssh key (e.g., rsa, dsa, ecdsa, ed25519)' },
        { name: 'key', description: 'The public key string' },
        { name: 'comment', variadic: true, description: 'A comment for the key (e.g., email or identifier)' },
    ],
}

const delConfig = {
    func: doDel,
    command: 'del',
    usage: 'ssh-key del <fingerprint>',
    description: 'Delete an existing ssh key',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'fingerprint', description: 'The fingerprint of the ssh key to delete' },
    ],
}

parseAndRun(process.argv.slice(2), defaultConfig, [
    listConfig,
    addConfig,
    delConfig,
]);

function doList(config, args) {
    const client = new neoapi.Client(config);
    client.listSSHKeys()
        .then((rows) => {
            let box = pretty.Table(config);
            box.appendHeader(["NAME", "KEY TYPE", "FIGERPRINT"]);
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
