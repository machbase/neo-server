// net-server-demo.js
// Simple TCP server example

const net = require('net');

const PORT = 3000;
const HOST = '127.0.0.1';

// Create a server
const server = net.createServer((socket) => {
    console.log(`Client connected: ${socket.remoteAddress}:${socket.remotePort}`);
    
    // Send welcome message
    socket.write('Welcome to JSH TCP Server!\n');
    socket.write('Type something and press enter.\n');
    
    // Handle data from client
    socket.on('data', (data) => {
        const message = data.toString().trim();
        console.log(`Received: ${message}`);
        
        // Echo back to client
        socket.write(`Echo: ${message}\n`);
        
        // Exit command
        if (message.toLowerCase() === 'exit') {
            socket.write('Goodbye!\n');
            socket.end();
        }
    });
    
    socket.on('end', () => {
        console.log('Client disconnected');
    });
    
    socket.on('error', (err) => {
        console.error('Socket error:', err.message);
    });
});

// Handle server errors
server.on('error', (err) => {
    console.error('Server error:', err.message);
    process.exit(1);
});

// Start listening
server.listen(PORT, HOST, () => {
    const addr = server.address();
    console.log(`Server listening on ${addr.address}:${addr.port}`);
    console.log('Connect with: telnet 127.0.0.1', addr.port);
    
    // Auto-close after 30 seconds for testing
    setTimeout(() => {
        console.log('\nAuto-closing server...');
        server.close(() => {
            console.log('Server closed');
        });
    }, 30000);
});
