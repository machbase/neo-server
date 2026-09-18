'use strict';
const { simple } = require('help/config');
module.exports = simple('wc', 'Count lines, words, bytes, and characters', 'Usage: wc [OPTION]... [FILE]...', {
    strict: false,
    options: {
        lines: { type: 'boolean', short: 'l', description: 'Print the line counts', default: false },
        words: { type: 'boolean', short: 'w', description: 'Print the word counts', default: false },
        bytes: { type: 'boolean', short: 'c', description: 'Print the byte counts', default: false },
        chars: { type: 'boolean', short: 'm', description: 'Print the character counts', default: false },
    },
    positionals: [{ name: 'files', variadic: true, optional: true, description: 'Files to count' }],
});

module.exports.content = `Usage: wc [OPTION]... [FILE]...
Count lines, words, bytes, and characters for each FILE.
Read standard input when no FILE is given or when FILE is -.

Options:
    -l, --lines           print the line counts
    -w, --words           print the word counts
    -c, --bytes           print the byte counts
    -m, --chars           print the character counts
    -h, --help            display this help and exit`;