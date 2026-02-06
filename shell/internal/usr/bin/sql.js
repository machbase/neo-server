'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const { Client } = require('/usr/lib/machcli');
const pretty = require('/usr/lib/pretty');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    output: { type: 'string', short: 'o', description: "output file (default:'-' stdout)", default: '-' },
    compress: { type: 'string', description: "compression type (none, gzip)", default: 'none' },
    timing: { type: 'boolean', short: 'T', description: "print elapsed time", default: false },
    showTz: { type: 'boolean', short: 'Z', description: "show time zone in datetime column header", default: false },
    progress: { type: 'integer', description: "the expected maximum progress value (0: unknown, -1: disable)", default: 0 },
    ...pretty.TableArgOptions,
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
        usage: 'Usage: sql [options] <sql>',
        options,
        positionals: positionals
    }));
    process.exit(showHelp ? 0 : 1);
}

const sqlText = args.sql.join(' ');
let db, conn, rows;
try {
    db = new Client(config);
    conn = db.connect();
    rows = conn.query(sqlText);

    let tick = process.now();
    let box = pretty.Table(config);
    let writer = null;
    let gzip = null;
    let nRows = 0;
    let tracker = null;

    if (config.output === '' || config.output === '-') {
        box.setOutput(console);
    } else {
        const fs = require('fs');
        const path = require('path');
        const outputPath = path.resolve(config.output);
        writer = fs.createWriteStream(outputPath, { encoding: 'utf8' });
        if (config.compress === 'gzip') {
            const zlib = require('zlib');
            gzip = zlib.createGzip();
            gzip.pipe(writer);
            box.setOutput(gzip);
        } else {
            box.setOutput(writer);
        }
        if (config.progress >= 0) {
            let pw = pretty.Progress({ showPercentage: config.progress > 0 });
            tracker = pw.tracker({
                total: config.progress,
                message: `Writing to ${outputPath}`,
            });
        }
        // disable pause for file output
        box.setPause(false);
    }

    if (config.showTz) {
        let columnLabels = [];
        for (let i = 0; i < rows.columnTypes.length; i++) {
            if (rows.columnTypes[i] == 'datetime') {
                columnLabels.push(rows.columnNames[i] + `(${config.tz})`);
            } else {
                columnLabels.push(rows.columnNames[i])
            }
        }
        box.appendHeader(columnLabels);
    } else {
        box.appendHeader(rows.columnNames);
    }
    box.setColumnTypes(rows.columnTypes);

    for (const row of rows) {
        nRows += 1;
        tracker && tracker.setValue(nRows);
        // spread row values
        box.append([...row]);
        if (box.requirePageRender()) {
            // render page
            box.render();
            // wait for user input to continue if pause is enabled
            if (!box.pauseAndWait()) {
                break;
            }
        }
    }
    tracker && tracker.markAsDone();

    // render remaining rows
    box.close();
    if (gzip) {
        gzip.end();
    }
    if (writer) {
        writer.end();
    }
    // footer message
    let footMessage = '';
    if (config.footer) {
        footMessage += rows.message;
    }
    // print elapsed time
    if (config.timing) {
        footMessage += ` ${pretty.Durations(process.now().unixNano() - tick.unixNano())} elapsed.`;
    }
    if (config.footer || config.timing) {
        console.println(footMessage.trim());
    }
} catch (err) {
    console.println("Error: ", err.message);
} finally {
    rows && rows.close();
    conn && conn.close();
    db && db.close();
}
