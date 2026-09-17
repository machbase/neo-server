'use strict';

const { command, root } = require('help/config');

module.exports = root('ssh-key', 'Manage SSH keys', 'Usage: ssh-key <command> [options]', {
    list: command('ssh-key list', 'List all registered ssh keys', { table: true }),
    add: command('ssh-key add <type> <key> [comment]', 'Add a new ssh key', {
        positionals: [
            { name: 'type', description: 'Type of the ssh key (e.g., rsa, dsa, ecdsa, ed25519)' },
            { name: 'key', description: 'The public key string' },
            { name: 'comment', variadic: true, description: 'A comment for the key (e.g., email or identifier)' },
        ],
    }),
    del: command('ssh-key del <fingerprint>', 'Delete an existing ssh key', {
        positionals: [{ name: 'fingerprint', description: 'The fingerprint of the ssh key to delete' }],
    }),
});