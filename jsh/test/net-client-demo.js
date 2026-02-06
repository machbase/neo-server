// net-client-demo.js
// Simple TCP client example

const net = require('net');

const PORT = 3000;
const HOST = '127.0.0.1';

console.log(`Connecting to ${HOST}:${PORT}...`);

const client = net.createConnection({ port: PORT, host: HOST }, () => {
    console.log('Connected to server');
    
    // Send initial message
    client.write('Hello from JSH client!\n');
});

// Handle data from server
client.on('data', (data) => {
    const message = data.toString();
    process.stdout.write(message);
});

// Handle connection end
client.on('end', () => {
    console.log('Disconnected from server');
});

// Handle errors
client.on('error', (err) => {
    console.error('Connection error:', err.message);
    process.exit(1);
});

// Send some test messages
setTimeout(() => {
    client.write('Test message 1\n');
}, 1000);

setTimeout(() => {
    client.write('Test message 2\n');
}, 2000);

setTimeout(() => {
    client.write('exit\n');
}, 3000);
