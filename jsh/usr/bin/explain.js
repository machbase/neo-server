'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const { newMachCliClient } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/explain');

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

if (showHelp || (!args.sql) || args.sql.length === 0) {
    console.println(parseArgs.formatHelp(help));
    process.exit(showHelp ? 0 : 1);
}

const sqlText = args.sql.join(' ');
let db, conn;
try {
    db = newMachCliClient(config);
    conn = db.connect();
    let result = conn.explain(sqlText, config.full);
    console.println(result);
} catch (err) {
    console.println("Error: ", err.message);
} finally {
    conn && conn.close();
    db && db.close();
}
