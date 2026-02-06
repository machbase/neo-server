'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const { Client } = require('/usr/lib/machcli');
const pretty = require('/usr/lib/pretty');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    repeat: { type: 'integer', short: 'n', description: "Number of times to repeat the ping", default: '1' },
}

let showHelp = true;
let config = {};
let args = {};
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
        allowPositionals: true,
        allowNegative: true,
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
        usage: 'Usage: ping [options]',
        options,
    }));
    process.exit(showHelp ? 0 : 1);
}

const repeat = config.repeat;
let count = 0;

setTimeout(ping, 1000);

function ping() {
    count++;
    let db, conn, rows;
    try {
        let tick = process.now();

        db = new Client(config);
        conn = db.connect();
        rows = conn.query('SELECT EDITION FROM V$VERSION');
        if (rows.next()) {
            console.println(`seq=${count} time=${pretty.Durations(process.now().unixNano() - tick.unixNano())}`);
        } else {
            console.println("No response from server.");
        }
        if (count < repeat)
            setTimeout(ping, 1000);
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        rows && rows.close();
        conn && conn.close();
        db && db.close();
    }
}