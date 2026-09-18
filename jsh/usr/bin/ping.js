'use strict';

const process = require('process');
const pretty = require('pretty');
const parseArgs = require('util/parseArgs');
const { newMachCliClient } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/ping');

let showHelp = true;
let config = {};
let args = {};
try {
    const parsed = parseArgs(process.argv.slice(2), help);
    config = parsed.values;
    args = parsed.namedPositionals;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp) {
    console.println(parseArgs.formatHelp(help));
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

        db = newMachCliClient(config);
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