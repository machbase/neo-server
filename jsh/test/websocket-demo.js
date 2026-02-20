const {WebSocket} = require('ws');
const {parseArgs} = require('/util');
const args = require('process').argv.slice(2);

const opts = parseArgs(args, {
    options: {
        verbose: { type: 'boolean', short: 'v' }
    },
    allowPositionals: true
});

console.println(opts);

if (!opts.positionals || opts.positionals.length < 1) {
    console.println('Usage: websocket.js <ws://host:port/path>');
    require('/lib/process').exit(1);
}

const ws = new WebSocket(opts.positionals[0]);
ws.addEventListener('open', () => {
    console.printf('WebSocket connection opened\n');
});
ws.addEventListener('message', (event) => {
    console.printf('%v', event.data);
});
ws.addEventListener('close', (event) => {
    console.println('WebSocket connection closed:', event);
});
ws.addEventListener('error', (e) => {
    console.println('WebSocket error:', e.message);
});
