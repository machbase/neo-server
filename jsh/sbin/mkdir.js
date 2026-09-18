(() => {
    'use strict';

    const process = require('process');
    const fs = require('fs');
    const parseArgs = require('util/parseArgs');
    const help = require('help');
    const metadata = require('/usr/share/help/jsh/mkdir');
    const pwd = process.env.get('PWD') || '/';

    const { values, positionals } = parseArgs(process.argv.slice(2), metadata);

    if (values.help) {
        console.println(help.format(metadata));
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