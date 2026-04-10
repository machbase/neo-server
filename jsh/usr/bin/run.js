'use strict';

const fs = require('fs');
const process = require('process');
const parseArgs = require('util/parseArgs');
const { splitCmdLine, splitBatchLines } = require('/usr/lib/cmdline')

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    verbose: { type: 'boolean', short: 'v', description: 'Enable verbose output', default: false },
}

const positionals = [
    { name: 'filename', type: 'string', description: 'script file path to run' }
];

let showHelp = true;
let config = {};
let filename = '';
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
        allowPositionals: true,
        allowNegative: true,
        positionals: positionals
    });
    config = parsed.values;
    filename = parsed.namedPositionals.filename;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp || (!filename) || filename.length === 0) {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: run [options] <filename>',
        options,
        positionals: positionals
    }));
    process.exit(showHelp ? 0 : 1);
}


if (!filename.startsWith("/")) {
    filename = process.cwd() + "/" + filename;
}

try {
    const content = fs.readFile(filename);
    const lines = splitBatchLines(content)
    runSqlStatements(lines);
} catch (err) {
    let errMsg = err.message || String(err);
    if (errMsg.includes(filename)) {
        console.println(errMsg);
    } else {
        console.println(`Error reading file '${filename}': ${errMsg}`);
    }
    process.exit(1);
}

function runSqlStatements(statements) {
    if (!statements || statements.length === 0) {
        console.println(`No SQL statements found in file '${filename}'.`);
        return;
    }
    try {
        for (let i = 0; i < statements.length; i++) {
            const stmt = statements[i];
            if (!stmt || stmt.isComment) {
                continue;
            }
            if (config.verbose) {
                let sqlText = stmt.text.split('\n').map(line => line.trim()).filter(line => line.length > 0).join(' ');
                console.println(`\n[${i + 1}/${statements.length}] Line ${stmt.beginLine}~${stmt.endLine}: ${sqlText}`);
            }
            stmt.text = stmt.text.replace(/;+\s*$/g, ''); // remove trailing semicolons
            console.println(stmt.text);

            let fields = splitCmdLine(process.env, stmt.text);
            if (!fields || fields.length === 0) {
                continue;
            }
            // Execute neo-shell commands
            process.exec(fields[0].toLowerCase(), ...fields.slice(1));
            console.println();
        }
    } catch (err) {
        console.println(`Error executing statements: ${err.message}`);
    }
}
