'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const help = require('help');

function getMachCliConfig(conf = {}) {
    const session = require('@jsh/session');
    return { ...session.getMachCliConfig(), ...conf };
}

function newMachCliClient(conf = {}) {
    const machCliConf = getMachCliConfig(conf);
    return new (require('machcli').Client)(machCliConf);
}

function formatCommandHelp(argv, defaultConfig, configs) {
    if (defaultConfig.commands) {
        if (argv.length > 0) {
            const commandHelp = defaultConfig.commands[argv[0].toLowerCase()];
            if (commandHelp) {
                return help.format(commandHelp);
            }
        }
        return help.format(defaultConfig);
    }
    if (argv.length > 0) {
        const cmd = argv[0].toLowerCase();
        for (const config of configs) {
            if (config.command.toLowerCase() === cmd) {
                return parseArgs.formatHelp(config);
            }
        }
    }
    return parseArgs.formatHelp(defaultConfig, ...configs);
}

function parseAndRun(argv, defaultConfig, configs) {
    let showHelp = false;
    let config = {};
    let args = {};
    let command = null;

    try {
        const parsed = parseArgs(argv, defaultConfig, ...configs);

        config = parsed.values;
        args = parsed.namedPositionals || {};
        command = parsed.command;
        showHelp = config.help;
    }
    catch (err) {
        console.println(err.message);
        showHelp = true;
    }


    function printHelp() {
        console.println(formatCommandHelp(argv, defaultConfig, configs));
    }

    if (showHelp || !command) {
        printHelp();
        process.exit(showHelp ? 0 : 1);
    }

    command = command.toLowerCase();
    let commandFunc = null;
    for (const c of configs) {
        if (c.command.toLowerCase() === command) {
            commandFunc = c.func;
            break;
        }
    }

    // Validate that the provided command is in the allowed list
    if (!commandFunc) {
        console.println(`Error: Unknown command '${command}'\n`);
        printHelp();
        process.exit(1);
    }

    // Dispatch to appropriate handler based on command
    commandFunc(config, args);
}

module.exports = {
    formatCommandHelp,
    getMachCliConfig,
    newMachCliClient,
    parseAndRun,
}