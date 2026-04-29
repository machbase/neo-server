const process = require('process');
const env = process.env;
const q = env.get('QUERY_STRING') || '';

const out = console.println;
const conf = require('./config.json');

out("Content-Type: text/plain; charset=utf-8;");
out();

out(`GREETING: ${conf.greeting} ${q}`);
