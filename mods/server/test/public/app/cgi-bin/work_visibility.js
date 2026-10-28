const fs = require('fs');

const out = console.println;
const conf = require('./config.json');

out("Content-Type: application/json");
out();
out(JSON.stringify({
    work: fs.readdirSync('/work').filter((name) => name !== '.' && name !== '..').sort(),
    greeting: conf.greeting,
}));
