(() => {
    const process = require('process');
    const args = process.argv.slice(2);
    const message = args[0] || 'stderr';
    const exitCode = args.length > 1 ? Number(args[1]) : 0;
    process.stderr.write(message + '\n');
    process.exit(exitCode);
})()