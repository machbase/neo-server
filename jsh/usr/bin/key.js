'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/key');

const commandFunctions = {
    list: doList,
    gen: doGen,
    del: doDel,
    'server-cert': doServerCert,
};
const commandConfigs = Object.keys(help.commands).map((name) => ({
    ...help.commands[name],
    command: name,
    func: commandFunctions[name],
}));

parseAndRun(process.argv.slice(2), help, commandConfigs);

function doList(config, args) {
    const client = new neoapi.Client(config);
    client.listKeys()
        .then((keys) => {
            let box = pretty.Table(config);
            box.appendHeader(["ID", "NAME", "NOT VALID BEFORE", "NOT VALID AFTER"]);
            for (const key of keys) {
                const nb = new Date(key.notBefore * 1000);
                const na = new Date(key.notAfter * 1000);
                box.append([key.id, key.name, nb, na]);
            }
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function doGen(config, args) {
    const name = args.name;
    // check if name is match with /^[a-zA-Z][a-zA-Z0-9_.@-]+$/
    if (!/^[a-zA-Z][a-zA-Z0-9_.@-]+$/.test(name)) {
        console.println('Invalid key name. It must start with a letter and contain only letters, digits, underscores, dots, at signs, or hyphens.');
        return;
    }
    const output = config.output;
    const type = config.type.toLowerCase();
    const store = !!config.store
    const client = new neoapi.Client(config);
    client.genKey(name, type, 0, 0, store)
        .then(({ id, certificate, key }) => {
            if (output && output !== '-' && output !== '') {
                const fs = require('fs');
                const path = require('path');
                const basePath = path.resolve(output);
                const certPath = `${basePath}_cert.pem`;
                const keyPath = `${basePath}_key.pem`;
                fs.mkdirSync(basePath);
                fs.writeFileSync(certPath, certificate);
                fs.writeFileSync(keyPath, key);

                console.println(`Key generated successfully. id=${id}`);
                console.println(`Save certificate: ${certPath}`);
                console.println(`Save private Key: ${keyPath}`);
                return;
            } else {
                console.println(`id=${id}`);
                console.println(certificate);
                console.println(key);
                console.println('\nCaution:\n  This is the last chance to copy and store the PRIVATE KEY.');
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
    const id = Number(args.id);
    const client = new neoapi.Client(config);
    client.deleteKey(id)
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