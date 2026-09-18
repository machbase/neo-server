'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/http');

const commandConfigs = [{ ...help.commands.debug, command: 'debug', func: httpDebug }];

parseAndRun(process.argv.slice(2), help, commandConfigs);

function httpDebug(config, args) {
    const newConfig = {};
    if (config.enable !== '' || config.logLatency !== '-1') {
        // Set debug config
        if (config.enable !== '') {
            let strEnable = config.enable.toLowerCase();
            newConfig.enable = strEnable === 'true' || strEnable === '1' || strEnable === 'yes' || strEnable === 'on';
        }
        if (config.logLatency !== '-1') {
            newConfig.logLatency = config.logLatency;
        }
    }
    const client = new neoapi.Client(config);
    client.setHttpDebug(newConfig)
        .then((nfo) => {
            let box = pretty.Table(config);
            box.appendHeader(['NAME', 'VALUE']);
            box.appendRow(box.row('enable', nfo.enable));
            box.appendRow(box.row('logLatency', nfo.logLatency));
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

