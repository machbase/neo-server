'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
}
const positionals = [
    { name: 'sec', type: 'integer', variadic: true, description: 'Number of seconds to sleep' }
];

let showHelp = true;
let config = {};
let args = {};
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
        positionals,
        allowPositionals: true,
    });
    config = parsed.values;
    args = parsed.namedPositionals;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp) {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: sleep [options] <sec...>',
        options,
        positionals,
    }));
    process.exit(showHelp ? 0 : 1);
}

setTimeout(() => {
    process.exit(0);
}, args.sec * 1000);