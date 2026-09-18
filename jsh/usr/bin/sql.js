'use strict';

const process = require('process');
const pretty = require('pretty');
const parseArgs = require('util/parseArgs');
const { newMachCliClient } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/sql');

let showHelp = true;
let config = {};
let args = {};
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options: help.options,
        allowPositionals: true,
        allowNegative: true,
        positionals: help.positionals
    });
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
let db, conn, rows;
let tick = process.now();
let exitCode = 0;
try {
    db = newMachCliClient(config);
    conn = db.connect();
    rows = conn.query(sqlText);

    if (rows.isFetchable()) {
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
            const tzSuffix = config && config.tz ? `(${config.tz})` : '';
            for (let i = 0; i < (rows.columnTypes ? rows.columnTypes.length : 0); i++) {
                const columnName = rows && rows.columnNames && rows.columnNames[i] ? rows.columnNames[i] : '';
                const columnType = rows && rows.columnTypes ? rows.columnTypes[i] : null;
                // columnType is sql.ColumnType object
                if (columnType && typeof columnType.databaseTypeName === 'function' && columnType.databaseTypeName() === 'DATETIME') {
                    columnLabels.push(columnName ? `${columnName}${tzSuffix}` : `DATETIME${tzSuffix}`);
                } else {
                    columnLabels.push(columnName);
                }
            }
            box.appendHeader(columnLabels);
        } else {
            box.appendHeader(rows.columnNames || []);
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
    }
    // footer message
    let footMessage = '';
    if (config.footer) {
        footMessage += rows.message();
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
    exitCode = 1;
} finally {
    rows && rows.close();
    conn && conn.close();
    db && db.close();
}
if (exitCode !== 0) {
    process.exit(exitCode);
}