'use strict';
const { command, root } = require('help/config');

const dir = { type: 'string', short: 'C', description: 'Use this project directory instead of the current working directory' };
const global = { type: 'boolean', short: 'g', description: 'Install into the reserved global package directory /work and ignore --dir', default: false };

module.exports = root('pkg', 'Manage JSH packages', 'Usage: pkg <command> [options]', {
    init: command('pkg init [options] <name>', 'Create a package.json in the selected project directory', {
        options: { dir },
        positionals: [{ name: 'name', description: 'Package name for the current project' }],
    }),
    install: command('pkg install [options] [name]', 'Install dependencies and maintain package-lock.json', {
        options: { dir, global },
        positionals: [{ name: 'name', description: 'Optional package name to add or update', optional: true }],
    }),
    uninstall: command('pkg uninstall [options] <name>', 'Remove a dependency and its generated command wrapper', {
        options: { dir, global },
        positionals: [{ name: 'name', description: 'Package name to remove' }],
    }),
    copy: command('pkg copy [options] <source> <dest>', 'Copy a GitHub package source and install dependencies', {
        options: { force: { type: 'boolean', short: 'f', description: 'Proceed even if the destination directory is not empty', default: false } },
        positionals: [
            { name: 'source', description: 'GitHub repository package source to copy' },
            { name: 'dest', description: 'Destination directory path resolved from the current working directory' },
        ],
    }),
    run: command('pkg run [options] <key> [...args]', 'Run a package.json script from the selected project directory', {
        options: { dir },
        positionals: [
            { name: 'key', description: 'Script name in package.json' },
            { name: 'args', description: 'Additional arguments to append to the script', optional: true, variadic: true },
        ],
    }),
});