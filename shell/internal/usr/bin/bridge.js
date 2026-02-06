'use strict';

const process = require('process');
const neoapi = require('/usr/lib/neoapi');
const pretty = require('/usr/lib/pretty');
const { parseAndRun } = require('/usr/lib/opts');

// '-t' ('--type') is duplicated in pretty.TableArgOptions, so remove it here
const prettyTableOptions = delete pretty.TableArgOptions['timeformat'];

// Global options (available for all commands)
const globalOptions = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message' },
    ...prettyTableOptions,
};

// Sub-command configurations
const listConfig = {
    func: listBridges,
    command: 'list',
    usage: 'bridge list',
    description: 'Show registered bridges',
    options: {
        ...globalOptions
    }
};

const addConfig = {
    func: addBridge,
    command: 'add',
    usage: 'bridge add <name> <connection>',
    description: 'Add a new bridge',
    options: {
        ...globalOptions,
        type: { type: 'string', short: 't', description: 'Bridge type [sqlite|postgres|mysql|mssql|mqtt|nats]' }
    },
    positionals: [
        { name: 'name', description: 'Name of the bridge' },
        { name: 'connection', variadic: true, description: 'Connection string' }
    ],
    allowPositionals: true,
    longDescription: `
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
`
};

const delConfig = {
    func: delBridge,
    command: 'del',
    usage: 'bridge del <name>',
    description: 'Remove a bridge',
    options: {
        ...globalOptions
    },
    positionals: [
        { name: 'name', description: 'Name of the bridge to remove' }
    ],
    allowPositionals: true
};

const testConfig = {
    func: testBridge,
    command: 'test',
    usage: 'bridge test <name>',
    description: 'Test connectivity of a bridge',
    options: {
        ...globalOptions
    },
    positionals: [
        { name: 'name', description: 'Name of the bridge to test' }
    ],
    allowPositionals: true
};

const statsConfig = {
    func: statsBridge,
    command: 'stats',
    usage: 'bridge stats <name>',
    description: 'Show bridge statistics',
    options: {
        ...globalOptions
    },
    positionals: [
        { name: 'name', description: 'Name of the bridge' }
    ],
    allowPositionals: true
};

const execConfig = {
    func: execBridge,
    command: 'exec',
    usage: 'bridge exec <name> <command>',
    description: 'Execute command on the bridge',
    options: {
        ...globalOptions
    },
    positionals: [
        { name: 'name', description: 'Name of the bridge' },
        { name: 'command', variadic: true, description: 'Command to execute' }
    ],
    allowPositionals: true
};

const queryConfig = {
    func: queryBridge,
    command: 'query',
    usage: 'bridge query <name> <command>',
    description: 'Query the bridge with command',
    options: {
        ...globalOptions
    },
    positionals: [
        { name: 'name', description: 'Name of the bridge' },
        { name: 'command', variadic: true, description: 'Query command' }
    ],
    allowPositionals: true
};

const defaultConfig = {
    usage: 'Usage: bridge <command> [options]',
    options: {
        help: { type: 'boolean', short: 'h', description: 'Show this help message' }
    }
};

const commands = [
    listConfig,
    addConfig,
    delConfig,
    testConfig,
    statsConfig,
    execConfig,
    queryConfig,
];

parseAndRun(process.argv.slice(2), defaultConfig, commands);

