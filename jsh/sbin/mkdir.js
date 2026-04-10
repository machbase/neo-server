(() => {
    'use strict';

    const process = require('process');
    const fs = require('fs');
    const parseArgs = require('util/parseArgs');
    const pwd = process.env.get('PWD') || '/';

    const config = {
        usage: 'Usage: mkdir [OPTION]... DIRECTORY...',
        description: 'Create the DIRECTORY(ies), if they do not already exist.',
        options: {
            parents: { type: 'boolean', short: 'p', description: 'Make parent directories as needed', default: false },
            verbose: { type: 'boolean', short: 'v', description: 'Print a message for each created directory', default: false },
            help: { type: 'boolean', short: 'h', description: 'Show help', default: false },
        },
        allowPositionals: true,
        strict: false,
        positionals: [{ name: 'paths', variadic: true }],
    };

    const { values, positionals } = parseArgs(process.argv.slice(2), config);

    if (values.help) {
        console.println(parseArgs.formatHelp(config));
        process.exit(0);
    }

    if (!positionals || positionals.length === 0) {
        console.println('mkdir: missing operand');
        console.println("Try 'mkdir --help' for more information.");
        process.exit(1);
    }

    try {
        for (const targetPath of positionals) {
            createDirectory(targetPath, values.parents, values.verbose);
        }
    } catch (err) {
        console.println(err.message);
        process.exit(1);
    }

    function createDirectory(targetPath, recursive, verbose) {
        const resolvedPath = resolvePath(targetPath);

        if (fs.existsSync(resolvedPath)) {
            const stat = fs.statSync(resolvedPath);
            if (stat.isDirectory()) {
                if (recursive) {
                    return;
                }
                throw new Error(`mkdir: cannot create directory '${targetPath}': File exists`);
            }
            throw new Error(`mkdir: cannot create directory '${targetPath}': File exists`);
        }

        fs.mkdirSync(resolvedPath, { recursive });
        if (verbose) {
            console.println(`mkdir: created directory '${targetPath}'`);
        }
    }

    function resolvePath(targetPath) {
        if (typeof targetPath !== 'string' || targetPath.length === 0) {
            return pwd;
        }
        if (targetPath.startsWith('/')) {
            return targetPath;
        }
        if (pwd === '/') {
            return `/${targetPath}`;
        }
        return `${pwd}/${targetPath}`;
    }
})();