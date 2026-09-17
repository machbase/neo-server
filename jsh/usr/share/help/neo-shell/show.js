'use strict';

const tableOptions = require('help/table_options');
const helpOption = { type: 'boolean', short: 'h', description: 'Show this help message', default: false };
const clausePositionals = [
    { name: 'clause', variadic: true, optional: true, description: 'SHOW clauses' },
];

function command(usage, description, extraOptions, positionals) {
    return {
        usage,
        description,
        allowNegative: true,
        options: { help: helpOption, ...(extraOptions || {}), ...tableOptions },
        positionals: positionals || [],
    };
}

module.exports = {
    name: 'show',
    description: 'Show database objects',
    usage: 'Usage: show <command> [options]',
    options: { help: helpOption },
    commands: {
        info: command('show info', 'Display server information'),
        license: command('show license', 'Display license information'),
        ports: command('show ports [service]', 'Display service ports configuration', null, [
            { name: 'service', optional: true, description: 'Service name to filter' },
        ]),
        users: command('show users', 'List all database users', null, clausePositionals),
        databases: command('show databases', 'List all databases', null, clausePositionals),
        tables: command('show tables [-a] [FROM <db>[.<user>]] [LIKE <pattern>] [WITH ALL]', 'List tables', {
            all: { type: 'boolean', short: 'a', description: 'Show all hidden tables', default: false },
        }, clausePositionals),
        table: command('show table [-a] <table>', 'Show table schema and details', {
            all: { type: 'boolean', short: 'a', description: 'Show all hidden columns', default: false },
        }, [{ name: 'table', description: 'Table name' }]),
        'meta-tables': command('show meta-tables', 'List meta/system tables', null, clausePositionals),
        'virtual-tables': command('show virtual-tables', 'List virtual tables', null, clausePositionals),
        sessions: command('show sessions', 'List active database sessions', null, clausePositionals),
        statements: command('show statements', 'List currently running SQL statements', {
            long: { type: 'boolean', short: 'l', description: 'Show full SQL statements', default: false },
        }, clausePositionals),
        indexes: command('show indexes [FROM <db>[.<user>]] [LIKE <pattern>]', 'List all indexes', null, clausePositionals),
        index: command('show index <index>', 'Show index structure and details', null, [
            { name: 'index', description: 'Index name' },
        ]),
        storage: command('show storage [FROM <db>[.<user>]] [LIKE <pattern>]', 'Show storage statistics', null, clausePositionals),
        'table-usage': command('show table-usage [FROM <db>[.<user>]] [LIKE <pattern>]', 'Show storage usage by table', null, clausePositionals),
        lsm: command('show lsm [FROM <db>[.<user>]] [LIKE <pattern>]', 'Show LSM index status', null, clausePositionals),
        indexgap: command('show indexgap [FROM <db>[.<user>]] [LIKE <pattern>]', 'Show index gap information', null, clausePositionals),
        rollupgap: command('show rollupgap [FROM <db>[.<user>]] [LIKE <pattern>]', 'Show rollup gap information', {
            long: { type: 'boolean', short: 'l', description: 'Show running state', default: false },
        }, clausePositionals),
        tagindexgap: command('show tagindexgap [FROM <db>[.<user>]] [LIKE <pattern>]', 'Show tag index gap information', null, clausePositionals),
        tags: command('show tags <table> [tag...]', 'List all/specific tags in the specified table', null, [
            { name: 'table', description: 'Table name' },
            { name: 'tag', variadic: true, optional: true, description: 'Tag names' },
        ]),
        tagstat: command('show tagstat <table> [tag...]', 'Show statistics for the specific tags', null, [
            { name: 'table', description: 'Table name' },
            { name: 'tag', variadic: true, optional: true, description: 'Tag names' },
        ]),
    },
};