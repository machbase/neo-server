'use strict';
const { simple } = require('help/config');
module.exports = simple('tail', 'Output the last part of a file', 'Usage: tail [options] <file>', {
    options: {
        follow: { type: 'boolean', short: 'f', description: 'Follow the file as it grows', default: false },
        lines: { type: 'string', short: 'n', description: 'Number of lines to print', default: '10' },
    },
    positionals: [{ name: 'file', type: 'string', description: 'File to tail' }],
});