function listBridges(config, args) {
    const client = new neoapi.Client(config);
    client.getBridgeList()
        .then((lst) => {
            let box = pretty.Table(config);
            box.appendHeader(['NAME', 'TYPE', 'CONNECTION']);
            for (const br of lst) {
                box.append([br.name, br.type, br.path]);
            }
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function addBridge(config, args) {
    if (!config.type) {
        console.println("Error: Missing bridge type. Use -t option to specify one of [sqlite, postgres, mysql, mssql, mqtt, nats]");
        process.exit(1);
    }
    if (['sqlite', 'postgres', 'mysql', 'mssql', 'mqtt', 'nats'].indexOf(config.type) < 0) {
        console.println("Error: Invalid bridge type. Use -t option to specify one of [sqlite, postgres, mysql, mssql, mqtt, nats]");
        process.exit(1);
    }
    if (!args.name) {
        console.println("Error: Missing bridge name.");
        process.exit(1);
    }
    if (!args.connection || args.connection.length === 0) {
        console.println("Error: Missing connection string.");
        process.exit(1);
    }

    const name = args.name;
    const connection = Array.isArray(args.connection) ? args.connection.join(' ') : args.connection;

    const client = new neoapi.Client(config);
    client.addBridge(name, config.type, connection)
        .then((result) => {
            console.println("Adding bridge...", name, "type:", config.type, "path:", connection);
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function delBridge(config, args) {
    if (!args.name) {
        console.println("Error: Missing bridge name.");
        process.exit(1);
    }
    const name = args.name;
    const client = new neoapi.Client(config);
    client.deleteBridge(name)
        .then((result) => {
            console.println("Deleting bridge...", name);
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function testBridge(config, args) {
    if (!args.name) {
        console.println("Error: Missing bridge name.");
        process.exit(1);
    }
    const name = args.name;
    const client = new neoapi.Client(config);
    client.testBridge(name)
        .then((result) => {
            console.println("Testing bridge...", name);
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function statsBridge(config, args) {
    if (!args.name) {
        console.println("Error: Missing bridge name.");
        process.exit(1);
    }
    const name = args.name;
    const client = new neoapi.Client(config);
    client.statsBridge(name)
        .then((result) => {
            let box = pretty.Table(config);
            box.appendHeader(['NAME', 'VALUE']);
            box.append(['In Messages', pretty.Ints(result.InMsgs)]);
            box.append(['Out Messages', pretty.Ints(result.OutMsgs)]);
            box.append(['In Bytes', pretty.Bytes(result.InBytes)]);
            box.append(['Out Bytes', pretty.Bytes(result.OutBytes)]);
            box.append(['Inserted Rows', pretty.Ints(result.Inserted)]);
            box.append(['Appended Rows', pretty.Ints(result.Appended)]);
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function execBridge(config, args) {
    if (!args.name) {
        console.println("Error: Missing bridge name.");
        process.exit(1);
    }
    if (!args.command || args.command.length === 0) {
        console.println("Error: Missing command.");
        process.exit(1);
    }
    const name = args.name;
    const command = Array.isArray(args.command) ? args.command.join(' ') : args.command;
    const client = new neoapi.Client(config);
    client.execBridge(name, command)
        .then((result) => {
            if (result.LastInsertedId == 0 && result.AffectedRows == 0) {
                console.println("executed.");
            } else {
                console.println(`executed. LastInsertedId: ${result.LastInsertedId}, AffectedRows: ${result.AffectedRows}`);
            }
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function queryBridge(config, args) {
    if (!args.name) {
        console.println("Error: Missing bridge name.");
        process.exit(1);
    }
    if (!args.command || args.command.length === 0) {
        console.println("Error: Missing query command.");
        process.exit(1);
    }
    const name = args.name;
    const command = Array.isArray(args.command) ? args.command.join(' ') : args.command;
    const client = new neoapi.Client(config);
    client.queryBridge(name, command)
        .then((result) => {
            if (result.Columns && result.Columns.length > 0) {
                let header = [];
                for (const col of result.Columns) {
                    header.push(col.Name);
                }
                let box = pretty.Table(config);
                let hasRows = true;
                let values = [];
                box.appendHeader(header);
                // while (hasRows) {
                //     client.fetchResultBridge(result.Handle)
                //         .then((rows) => {
                //             hasRows = !rows.HasNoRows;
                //             values = rows.Values;
                //         })
                //         .catch((err) => {
                //             console.println('Error:', err.message);
                //             hasRows = false;
                //         });
                //     box.append(values);
                // }
                client.closeResultBridge(result.Handle)
                    .catch((err) => {
                        console.println('Error:', err.message);
                    });
                console.println(box.render());
            } else {
                console.println("executed.");
            }
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}
