'use strict';
const { simple } = require('help/config');
module.exports = simple('ls', 'List directory contents', 'Usage: ls [options] [path...]', {
    strict: false,
    options: {
        long: { type: 'boolean', short: 'l', description: 'Use a long listing format', default: false },
        all: { type: 'boolean', short: 'a', description: 'Include hidden entries', default: false },
        time: { type: 'boolean', short: 't', description: 'Sort by modification time', default: false },
        recursive: { type: 'boolean', short: 'R', description: 'List subdirectories recursively', default: false },
    },
    positionals: [{ name: 'paths', variadic: true, optional: true, description: 'Paths to list' }],
});