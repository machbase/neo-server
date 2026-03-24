const process = require('process');

console.println("Hello", "0:", process.argv[0], "1:", process.argv[1], "2:", process.argv[2]);
console.println("Environment variables:");
console.println(`ENV_VAR1=${process.env.get('ENV_VAR1')}`);
console.println(`ENV_VAR2=${process.env.get('ENV_VAR2')}`);
