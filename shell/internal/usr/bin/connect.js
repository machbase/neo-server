'use strict';

const process = require('process');
const env = process.env;
const parseArgs = require('util/parseArgs');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
}

const positionals = [
    { name: 'connection', type: 'string', description: 'connection string to use' }
];

let showHelp = true;
let config = {};
let connection = null;
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
        allowPositionals: true,
        allowNegative: false,
        positionals: positionals
    });
    config = parsed.values;
    connection = parsed.namedPositionals.connection;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp) {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: connect [options] <connection_string>',
        options,
        positionals: positionals
    }));
    process.exit(showHelp ? 0 : 1);
}

const user = env.get('NEOSHELL_USER');
const password = env.get('NEOSHELL_PASSWORD');

env.set('NEOSHELL_USER', null);
env.set('NEOSHELL_PASSWORD', null);

if (connection && connection.length > 0) {
    // connection [user:password@]host[:port]
    const atIdx = connection.indexOf('@');
    if (atIdx > 0) {
        const authPart = connection.substring(0, atIdx);
        const hostPart = connection.substring(atIdx + 1);
        const colonIdx = authPart.indexOf(':');
        if (colonIdx > 0) {
            env.set('NEOSHELL_USER', authPart.substring(0, colonIdx));
            env.set('NEOSHELL_PASSWORD', authPart.substring(colonIdx + 1));
        } else {
            env.set('NEOSHELL_USER', authPart);
        }
        connection = hostPart;
    }
    if (connection && connection.length > 0) {
        env.set('NEOSHELL_HOST', connection);
    }
}

process.exec('neo-shell')

console.println("disconnected from neo-shell");
env.set('NEOSHELL_USER', user);
env.set('NEOSHELL_PASSWORD', password);