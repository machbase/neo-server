'use strict';

const tableOptions = require('help/table_options');

const helpOption = { type: 'boolean', short: 'h', description: 'Show this help message', default: false };

function command(usage, description, config = {}) {
    const selectedTableOptions = { ...tableOptions };
    if (Array.isArray(config.table)) {
        for (const name of config.table) {
            delete selectedTableOptions[name];
        }
    }
    const result = {
        usage,
        description,
        options: {
            help: helpOption,
            ...(config.table ? selectedTableOptions : {}),
            ...(config.options || {}),
        },
    };
    for (const key of ['allowNegative', 'allowPositionals', 'longDescription', 'positionals', 'strict']) {
        if (config[key] !== undefined) {
            result[key] = config[key];
        }
    }
    return result;
}

function root(name, description, usage, commands) {
    return {
        name,
        description,
        usage,
        options: { help: helpOption },
        commands,
    };
}

function simple(name, description, usage, config = {}) {
    return { name, ...command(usage, description, config) };
}

module.exports = { command, helpOption, root, simple, tableOptions };