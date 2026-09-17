'use strict';

const _pretty = require('@jsh/pretty');
const TableArgOptions = require('help/table_options');

const defaultTableConfig = {
    header: true,
    footer: true,
    boxStyle: 'light',
    timeformat: 'default',
    binaryformat: 'preview',
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