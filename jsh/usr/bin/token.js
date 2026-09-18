'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/token');

const commandFunctions = { list: doList, gen: doGen, del: doDel };
const commandConfigs = Object.keys(help.commands).map((name) => ({
    ...help.commands[name],
    command: name,
    func: commandFunctions[name],
}));

parseAndRun(process.argv.slice(2), help, commandConfigs);

function doList(config) {
    new neoapi.Client(config).listTokens()
        .then((tokens) => {
            const table = pretty.Table(config);
            table.appendHeader(['ID', 'NAME', 'USER', 'TOKEN', 'CREATED', 'EXPIRES', 'LAST USED']);
            for (const token of tokens) {
                table.append([token.id, token.name, token.user, token.hint, new Date(token.createdAt * 1000), new Date(token.notAfter * 1000), token.lastUsedAt ? new Date(token.lastUsedAt * 1000) : '']);
            }
            console.println(table.render());
        })
        .catch((err) => console.println('Error:', err.message));
}

function doGen(config, args) {
    let notAfter = 0;
    if (config.notAfter) {
        const date = new Date(config.notAfter);
        if (isNaN(date.getTime())) {
            console.println('Invalid expiration date. Use ISO-8601 format.');
            return;
        }
        notAfter = Math.floor(date.getTime() / 1000);
    }
    new neoapi.Client(config).generateToken(args.name, notAfter)
        .then((token) => console.println(token.token))
        .catch((err) => console.println('Error:', err.message));
}

function doDel(config, args) {
    new neoapi.Client(config).deleteToken(Number(args.id))
        .then(() => console.println('Token deleted successfully.'))
        .catch((err) => console.println('Error:', err.message));
}