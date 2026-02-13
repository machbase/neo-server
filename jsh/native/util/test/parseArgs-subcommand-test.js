const parseArgs = require('/lib/util/parseArgs');

console.log('=== Sub-command Test 1: add command ===');
const args1 = ['add', '-f', '--message', 'Initial commit', 'file.txt'];
const result1 = parseArgs(args1, 
    {
        command: 'add',
        options: {
            force: { type: 'boolean', short: 'f' },
            message: { type: 'string', short: 'm' }
        },
        positionals: ['file'],
        allowPositionals: true
    },
    {
        command: 'remove',
        options: {
            recursive: { type: 'boolean', short: 'r' }
        },
        positionals: ['file'],
        allowPositionals: true
    }
);
console.log('command:', result1.command);
console.log('force:', result1.values.force);
console.log('message:', result1.values.message);
console.log('file:', result1.namedPositionals.file);
console.log();

console.log('=== Sub-command Test 2: remove command ===');
const args2 = ['remove', '-r', 'dir/'];
const result2 = parseArgs(args2,
    {
        command: 'add',
        options: {
            force: { type: 'boolean', short: 'f' }
        },
        allowPositionals: true
    },
    {
        command: 'remove',
        options: {
            recursive: { type: 'boolean', short: 'r' },
            verbose: { type: 'boolean', short: 'v' }
        },
        positionals: ['file'],
        allowPositionals: true
    }
);
console.log('command:', result2.command);
console.log('recursive:', result2.values.recursive);
console.log('file:', result2.namedPositionals.file);
console.log();

console.log('=== Sub-command Test 3: default config (no matching command) ===');
const args3 = ['--help'];
const result3 = parseArgs(args3,
    {
        options: {
            help: { type: 'boolean', short: 'h' }
        },
        strict: false
    },
    {
        command: 'add',
        options: {
            force: { type: 'boolean', short: 'f' }
        }
    }
);
console.log('command:', result3.command);
console.log('help:', result3.values.help);
console.log();

console.log('=== Sub-command Test 4: git-like commit ===');
const args4 = ['commit', '-am', 'Fix bug'];
const result4 = parseArgs(args4,
    {
        command: 'commit',
        options: {
            message: { type: 'string', short: 'm' },
            all: { type: 'boolean', short: 'a' },
            amend: { type: 'boolean' }
        }
    },
    {
        command: 'push',
        options: {
            force: { type: 'boolean', short: 'f' }
        }
    }
);
console.log('command:', result4.command);
console.log('all:', result4.values.all);
console.log('message:', result4.values.message);
console.log();

console.log('=== Sub-command Test 5: git-like push with positionals ===');
const args5 = ['push', '-f', 'origin', 'main'];
const result5 = parseArgs(args5,
    {
        command: 'commit',
        options: {
            message: { type: 'string', short: 'm' }
        }
    },
    {
        command: 'push',
        options: {
            force: { type: 'boolean', short: 'f' },
            tags: { type: 'boolean' }
        },
        positionals: ['remote', { name: 'branch', optional: true }],
        allowPositionals: true
    }
);
console.log('command:', result5.command);
console.log('force:', result5.values.force);
console.log('remote:', result5.namedPositionals.remote);
console.log('branch:', result5.namedPositionals.branch);
console.log();

console.log('=== All sub-command tests passed ===');
