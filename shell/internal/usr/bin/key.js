'use strict';

const process = require('process');
const neoapi = require('/usr/lib/neoapi');
const pretty = require('/usr/lib/pretty');
const { parseAndRun } = require('/usr/lib/opts');

const optionHelp = { type: 'boolean', short: 'h', description: 'Show this help message', default: false }

const defaultConfig = {
    usage: 'Usage: key <command> [options]',
    options: {
        help: optionHelp,
    }
};

const listConfig = {
    func: doList,
    command: 'list',
    usage: 'key list',
    description: 'List all registered keys',
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    }
}

const genConfig = {
    func: doGen,
    command: 'gen',
    usage: 'key gen [options]',
    description: 'Generate new key with the given id',
    options: {
        help: optionHelp,
        output: { type: 'string', short: "o", description: 'Output file for the new key and token files', default: '-' },
    },
    positionals: [
        { name: 'id', description: 'The identifier for the new key' },
    ],
}

const delConfig = {
    func: doDel,
    command: 'del',
    usage: 'key del <id>',
    description: 'Delete an existing key',
    options: {
        help: optionHelp,
    },
    positionals: [
        { name: 'id', description: 'The identifier for the key to delete' },
    ],
}

const serverCertConfig = {
    func: doServerCert,
    command: 'server-cert',
    usage: 'key server-cert',
    description: 'Retrieve server certificate',
    options: {
        help: optionHelp,
        output: { type: 'string', short: "o", description: 'Output file for the server certificate', default: '-' },
    },
}

parseAndRun(process.argv.slice(2), defaultConfig, [
    listConfig,
    genConfig,
    delConfig,
    serverCertConfig,
]);

function doList(config, args) {
    const client = new neoapi.Client(config);
    client.listKeys()
        .then((keys) => {
            let box = pretty.Table(config);
            box.appendHeader(["ID", "NOT VALID BEFORE", "NOT VALID AFTER"]);
            for (const key of keys) {
                const nb = new Date(key.notBefore * 1000);
                const na = new Date(key.notAfter * 1000);
                box.append([key.id, nb, na]);
            }
            console.println(box.render());
        })
}

function doGen(config, args) {
    const name = args.id;
    // check if name is match with /^[a-zA-Z][a-zA-Z0-9_.@-]+$/
    if (!/^[a-zA-Z][a-zA-Z0-9_.@-]+$/.test(name)) {
        console.println('Invalid key id. It must start with a letter and contain only letters, digits, underscores, dots, at signs, or hyphens.');
        return;
    }
    const output = config.output;

    const client = new neoapi.Client(config);
    client.genKey(name)
        .then(({ certificate, key, token }) => {
            if (output && output !== '-' && output !== '') {
                const fs = require('fs');
                const path = require('path');
                const basePath = path.resolve(output);
                const certPath = `${basePath}_cert.pem`;
                const keyPath = `${basePath}_key.pem`;
                const tokenPath = `${basePath}_token.txt`;

                fs.mkdirSync(basePath);
                fs.writeFileSync(certPath, certificate);
                fs.writeFileSync(keyPath, key);
                fs.writeFileSync(tokenPath, token);

                console.println(`Key generated successfully.`);
                console.println(`Save certificate: ${certPath}`);
                console.println(`Save private Key: ${keyPath}`);
                console.println(`Save token: ${tokenPath}`);
                return;
            } else {
                console.println(certificate);
                console.println(key);
                console.println('-----BEGIN TOKEN-----');
                console.println(token);
                console.println('-----END TOKEN-----');
                console.println('\nCaution:\n  This is the last chance to copy and store PRIVATE KEY and TOKEN.');
                console.println('  It will not be shown again.\n');
            }
        })
        .catch((err) => {
            let message = err.message;
            //trim 'JSON-RPC error: ' prefix if exists
            if (message.startsWith('JSON-RPC error: ')) {
                message = message.substring('JSON-RPC error: '.length);
            }
            console.println('Error generating key:', message);
        });
}

function doDel(config, args) {
    const name = args.id;
    const client = new neoapi.Client(config);
    client.deleteKey(name)
        .then(() => {
            console.println('Key deleted successfully.');
        })
        .catch((err) => {
            console.println('Error deleting key:', err.message);
        });
}

function doServerCert(config, args) {
    const client = new neoapi.Client(config);
    client.getServerCertificate()
        .then((certificate) => {
            const output = config.output;
            if (output && output !== '-' && output !== '') {
                const fs = require('fs');
                const path = require('path');
                const certPath = path.resolve(output);
                fs.writeFileSync(certPath, certificate);
                console.println(`Server certificate saved to ${certPath}`);
            } else {
                console.println(certificate);
            }
        })
        .catch((err) => {
            console.println('Error retrieving server certificate:', err.message);
        });
}