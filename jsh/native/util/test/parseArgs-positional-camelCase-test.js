const {parseArgs} = require('util');

console.log('=== Test 1: kebab-case positional argument name ===');
const result1 = parseArgs(['my-tql-file'], {
    positionals: [{ name: 'tql-name' }],
    allowPositionals: true
});
console.log('positionals:', result1.positionals);
console.log('namedPositionals.tqlName:', result1.namedPositionals.tqlName);
console.log('Expected: my-tql-file');
console.log();

console.log('=== Test 2: Multiple kebab-case positional arguments ===');
const result2 = parseArgs(['input.txt', 'output.txt'], {
    positionals: [
        { name: 'input-file' },
        { name: 'output-file' }
    ],
    allowPositionals: true
});
console.log('namedPositionals.inputFile:', result2.namedPositionals.inputFile);
console.log('namedPositionals.outputFile:', result2.namedPositionals.outputFile);
console.log('Expected: input.txt, output.txt');
console.log();

console.log('=== Test 3: Variadic kebab-case positional ===');
const result3 = parseArgs(['file1.js', 'file2.js', 'file3.js'], {
    positionals: [
        { name: 'source-files', variadic: true }
    ],
    allowPositionals: true
});
console.log('namedPositionals.sourceFiles:', result3.namedPositionals.sourceFiles);
console.log('Expected: [file1.js, file2.js, file3.js]');
console.log();

console.log('=== Test 4: Optional kebab-case positional with default ===');
const result4 = parseArgs([], {
    positionals: [
        { name: 'config-file', optional: true, default: 'config.json' }
    ],
    allowPositionals: true
});
console.log('namedPositionals.configFile:', result4.namedPositionals.configFile);
console.log('Expected: config.json');
console.log();

console.log('=== Test 5: Mix of simple and kebab-case names ===');
const result5 = parseArgs(['cmd', 'param'], {
    positionals: [
        { name: 'command' },
        { name: 'command-param' }
    ],
    allowPositionals: true
});
console.log('namedPositionals.command:', result5.namedPositionals.command);
console.log('namedPositionals.commandParam:', result5.namedPositionals.commandParam);
console.log('Expected: cmd, param');
console.log();

console.log('=== Test 6: Complex kebab-case with multiple hyphens ===');
const result6 = parseArgs(['value'], {
    positionals: [
        { name: 'my-special-config-name' }
    ],
    allowPositionals: true
});
console.log('namedPositionals.mySpecialConfigName:', result6.namedPositionals.mySpecialConfigName);
console.log('Expected: value');
console.log();

console.log('All positional camelCase conversion tests completed!');
