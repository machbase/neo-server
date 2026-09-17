'use strict';

module.exports = {
    format: { type: 'string', short: 'f', description: "output format (box, csv, tsv, json, ndjson)", default: 'box' },
    boxStyle: { type: 'string', description: "box style (simple, bold, double, light, round, bright, dark)", default: 'light' },
    rownum: { type: 'boolean', description: "show row numbers", default: true },
    timeformat: { type: 'string', short: 't', description: "time format [ns|us|ms|s|<timeformat>]", default: 'default' },
    binaryformat: { type: 'string', description: "binary format (base64, hex, bytes, preview)", default: 'preview' },
    tz: { type: 'string', description: "time zone for handling datetime (default: time zone)", default: 'local' },
    precision: { type: 'integer', description: "set precision of float value to force round", default: -1 },
    header: { type: 'boolean', description: "print header", default: true },
    footer: { type: 'boolean', description: "print footer", default: true },
    pause: { type: 'boolean', description: "pause for the screen paging", default: true },
    nullValue: { type: 'string', description: "string to represent null values", default: 'NULL' },
};