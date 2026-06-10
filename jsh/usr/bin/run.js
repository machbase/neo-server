'use strict';

const fs = require('fs');
const process = require('process');
const parseArgs = require('util/parseArgs');
const { splitCmdLine, splitBatchLines } = require('/usr/lib/cmdline')
const { switchUser } = require('@jsh/session');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    stopOnError: { type: 'boolean', short: 'e', description: 'Stop executing if any statement returns a non-zero exit code', default: false },
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


if (!filename.startsWith("/") && !filename.startsWith("@")) {
    filename = process.cwd() + "/" + filename;
}

try {
    const content = fs.readFile(filename);
    const lines = splitBatchLines(content)
    runSqlStatements(lines, config.stopOnError);
} catch (err) {
    let errMsg = err.message || String(err);
    if (errMsg.includes(filename)) {
        console.println(errMsg);
    } else {
        console.println(`Error reading file '${filename}': ${errMsg}`);
    }
    process.exit(1);
}

function runSqlStatements(statements, stopOnError = false) {
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
            let exitCode = 0;
            if (fields[0].toLowerCase() === 'connect') {
                // connect user/password
                //
                // internal command it should change the current user context 
                // for following statements, so it won't be executed via "process.exec" 
                // but directly handled in this script
                let connection = fields[1] || '';
                const slashIdx = connection.indexOf('/');
                if (slashIdx <= 0) {
                    console.println("Error: invalid user/password format");
                    exitCode = 1;
                } else {
                    const user = connection.substring(0, slashIdx);
                    const password = connection.substring(slashIdx + 1);
                    try {
                        switchUser(user, password);
                        process.env.set('NEOSHELL_USER', user);
                        process.env.set('NEOSHELL_PASSWORD', password);
                    } catch (err) {
                        console.println("Error: failed to switch user");
                        console.println(err.message);
                        exitCode = 1;
                    }
                }
            } else {
                // Execute neo-shell commands
                exitCode = process.exec(fields[0].toLowerCase(), ...fields.slice(1));
            }
            if (exitCode !== 0 && stopOnError) {
                console.println(`Script exited with code ${exitCode}: ${stmt.text}`);
                process.exit(exitCode);
            }
            console.println();
        }
    } catch (err) {
        console.println(`Error executing statements: ${err.message}`);
    }
}
