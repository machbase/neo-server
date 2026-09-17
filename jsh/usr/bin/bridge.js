'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/bridge');

const commandFunctions = {
    list: listBridges,
    add: addBridge,
    del: delBridge,
    test: testBridge,
    stats: statsBridge,
    exec: execBridge,
    query: queryBridge,
};
const commandConfigs = Object.keys(help.commands).map((name) => ({
    ...help.commands[name],
    command: name,
    func: commandFunctions[name],
}));

parseAndRun(process.argv.slice(2), help, commandConfigs);

function listBridges(config, args) {
    const client = new neoapi.Client(config);
    client.listBridges()
        .then((lst) => {
            let box = pretty.Table(config);
            box.appendHeader(['ID', 'NAME', 'IS_PUBLIC', 'ALLOWED_USER', 'TYPE', 'CONNECTION']);
            for (const br of lst) {
                box.append([br.id, br.name, br.isPublic, br.allowedUser, br.type, br.path]);
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
        .then(() => {
            console.println("Deleted.");
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
    console.println("Testing bridge...", name);
    client.testBridge(name)
        .then((result) => {
            console.println(result ? "OK." : "Failed.");
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
            console.println("executed.");
            // DEBUG: console.println(`executed. LastInsertedId: ${result.LastInsertedId}, RowsAffected: ${result.RowsAffected}`);
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
            if (!result.Columns || result.Columns.length === 0) {
                console.println("executed.");
                return;
            }
            let header = [];
            for (const col of result.Columns) {
                header.push(col.Name);
            }
            let box = pretty.Table(config);
            box.appendHeader(header);

            const fetchRows = () => {
                client.fetchResultBridge(result.Handle)
                    .then((rows) => {
                        box.append(rows.Values);
                        if (!rows.HasNoRows) {
                            setImmediate(fetchRows);
                        } else {
                            console.println(box.render());
                            client.closeResultBridge(result.Handle)
                                .catch((err) => {
                                    console.println('Error:', err.message);
                                });
                        }
                    })
                    .catch((err) => {
                        console.println('Error:', err.message);
                    });
            };
            fetchRows();
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}
