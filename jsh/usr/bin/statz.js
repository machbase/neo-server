'use strict';

const process = require('process');
const pretty = require('pretty');
const viz = require('vizspec');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/statz');

const commandFunctions = { list: statzKeys, get: statzGet, viz: statzViz };
const commandConfigs = Object.keys(help.commands).map((name) => ({
    ...help.commands[name],
    command: name,
    func: commandFunctions[name],
}));

parseAndRun(process.argv.slice(2), help, commandConfigs);

function statzKeys(config, args) {
    const client = new neoapi.Client(config);
    client.getServerStatzKeys(args.names)
        .then((rsp) => {
            if (rsp.length === 0) {
                console.println("No matching statz metrics found.");
            } else {
                for (const name of rsp) {
                    console.println(name);
                }
            }
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function statzGet(config, args) {
    const client = new neoapi.Client(config);
    client.getServerStatzQuery({ names: args.names, maxRows: config.nrow })
        .then((rsp) => {
            let box = pretty.Table(config);
            if (rsp.types[0] === 'datetime') {
                rsp.columns[0] = `${rsp.columns[0]}(${config.tz})`;
            }
            box.appendHeader(rsp.columns);
            box.setColumnTypes(rsp.types);
            for (const row of rsp.rows) {
                if (rsp.types[0] === 'datetime' && row[0] !== null && row[0] !== undefined) {
                    // row[0] is expected to be timestamp in RFC3339 format,
                    // convert it to Date object for better display formatting
                    row[0] = pretty.parseTime(row[0], '2006-01-02T15:04:05Z07:00', 'Local');
                }
                box.append([...row]);
            }
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function statzViz(config, args) {
    const client = new neoapi.Client(config);
    client.getServerStatz(...args.names)
        .then((rsp) => {
            for (const stat of rsp.statz) {
                console.println("[" + stat.name + "]");
                let spec = viz.createSpec(stat.spec);

                console.println(viz.toTUILines(spec, { rows: 4 }).join("\n"));
                console.println();
            }
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}
