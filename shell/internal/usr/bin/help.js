'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
}

const positionals = [
    { name: 'object', type: 'string', optional: true, description: 'The object to display help for' }
];

let showHelp = true;
let config = {};
let objectName = '';
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
        allowPositionals: true,
        allowNegative: true,
        positionals: positionals
    });
    config = parsed.values;
    objectName = parsed.namedPositionals.object;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp) {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: help <object>',
        options,
        positionals: positionals
    }));
    process.exit(showHelp ? 0 : 1);
}


const helpObjects = [
    { name: 'timeformat', description: 'Time formats' },
    { name: 'tz', description: 'Time zones' },
];

const helpCommands = [
    { name: 'connect', description: 'Connect to a database' },
    { name: 'sql', description: 'Execute an SQL command' },
];

if ((!objectName) || objectName.length === 0) {
    console.println('Available objects:');
    for (const obj of helpObjects) {
        console.println(`  ${obj.name} - ${obj.description}`);
    }
    console.println();
    console.println('Available commands:');
    for (const cmd of helpCommands) {
        console.println(`  ${cmd.name} - ${cmd.description}`);
    }
    console.println('\nUse "help <object|command>" to get more information.');
} else {
    if (objectName === 'timeformat') {
        console.println(`Help for object: ${objectName}`);
    } else if (objectName === 'tz') {
        console.println(`Help for object: ${objectName}`);
    } else {
        process.exec(objectName, '-h');
    }
}
