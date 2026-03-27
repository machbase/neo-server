(() => {
    const process = require('process');
    const args = process.argv.slice(2);
    console.println(args.join('|'));
})()
