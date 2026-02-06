'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const { Client } = require('/usr/lib/machcli');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    full: { type: 'boolean', short: 'f', description: 'Show full explain plan', default: false },
}
const positionals = [
    { name: 'sql', type: 'string', variadic: true, description: 'SQL query to explain' }
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

if (showHelp || (!args.sql) || args.sql.length === 0) {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: explain [options] <sql>',
        options,
        positionals: positionals
    }));
    process.exit(showHelp ? 0 : 1);
}

explain(config, args.sql.join(' '), config.full);

function explain(config, sqlText, full = false) {
    let db, conn;
    try {
        db = new Client(config);
        conn = db.connect();
        let result = conn.explain(sqlText, full);
        console.println(result);
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        conn && conn.close();
        db && db.close();
    }
}
