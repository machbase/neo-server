const {parseArgs} = require('util');

console.log('=== Test 1: integer type ===');
const result1 = parseArgs(['--port', '8080', '--max-connections', '100'], {
    options: {
        port: { type: 'integer' },
        maxConnections: { type: 'integer' }
    }
});
console.log('port:', result1.values.port, 'type:', typeof result1.values.port);
console.log('maxConnections:', result1.values.maxConnections, 'type:', typeof result1.values.maxConnections);
console.log();

console.log('=== Test 2: float type ===');
const result2 = parseArgs(['--threshold', '3.14', '--ratio', '0.5'], {
    options: {
        threshold: { type: 'float' },
        ratio: { type: 'float' }
    }
});
console.log('threshold:', result2.values.threshold, 'type:', typeof result2.values.threshold);
console.log('ratio:', result2.values.ratio, 'type:', typeof result2.values.ratio);
console.log();

console.log('=== Test 3: integer with short option ===');
const result3 = parseArgs(['-p', '5432', '-c', '10'], {
    options: {
        port: { type: 'integer', short: 'p' },
        count: { type: 'integer', short: 'c' }
    }
});
console.log('port:', result3.values.port);
console.log('count:', result3.values.count);
console.log();

console.log('=== Test 4: negative integer ===');
const result4 = parseArgs(['--offset', '-5', '--precision', '-1'], {
    options: {
        offset: { type: 'integer' },
        precision: { type: 'integer' }
    }
});
console.log('offset:', result4.values.offset);
console.log('precision:', result4.values.precision);
console.log();

console.log('=== Test 5: integer with inline value ===');
const result5 = parseArgs(['--port=3000', '-c=20'], {
    options: {
        port: { type: 'integer' },
        count: { type: 'integer', short: 'c' }
    }
});
console.log('port:', result5.values.port);
console.log('count:', result5.values.count);
console.log();

console.log('=== Test 6: Error - decimal for integer ===');
try {
    parseArgs(['--count', '3.14'], {
        options: {
            count: { type: 'integer' }
        }
    });
    console.log('ERROR: Should have thrown');
} catch (err) {
    console.log('Caught expected error:', err.message);
}
console.log();

console.log('=== Test 7: Error - invalid number ===');
try {
    parseArgs(['--port', 'abc'], {
        options: {
            port: { type: 'integer' }
        }
    });
    console.log('ERROR: Should have thrown');
} catch (err) {
    console.log('Caught expected error:', err.message);
}
console.log();

console.log('=== Test 8: multiple integer values ===');
const result8 = parseArgs(['--id', '1', '--id', '2', '--id', '3'], {
    options: {
        id: { type: 'integer', multiple: true }
    }
});
console.log('ids:', result8.values.id);
console.log();

console.log('=== Test 9: mix of types ===');
const result9 = parseArgs(['--port', '8080', '--ratio', '0.75', '--debug'], {
    options: {
        port: { type: 'integer' },
        ratio: { type: 'float' },
        debug: { type: 'boolean' }
    }
});
console.log('port:', result9.values.port, 'type:', typeof result9.values.port);
console.log('ratio:', result9.values.ratio, 'type:', typeof result9.values.ratio);
console.log('debug:', result9.values.debug, 'type:', typeof result9.values.debug);
console.log();

console.log('All tests completed!');
