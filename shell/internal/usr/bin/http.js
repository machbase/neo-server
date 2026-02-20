'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun } = require('/usr/lib/opts');

const optionHelp = { type: 'boolean', short: 'h', description: 'Show this help message', default: false }

const defaultConfig = {
    usage: 'Usage: http <command> [options]',
    options: {
        help: optionHelp,
    }
};

const debugConfig = {
    func: httpDebug,
    command: 'debug',
    usage: 'http debug',
    description: 'Show or set HTTP debug mode configuration',
    options: {
        help: optionHelp,
        enable: { type: 'string', description: 'Set debug mode (true/false)', default: '' },
        logLatency: { type: 'string', description: 'Log requests that take longer than the specified duration (e.g., 100ms)', default: '-1' },
        ...pretty.TableArgOptions,
    }
};


parseAndRun(process.argv.slice(2), defaultConfig, [
    debugConfig,
]);

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

