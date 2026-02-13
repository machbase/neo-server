#!/usr/bin/env jsh
// Demo: Stream module usage

const { Readable, Writable, PassThrough } = require('stream');

console.log('=== Stream Module Demo ===\n');

// Demo 1: PassThrough - Simple buffering
console.log('Demo 1: PassThrough Stream');
console.log('---------------------------');

const passthrough = new PassThrough();

// Setup event listeners
passthrough.on('data', (chunk) => {
    console.log('Received data event:', chunk.toString());
});

passthrough.on('finish', () => {
    console.log('Stream finished');
});

passthrough.on('close', () => {
    console.log('Stream closed');
});

// Write some data
passthrough.write('Hello, ');
passthrough.write('Stream ');
passthrough.write('World!');
passthrough.end('\n');

// Read the data
const data = passthrough.read();
if (data) {
    console.log('Direct read result:', data.toString());
}

console.log('\n');

// Demo 2: Error handling
console.log('Demo 2: Error Handling');
console.log('----------------------');

const errorStream = new PassThrough();

errorStream.on('error', (err) => {
    console.log('Caught error:', err.message);
});

errorStream.on('close', () => {
    console.log('Error stream closed');
});

// Simulate an error scenario
try {
    errorStream.write('Test data');
    errorStream.close();
    // Try to write after close (should be handled gracefully)
    errorStream.write('This should not work');
} catch (err) {
    console.log('Caught exception:', err.message);
}

console.log('\n=== Demo Complete ===');
