(() => {
    'use strict';

    const process = require('process');
    const fs = require('fs');
    const parseArgs = require('util/parseArgs');
    const help = require('help');
    const metadata = require('/usr/share/help/jsh/rm');
    const pwd = process.env.get('PWD') || '/';

    const argv = normalizeArgs(process.argv.slice(2));
    const { values, positionals } = parseArgs(argv, metadata);

    if (values.help) {
        console.println(help.format(metadata));
        process.exit(0);
    }

    if ((!positionals || positionals.length === 0) && !values.force) {
        console.println('rm: missing operand');
        console.println("Try 'rm --help' for more information.");
        process.exit(1);
    }

    try {
        for (const targetPath of positionals || []) {
            removePath(targetPath, values);
        }
    } catch (err) {
        console.println(err.message);
        process.exit(1);
    }

    function removePath(targetPath, options) {
        const resolvedPath = resolvePath(targetPath);

        if (!fs.existsSync(resolvedPath)) {
            if (options.force) {
                return;
            }
            throw new Error(`rm: cannot remove '${targetPath}': No such file or directory`);
        }

        const stat = fs.statSync(resolvedPath);
        if (stat.isDirectory()) {
            if (options.recursive) {
                fs.rmSync(resolvedPath, { recursive: true, force: options.force });
            } else if (options.dir) {
                ensureEmptyDirectory(targetPath, resolvedPath);
                fs.rmdirSync(resolvedPath);
            } else {
                throw new Error(`rm: cannot remove '${targetPath}': Is a directory`);
            }
        } else {
            fs.rmSync(resolvedPath, { force: options.force });
        }

        if (options.verbose) {
            console.println(stat.isDirectory() ? `removed directory '${targetPath}'` : `removed '${targetPath}'`);
        }
    }

    function ensureEmptyDirectory(targetPath, resolvedPath) {
        const entries = fs.readdirSync(resolvedPath).filter((entry) => entry !== '.' && entry !== '..');
        if (entries.length > 0) {
            throw new Error(`rm: cannot remove '${targetPath}': Directory not empty`);
        }
    }

    function normalizeArgs(args) {
        return args.map((arg) => {
            if (arg === '-R') {
                return '-r';
            }
            if (arg === '--directory') {
                return '--dir';
            }
            return arg;
        });
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