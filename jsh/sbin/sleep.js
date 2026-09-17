'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const help = require('help');
const metadata = require('/usr/share/help/jsh/sleep');

let showHelp = true;
let config = {};
let args = {};
try {
    const parsed = parseArgs(process.argv.slice(2), metadata);
    config = parsed.values;
    args = parsed.namedPositionals;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp) {
    console.println(help.format(metadata));
    process.exit(showHelp ? 0 : 1);
}

setTimeout(() => {
    process.exit(0);
}, args.sec * 1000);