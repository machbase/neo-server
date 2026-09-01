'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');

const optionHelp = { type: 'boolean', short: 'h', description: 'Show this help message', default: false };

const listConfig = {
    func: doList,
    command: 'list',
    usage: 'token list',
    description: 'List your API tokens',
    options: { help: optionHelp, ...pretty.TableArgOptions },
};

const genConfig = {
    func: doGen,
    command: 'gen',
    usage: 'token gen <name> [--not-after <date>]',
    description: 'Generate an API token',
    options: {
        help: optionHelp,
        'not-after': { type: 'string', description: 'Expiration date in ISO-8601 format', default: '' },
    },
    positionals: [{ name: 'name', description: 'A label for the token' }],
};

const delConfig = {
    func: doDel,
    command: 'del',
    usage: 'token del <id>',
    description: 'Delete one of your API tokens',
    options: { help: optionHelp },
    positionals: [{ name: 'id', description: 'Token ID' }],
};

parseAndRun(process.argv.slice(2), { usage: 'Usage: token <command> [options]', options: { help: optionHelp } }, [listConfig, genConfig, delConfig]);

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