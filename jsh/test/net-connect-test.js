// net-connect-test.js
// Test simple connection

const net = require('net');

console.log('Testing connection...\n');

const server = net.createServer((socket) => {
    console.log('Server: Client connected!');
    socket.write('Welcome!\n');
    socket.end();
});

server.listen(0, '127.0.0.1', () => {
    const addr = server.address();
    console.log('Server listening on port:', addr.port);
    
    const client = new net.Socket();
    
    client.on('connect', () => {
        console.log('Client: Connected!');
        client.end();
    });
    
    client.on('data', (data) => {
        console.log('Client: Received data:', data);
    });
    
    client.on('close', () => {
        console.log('Client: Connection closed');
        server.close();
    });
    
    client.on('error', (err) => {
        console.error('Client: Error:', err);
    });
    
    console.log('Client: Connecting to port:', addr.port);
    client.connect(addr.port, '127.0.0.1');
});

server.on('close', () => {
    console.log('Server: Closed');
    console.log('\nTest done!');
});
