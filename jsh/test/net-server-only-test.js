// net-server-only-test.js
// Test server listening and events

const net = require('net');

console.log('Creating server...');
const server = net.createServer((socket) => {
    console.log('✓ Connection received!');
    console.log('Socket:', typeof socket, socket);
    console.log('Socket constructor:', socket.constructor.name);
    
    // Try to access socket properties
    try {
        console.log('Socket.remoteAddress:', socket.remoteAddress);
        console.log('Socket.remotePort:', socket.remotePort);
    } catch (err) {
        console.error('Error accessing socket props:', err.message);
    }
    
    socket.end();
    server.close();
});

server.on('listening', () => {
    console.log('✓ Server listening event received');
    const addr = server.address();
    console.log('Address:', addr);
    
    // Create a client to connect
    console.log('\nConnecting client...');
    const client = net.createConnection(addr.port, '127.0.0.1', () => {
        console.log('✓ Client connected');
    });
    
    client.on('end', () => {
        console.log('✓ Client ended');
    });
    
    client.on('close', () => {
        console.log('✓ Client closed');
    });
});

server.on('close', () => {
    console.log('✓ Server closed');
    console.log('\nTest complete!');
});

server.on('error', (err) => {
    console.error('Server error:', err.message);
});

console.log('Starting server...');
server.listen(0, '127.0.0.1');
