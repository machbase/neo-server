(() => {
    const process = require('process');
    const args = process.argv.slice(2);
    if (args.length === 0) {
        console.println();
        return;
    }

    output = [];
    for (let i = 0; i < args.length; i++) {
        output.push(args[i]);
    }

    console.println(...output);
})()
