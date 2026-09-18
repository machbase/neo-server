'use strict';

const { simple } = require('help/config');

module.exports = simple('run', 'Run a script file', 'Usage: run [options] <filename>', {
    allowNegative: true,
    options: {
        stopOnError: { type: 'boolean', short: 'e', description: 'Stop executing if any statement returns a non-zero exit code', default: false },
        verbose: { type: 'boolean', short: 'v', description: 'Enable verbose output', default: false },
    },
    positionals: [{ name: 'filename', type: 'string', description: 'script file path to run' }],
});