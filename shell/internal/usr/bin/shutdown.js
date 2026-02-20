'use strict';

const process = require('process');
const neoapi = require('/usr/lib/neoapi');
const parseArgs = require('util/parseArgs');

const optionHelp = { type: 'boolean', short: 'h', description: 'Show this help message', default: false }

const defaultConfig = {
    usage: 'Usage: shutdown [options]',
    options: {
        help: optionHelp,
    }
};

let showHelp = false;
let config = {};
let argv = process.argv.slice(2);

try {
    const parsed = parseArgs(argv, defaultConfig);
    config = parsed.values;
    showHelp = config.help;
}
catch (err) {
    console.println(err.message);
    showHelp = true;
}

function printHelp() {
    if (argv.length > 0) {
        const cmd = argv[0].toLowerCase();
        for (const c of configs) {
            if (c.command.toLowerCase() === cmd) {
                const help = parseArgs.formatHelp(
                    c
                );
                console.println(help);
                return;
            }
        }
    }
    const help = parseArgs.formatHelp(defaultConfig, ...configs);
    console.println(help);
}

if (showHelp) {
    printHelp();
    process.exit(0);
}

const client = new neoapi.Client();
client.shutdownServer()
    .then(() => {
        console.println('Shutdown command sent successfully.');
    })
    .catch((err) => {
        console.println('Error sending shutdown command:', err.message);
    });