'use strict';

const _pretty = require('@jsh/pretty');

const defaultTableConfig = {
    header: true,
    footer: true,
    boxStyle: 'light',
    timeformat: 'default',
    tz: 'local',
    precision: -1,
    format: 'box',
    rownum: true,
    nullValue: 'NULL',
    stringEscape: false,
}

function Table(config) {
    config = { ...defaultTableConfig, ...config };
    try {
        const box = _pretty.Table(config);
        return box;
    }
    catch (err) {
        throw err;
    }
}

const TableArgOptions = {
    format: { type: 'string', short: 'f', description: "output format (box, csv, tsv, json, ndjson)", default: 'box' },
    boxStyle: { type: 'string', description: "box style (simple, bold, double, light, round, colored-bright, colored-dark)", default: 'light' },
    rownum: { type: 'boolean', description: "show row numbers", default: true },
    timeformat: { type: 'string', short: 't', description: "time format [ns|us|ms|s|<timeformat>]", default: 'default' },
    tz: { type: 'string', description: "time zone for handling datetime (default: time zone)", default: 'local' },
    precision: { type: 'integer', short: 'p', description: "set precision of float value to force round", default: -1 },
    header: { type: 'boolean', description: "print header", default: true },
    footer: { type: 'boolean', description: "print footer", default: true },
    pause: { type: 'boolean', description: "pause for the screen paging", default: true },
    nullValue: { type: 'string', description: "string to represent null values", default: 'NULL' },
}

const Align = {
    default: 0,
    left: 1,
    center: 2,
    justify: 3,
    right: 4,
    auto: 5,
}
module.exports = {
    ..._pretty,
    Table,
    TableArgOptions,
    Align,
}