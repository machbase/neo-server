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
    { name: 'bridge', description: 'Manage bridges' },
    { name: 'connect', description: 'Connect to a database' },
    { name: 'explain', description: 'Explain a query plan' },
    { name: 'import', description: 'Import data into a table' },
    { name: 'key', description: 'Manage X.509 keys and auth-tokens' },
    { name: 'ping', description: 'Ping a database server' },
    { name: 'run', description: 'Run a script file' },
    { name: 'session', description: 'Manage sessions' },
    { name: 'show', description: 'Show database objects' },
    { name: 'shutdown', description: 'Shutdown the database server' },
    { name: 'sql', description: 'Execute an SQL command' },
    { name: 'ssh-key', description: 'Manage SSH keys' },
    { name: 'subscriber', description: 'Manage subscribers' },
    { name: 'timer', description: 'Manage database tables' },
];

if ((!objectName) || objectName.length === 0) {
    console.println('\nUse "help <object|command>;" to get more information.');
    console.println();
    console.println('Available objects:');
    let maxObjLength = 0;
    for (const obj of helpObjects) {
        if (obj.name.length > maxObjLength) {
            maxObjLength = obj.name.length;
        }
    }
    for (const obj of helpObjects) {
        console.println(`  ${obj.name.padEnd(maxObjLength)}  ${obj.description}`);
    }
    console.println();
    console.println('Available commands:');
    let maxCmdLength = 0;
    for (const cmd of helpCommands) {
        if (cmd.name.length > maxCmdLength) {
            maxCmdLength = cmd.name.length;
        }
    }
    for (const cmd of helpCommands) {
        console.println(`  ${cmd.name.padEnd(maxCmdLength)}  ${cmd.description}`);
    }
} else {
    if (objectName === 'timeformat') {
        helpTimeformat();
    } else if (objectName === 'tz') {
        helpTz();
    } else {
        process.exec(objectName, '-h');
    }
}

function helpTz() {
    console.println(`    abbreviations
      UTC
      Local
      Europe/London
      America/New_York
      ...
    location examples
      America/Los_Angeles
      Europe/Paris
      ...
    Time Coordinates examples
      UTC+9
`);
}

function helpTimeformat() {
    console.println(`    epoch
      ns             nanoseconds
      us             microseconds
      ms             milliseconds
      s              seconds
      s_ns           sec.nanoseconds
      s_us           sec.microseconds
      s_ms           sec.milliseconds
      s.ns           sec.nanoseconds (zero padding)
      s.us           sec.microseconds (zero padding)
      s.ms           sec.milliseconds (zero padding)
    abbreviations
      Default,-      2006-01-02 15:04:05.999
      Default_ms     2006-01-02 15:04:05.999
      Default_us     2006-01-02 15:04:05.999999
      Default_ns     2006-01-02 15:04:05.999999999
      Default.ms     2006-01-02 15:04:05.000
      Default.us     2006-01-02 15:04:05.000000
      Default.ns     2006-01-02 15:04:05.000000000
      Numeric        01/02 03:04:05PM '06 -0700
      Ansic          Mon Jan _2 15:04:05 2006
      Unix           Mon Jan _2 15:04:05 MST 2006
      Ruby           Mon Jan 02 15:04:05 -0700 2006
      RFC822         02 Jan 06 15:04 MST
      RFC822Z        02 Jan 06 15:04 -0700
      RFC850         Monday, 02-Jan-06 15:04:05 MST
      RFC1123        Mon, 02 Jan 2006 15:04:05 MST
      RFC1123Z       Mon, 02 Jan 2006 15:04:05 -0700
      RFC3339        2006-01-02T15:04:05Z07:00
      RFC3339Nano    2006-01-02T15:04:05.999999999Z07:00
      Kitchen        3:04:05PM
      Stamp          Jan _2 15:04:05
      StampMilli     Jan _2 15:04:05.000
      StampMicro     Jan _2 15:04:05.000000
      StampNano      Jan _2 15:04:05.000000000
    custom format
      year           2006
      month          01
      day            02
      hour           03 or 15
      minute         04
      second         05 or with sub-seconds '05.999999'
`);
}