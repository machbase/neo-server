// net-data-test.js
// Test data transmission

const net = require('net');

const client = new net.Socket();

client.setEncoding('utf8'); // Set encoding to get strings instead of byte arrays

client.on('connect', () => {
    console.log('✓ Connected to google.com');
    client.write('GET / HTTP/1.0\r\nHost: google.com\r\n\r\n');
});

client.on('data', (data) => {
    console.log('✓ Received data (' + data.length + ' bytes)');
    console.log(data.substring(0, 100) + '...');
    client.end();
});

client.on('end', () => {
    console.log('✓ Connection ended');
});

client.on('close', () => {
    console.log('✓ Connection closed');
});

client.on('error', (err) => {
    console.error('Error:', err.message);
});

console.log('Connecting...');
client.connect(80, 'google.com');

setTimeout(() => {
    console.log('Timeout');
    client.destroy();
}, 5000);
