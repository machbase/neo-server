'use strict';

const process = require('process');
const pretty = require('pretty');
const parseArgs = require('util/parseArgs');
const neoapi = require('/usr/lib/neoapi');
const viz = require('vizspec');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
}

const positionals = [
    { name: 'names', type: 'string', variadic: true, description: 'metric names to show' }
];

let showHelp = true;
let config = {};
let args = {};
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
        allowPositionals: true,
        allowNegative: true,
        positionals: positionals
    });
    config = parsed.values;
    args = parsed.namedPositionals;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp || (!args.names) || args.names.length == 0) {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: statz [options] <metric_name...>',
        options,
        positionals: positionals
    }));
    process.exit(showHelp ? 0 : 1);
}

const client = new neoapi.Client(config);
client.getServerStatz(...args.names)
    .then((rsp) => {
        for (const stat of rsp.statz) {
            console.println("[" + stat.name + "]");
            let spec = viz.createSpec(stat.spec);
            console.println(viz.toTUILines(spec, { width: 80, height: 5, rows: 60 }).join("\n"));
            console.println();
        }
    })
    .catch((err) => {
        console.println('Error:', err.message);
    });