'use strict';
const { simple } = require('help/config');
module.exports = simple('cat', 'Concatenate files to standard output', 'Usage: cat [OPTION]... [FILE]...', {
    strict: false,
    options: {
        number: { type: 'boolean', short: 'n', description: 'Number all output lines', default: false },
        showEnds: { type: 'boolean', short: 'E', description: 'Display $ at end of each line', default: false },
        showTabs: { type: 'boolean', short: 'T', description: 'Display TAB characters as ^I', default: false },
        squeeze: { type: 'boolean', short: 's', description: 'Suppress repeated empty output lines', default: false },
        color: { type: 'boolean', short: 'c', description: 'Enable syntax highlighting', default: false },
    },
    positionals: [{ name: 'files', variadic: true, optional: true, description: 'Files to concatenate' }],
});

module.exports.content = `Usage: cat [OPTION]... [FILE]...
Concatenate FILE(s) to standard output.

Options:
    -n, --number          number all output lines
    -E, --showEnds        display $ at end of each line
    -T, --showTabs        display TAB characters as ^I
    -s, --squeeze         suppress repeated empty output lines
    -c, --color           enable syntax highlighting
    -h, --help            display this help and exit

Syntax highlighting (with -c) is supported for:
    .js, .json, .ndjson, .sql, .csv, .yaml, .yml, .toml

Examples:
    cat -c file.js        Display file.js with syntax highlighting
    cat -n data.json      Display data.json with line numbers
    cat -cs file1.txt     Squeeze blank lines with colors`;