const {parseArgs} = require('util');

console.log('=== Test 1: camelCase to kebab-case conversion ===');
const result1 = parseArgs(['--user-name', 'Alice'], {
    options: { userName: { type: 'string' } }
});
console.log('userName:', result1.values.userName);
console.log();

console.log('=== Test 2: Complex camelCase ===');
const result2 = parseArgs(['--max-retry-count', '10'], {
    options: { maxRetryCount: { type: 'string' } }
});
console.log('maxRetryCount:', result2.values.maxRetryCount);
console.log();

console.log('=== Test 3: Boolean with camelCase ===');
const result3 = parseArgs(['--enable-debug'], {
    options: { enableDebug: { type: 'boolean' } }
});
console.log('enableDebug:', result3.values.enableDebug);
console.log();

console.log('=== Test 4: Negative boolean with camelCase ===');
const result4 = parseArgs(['--no-enable-debug'], {
    options: { enableDebug: { type: 'boolean' } },
    allowNegative: true
});
console.log('enableDebug (negative):', result4.values.enableDebug);
console.log();

console.log('=== Test 5: Multiple camelCase options ===');
const result5 = parseArgs(['--user-name', 'Bob', '--max-connections', '100', '--enable-ssl'], {
    options: {
        userName: { type: 'string' },
        maxConnections: { type: 'string' },
        enableSsl: { type: 'boolean' }
    }
});
console.log('userName:', result5.values.userName);
console.log('maxConnections:', result5.values.maxConnections);
console.log('enableSsl:', result5.values.enableSsl);
console.log();

console.log('=== Test 6: Simple name (no camelCase) still works ===');
const result6 = parseArgs(['--port', '8080', '--verbose'], {
    options: {
        port: { type: 'string' },
        verbose: { type: 'boolean' }
    }
});
console.log('port:', result6.values.port);
console.log('verbose:', result6.values.verbose);
console.log();

console.log('=== Test 7: Short option with camelCase long name ===');
const result7 = parseArgs(['-u', 'Charlie'], {
    options: {
        userName: { type: 'string', short: 'u' }
    }
});
console.log('userName (via -u):', result7.values.userName);
console.log();

console.log('=== Test 8: Mix of kebab-case flag and camelCase property ===');
const result8 = parseArgs(['--connection-timeout', '30', '--max-retry-count', '3'], {
    options: {
        connectionTimeout: { type: 'string' },
        maxRetryCount: { type: 'string' }
    }
});
console.log('connectionTimeout:', result8.values.connectionTimeout);
console.log('maxRetryCount:', result8.values.maxRetryCount);
console.log();

console.log('All tests completed!');
