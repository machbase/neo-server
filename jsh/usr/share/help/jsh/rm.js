'use strict';
const { simple } = require('help/config');
module.exports = simple('rm', 'Remove files or directories', 'Usage: rm [OPTION]... FILE...', {
    strict: false,
    options: {
        recursive: { type: 'boolean', short: 'r', description: 'Remove directories and their contents recursively', default: false },
        dir: { type: 'boolean', short: 'd', description: 'Remove empty directories', default: false },
        force: { type: 'boolean', short: 'f', description: 'Ignore nonexistent files and arguments, never prompt', default: false },
        verbose: { type: 'boolean', short: 'v', description: 'Print a message for each removed path', default: false },
    },
    positionals: [{ name: 'paths', variadic: true, description: 'Files or directories to remove' }],
});