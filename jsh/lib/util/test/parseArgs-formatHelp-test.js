const parseArgs = require('/lib/util/parseArgs');

console.log('=== Test 1: Basic formatHelp with camelCase options ===');
const help1 = parseArgs.formatHelp({
    usage: 'Usage: myapp [options] <file>',
    options: {
        userName: { type: 'string', short: 'u', description: 'User name', default: 'guest' },
        maxRetryCount: { type: 'string', short: 'r', description: 'Maximum retry count', default: '3' },
        enableDebug: { type: 'boolean', short: 'd', description: 'Enable debug mode', default: false },
        port: { type: 'string', short: 'p', description: 'Port number', default: '8080' }
    },
    positionals: [
        { name: 'file', description: 'Input file to process' }
    ]
});
console.log(help1);
console.log();

console.log('=== Test 2: formatHelp with variadic positional ===');
const help2 = parseArgs.formatHelp({
    usage: 'Usage: command [options] <files...>',
    options: {
        outputDir: { type: 'string', short: 'o', description: 'Output directory' },
        verboseMode: { type: 'boolean', short: 'v', description: 'Verbose output' }
    },
    positionals: [
        { name: 'files', variadic: true, description: 'Files to process' }
    ]
});
console.log(help2);
console.log();

console.log('=== Test 3: toKebabCase function ===');
console.log('userName ->', parseArgs.toKebabCase('userName'));
console.log('maxRetryCount ->', parseArgs.toKebabCase('maxRetryCount'));
console.log('enableDebug ->', parseArgs.toKebabCase('enableDebug'));
console.log('port ->', parseArgs.toKebabCase('port'));
console.log('HTTPServer ->', parseArgs.toKebabCase('HTTPServer'));
console.log();

console.log('=== Test 4: formatHelp with sub-commands ===');
const help4 = parseArgs.formatHelp(
    {
        usage: 'Usage: git <command> [options]',
        options: {
            help: { type: 'boolean', short: 'h', description: 'Show help' }
        }
    },
    {
        command: 'commit',
        description: 'Record changes to the repository',
        longDescription: 'The commit command captures a snapshot of the project\'s currently staged changes.',
        options: {
            message: { type: 'string', short: 'm', description: 'Commit message' },
            all: { type: 'boolean', short: 'a', description: 'Stage all changes' }
        }
    },
    {
        command: 'push',
        description: 'Update remote refs',
        options: {
            force: { type: 'boolean', short: 'f', description: 'Force push' }
        },
        positionals: [
            { name: 'remote', description: 'Remote name' },
            { name: 'branch', optional: true, description: 'Branch name' }
        ]
    }
);
console.log(help4);
