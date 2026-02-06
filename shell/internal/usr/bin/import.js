'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const machcli = require('/usr/lib/machcli');
const pretty = require('/usr/lib/pretty');
const fs = require('fs');
const parser = require('parser');
const zlib = require('zlib');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    input: { type: 'string', short: 'i', description: "input file (default:'-' stdin)", default: '-' },
    compress: { type: 'string', description: "compression type (none, gzip)", default: 'none' },
    format: { type: 'string', short: 'f', description: "input format (csv, tsv, ndjson)", default: 'csv' },
    timeformat: { type: 'string', short: 't', description: "time format [ns|us|ms|s|<timeformat>]", default: 'ns' },
    tz: { type: 'string', description: "time zone for handling datetime (default: time zone)", default: 'local' },
    header: { type: 'string', description: "header option [skip|columns|none]", default: 'none' },
    nullValue: { type: 'string', description: "string to represent null values", default: 'NULL' },
    dryRun: { type: 'boolean', description: "run in dry mode", default: false },
}

const positionals = [
    { name: 'table', type: 'string', description: 'table name to read' }
];

let showHelp = true;
let config = {};
let tableName = '';
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
        allowPositionals: true,
        allowNegative: true,
        positionals: positionals
    });
    config = parsed.values;
    tableName = parsed.namedPositionals.table;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp || (!tableName) || tableName.length === 0) {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: import [options] <table>',
        options,
        positionals: positionals
    }));
    process.exit(showHelp ? 0 : 1);
}

const db = new machcli.Client(config);
const conn = db.connect();
let appender = conn.append(tableName);
const colDefs = appender.columns();

let columnNames = [];
let columnTypes = [];
for (let i = 0; i < colDefs.length; i++) {
    let col = colDefs[i];
    // skip system columns like _RID
    if (col.Name === '_RID') continue;
    columnNames.push(col.name);
    columnTypes.push(col.type.toString());
}

const csvParser = parser.csv({
    separator: config.format === 'tsv' ? '\t' : ',',
})


switch (config.header) {
    case 'columns':
        csvParser.headers = true;
        break;
    case 'skip':
        csvParser.headers = true;
        break;
    case 'none':
        csvParser.headers = columnNames;
        break;
    default:
        throw new Error(`Invalid header option: ${config.header}`);
}

let nRows = 0;
let totalLines = fs.countLines(config.input);

const tracker = pretty.Progress({ showPercentage: true }).tracker({
    label: `Importing ${config.input} into ${tableName}`,
    total: totalLines,
})

const onHeader = (headers) => {
    tracker.increment(1);
    if (config.header !== 'columns') {
        // skip header row, do nothing
        return;
    }
    columnNames = [];
    columnTypes = [];
    for (let i = 0; i < headers.length; i++) {
        let found = -1;
        for (let j = 0; j < colDefs.length; j++) {
            if (headers[i].toUpperCase() === colDefs[j].Name) {
                found = j;
                break;
            }
        }
        if (found === -1) {
            throw new Error(`Column '${headers[i]}' not found in table '${tableName}'`);
        } else {
            columnNames.push(colDefs[found].name);
            columnTypes.push(colDefs[found].type.toString());
        }
    }
}

appender = appender.withInputColumns(...columnNames);

fs.createReadStream(config.input, { highWaterMark: 1024 })
    .pipe(csvParser)
    .on('headers', onHeader)
    .on('data', (row) => {
        nRows++;
        tracker.increment(1);
        let rec = [];
        for (let i = 0; i < columnNames.length; i++) {
            let colName = columnNames[i];
            let colType = columnTypes[i];
            let value = row[colName];
            switch (colType) {
                case 'datetime':
                    value = pretty.parseTime(value, config.timeformat, config.tz);
                    break;
                case "double":
                    value = parseFloat(value);
                    break;
            }
            if (value === config.nullValue) {
                rec.push(null);
                continue;
            } else {
                rec.push(value);
            }
        }
        if (!config.dryRun) {
            appender.append(...rec);
        }
    })
    .on('error', (err) => {
        console.println(`Error during import: ${err.message}`);
        tracker.markAsErrored();
        conn && conn.close();
        db && db.close();
        process.exit(1);
    })
    .on('end', () => {
        let result = appender.close();
        if (tracker.value() < totalLines) {
            tracker.setValue(totalLines);
        }
        tracker.markAsDone();
        setTimeout(() => {
            if (config.dryRun) {
                console.println(`Import ${pretty.Ints(nRows)} rows dry run completed.`);
            } else {
                console.println(`Import ${pretty.Ints(nRows)} rows completed. ${result}`);
            }
        }, 100);
        conn && conn.close();
        db && db.close();
    });

