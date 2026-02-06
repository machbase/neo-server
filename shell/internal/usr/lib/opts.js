'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');

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
        if (argv.length > 0) {
            const cmd = argv[0].toLowerCase();
            for (const c of configs) {
                if (c.command.toLowerCase() === cmd) {
                    const help = parseArgs.formatHelp(
                        c
                    );
                    console.println(help);
                    return;
                }
            }
        }
        const help = parseArgs.formatHelp(defaultConfig, ...configs);
        console.println(help);
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
    parseAndRun,
}