const {parseArgs} = require('util');
const process = require('process');

console.log('\n=== Test 1: Simple named positionals ===');
try {
    const result1 = parseArgs(['input.txt', 'output.txt'], {
        options: {},
        allowPositionals: true,
        positionals: ['inputFile', 'outputFile']
    });
    console.log('positionals:', result1.positionals);
    console.log('namedPositionals:', result1.namedPositionals);
} catch (error) {
    console.log('Error:', error.message);
    process.exit(1);
}

console.log('\n=== Test 2: Optional positionals ===');
const result2 = parseArgs(['input.txt'], {
    options: {},
    allowPositionals: true,
    positionals: [
        'inputFile',
        { name: 'outputFile', optional: true, default: 'stdout' }
    ]
});
console.log('positionals:', result2.positionals);
console.log('namedPositionals:', result2.namedPositionals);

console.log('\n=== Test 3: Variadic positionals ===');
const result3 = parseArgs(['input.txt', 'output.txt', 'file1.js', 'file2.js', 'file3.js'], {
    options: {},
    allowPositionals: true,
    positionals: [
        'inputFile',
        'outputFile',
        { name: 'files', variadic: true }
    ]
});
console.log('positionals:', result3.positionals);
console.log('namedPositionals:', result3.namedPositionals);

console.log('\n=== Test 4: With options ===');
const result4 = parseArgs(['-v', '--config', 'app.json', 'src.js', 'dest.js'], {
    options: {
        verbose: { type: 'boolean', short: 'v' },
        config: { type: 'string' }
    },
    allowPositionals: true,
    positionals: ['source', 'destination']
});
console.log('values:', result4.values);
console.log('positionals:', result4.positionals);
console.log('namedPositionals:', result4.namedPositionals);

console.log('\n=== Test 5: Optional variadic (empty) ===');
const result5 = parseArgs(['input.txt'], {
    options: {},
    allowPositionals: true,
    positionals: [
        'inputFile',
        { name: 'includes', variadic: true, optional: true }
    ]
});
console.log('positionals:', result5.positionals);
console.log('namedPositionals:', result5.namedPositionals);

console.log('\n=== Test 6: Error - Missing required ===');
try {
    parseArgs(['input.txt'], {
        options: {},
        allowPositionals: true,
        positionals: ['inputFile', 'outputFile']  // outputFile is required
    });
} catch (error) {
    console.log('Error:', error.message);
}

console.log('\n=== Test 7: Error - Variadic not last ===');
try {
    parseArgs([], {
        options: {},
        allowPositionals: true,
        positionals: [
            { name: 'files', variadic: true },
            'output'  // Cannot come after variadic
        ]
    });
} catch (error) {
    console.log('Error:', error.message);
}

console.log('\n=== All tests completed ===');
