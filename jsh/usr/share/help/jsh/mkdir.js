'use strict';
const { simple } = require('help/config');
module.exports = simple('mkdir', 'Create directories', 'Usage: mkdir [OPTION]... DIRECTORY...', {
    strict: false,
    options: {
        parents: { type: 'boolean', short: 'p', description: 'Make parent directories as needed', default: false },
        verbose: { type: 'boolean', short: 'v', description: 'Print a message for each created directory', default: false },
    },
    positionals: [{ name: 'paths', variadic: true, description: 'Directories to create' }],
});