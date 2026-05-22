(() => {
    'use strict';

    const process = require('process');
    const fs = require('fs');
    const path = require('path');
    const parseArgs = require('util/parseArgs');
    const crypto = require('crypto');

    const argv = process.argv.slice(2);
    const rootConfig = {
        usage: 'Usage: authkey <command> [options]',
        description: 'Generate auth key files for Machbase challenge authentication.',
    };

    if (argv.length === 0 || argv[0] === '-h' || argv[0] === '--help') {
        console.println(`${rootConfig.usage}\n\n${rootConfig.description}`);
        console.println('');
        console.println('Commands:');
        console.println('  gen      Generate key files (<output> and <output>.pub)');
        process.exit(0);
    }

    const command = argv[0];
    const subArgs = argv.slice(1);

    if (command !== 'gen') {
        console.println(`authkey: unknown command '${command}'`);
        console.println("Try 'authkey --help' for more information.");
        process.exit(1);
    }

    const genConfig = {
        usage: 'Usage: authkey gen -t [rsa|ecdsa] -o OUTPUT_PATH',
        description: 'Generate auth private/public key files.',
        options: {
            type: { type: 'string', short: 't', description: 'key type: rsa or ecdsa', default: 'ecdsa' },
            output: { type: 'string', short: 'o', description: 'output base path (prefix with @ for host OS path)', default: '' },
            help: { type: 'boolean', short: 'h', description: 'Show help', default: false },
        },
        allowPositionals: false,
        strict: false,
    };

    const { values } = parseArgs(subArgs, genConfig);

    if (values.help) {
        console.println(parseArgs.formatHelp(genConfig));
        process.exit(0);
    }

    const keyType = String(values.type || '').toLowerCase();
    if (keyType !== 'rsa' && keyType !== 'ecdsa') {
        console.println(`authkey gen: invalid key type '${values.type}', expected rsa or ecdsa`);
        process.exit(1);
    }

    if (!values.output || String(values.output).trim() === '') {
        console.println('authkey gen: output path is required (-o)');
        process.exit(1);
    }

    const pair = crypto.generateAuthKeyPair(keyType);
    const output = String(values.output).trim();

    if (output.startsWith('@')) {
        const hostPath = output.slice(1);
        if (!hostPath) {
            console.println('authkey gen: invalid host output path');
            process.exit(1);
        }
        crypto.writeHostFile(hostPath, pair.privateKey, 0o600);
        crypto.writeHostFile(`${hostPath}.pub`, pair.publicKey, 0o644);
        console.println(`private key: @${hostPath}`);
        console.println(`public key : @${hostPath}.pub`);
        process.exit(0);
    }

    const resolvedPath = fs.resolveAbsPath(output);
    const parentDir = path.dirname(resolvedPath);
    if (parentDir && parentDir !== '.' && parentDir !== '/') {
        fs.mkdirSync(parentDir, { recursive: true });
    }
    fs.writeFileSync(resolvedPath, pair.privateKey, 'utf8');
    fs.writeFileSync(`${resolvedPath}.pub`, pair.publicKey, 'utf8');

    console.println(`private key: ${resolvedPath}`);
    console.println(`public key : ${resolvedPath}.pub`);
})();
