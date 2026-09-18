'use strict';

const { command, root } = require('help/config');

const bridgeTypes = `
  Bridge types (-t, --type for 'add' command):
    sqlite        SQLite            https://sqlite.org
        ex) bridge add -t sqlite my_memory file::memory:?cache=shared
            bridge add -t sqlite my_sqlite file:/tmp/sqlitefile.db
    postgres      PostgreSQL        https://postgresql.org
        ex) bridge add -t postgres my_pg "host=127.0.0.1 port=5432 user=dbuser dbname=postgres sslmode=disable"
    mysql         MySQL             https://mysql.com
        ex) bridge add -t mysql my_sql "root:passwd@tcp(127.0.0.1:3306)/testdb?parseTime=true"
    mqtt          MQTT (v3.1.1)     https://mqtt.org
        ex) bridge add -t mqtt my_mqtt "broker=127.0.0.1:1883 id=client-id"
    nats          NATS              https://nats.io
        ex) bridge add -t nats my_nats "server=nats://127.0.0.1:3000 name=client-name"
`;

module.exports = root('bridge', 'Manage bridges', 'Usage: bridge <command> [options]', {
    list: command('bridge list', 'Show registered bridges', { table: ['timeformat'] }),
    add: command('bridge add <name> <connection>', 'Add a new bridge', {
        table: ['timeformat'],
        options: { type: { type: 'string', short: 't', description: 'Bridge type [sqlite|postgres|mysql|mssql|mqtt|nats]' } },
        positionals: [
            { name: 'name', description: 'Name of the bridge' },
            { name: 'connection', variadic: true, description: 'Connection string' },
        ],
        longDescription: bridgeTypes,
    }),
    del: command('bridge del <name>', 'Remove a bridge', {
        table: ['timeformat'],
        positionals: [{ name: 'name', description: 'Name of the bridge to remove' }],
    }),
    test: command('bridge test <name>', 'Test connectivity of a bridge', {
        table: ['timeformat'],
        positionals: [{ name: 'name', description: 'Name of the bridge to test' }],
    }),
    stats: command('bridge stats <name>', 'Show bridge statistics', {
        table: ['timeformat'],
        positionals: [{ name: 'name', description: 'Name of the bridge' }],
    }),
    exec: command('bridge exec <name> <command>', 'Execute command on the bridge', {
        table: ['timeformat'],
        positionals: [
            { name: 'name', description: 'Name of the bridge' },
            { name: 'command', variadic: true, description: 'Command to execute' },
        ],
    }),
    query: command('bridge query <name> <command>', 'Query the bridge with command', {
        table: ['timeformat'],
        positionals: [
            { name: 'name', description: 'Name of the bridge' },
            { name: 'command', variadic: true, description: 'Query command' },
        ],
    }),
});