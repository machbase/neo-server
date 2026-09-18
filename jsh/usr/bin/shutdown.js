'use strict';

const process = require('process');
const neoapi = require('/usr/lib/neoapi');
const parseArgs = require('util/parseArgs');
const help = require('/usr/share/help/neo-shell/shutdown');

let showHelp = false;
let config = {};
let argv = process.argv.slice(2);

try {
    const parsed = parseArgs(argv, help);
    config = parsed.values;
    showHelp = config.help;
}
catch (err) {
    console.println(err.message);
    showHelp = true;
}

function printHelp() {
    console.println(parseArgs.formatHelp(help));
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