'use strict';

const process = require('process');
const parseArgs = require('util/parseArgs');
const neoapi = require('/usr/lib/neoapi');
const fs = require('fs');
const { splitFields } = require('util')

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
    filename = pwd + "/" + filename;
}

try {
    const content = fs.readFile(filename);
    const client = new neoapi.Client(config);
    client.splitSqlStatements(content)
        .then((result) => {
            runSqlStatements(result);
        })
        .catch((err) => {
            console.println(`Error connecting to server: ${err.message}`);
            process.exit(1);
        });
} catch (err) {
    console.println(`Error reading file '${filename}': ${err.message}`);
    process.exit(1);
}


const SQL_VERBS = new Set([
    'SELECT', 'INSERT', 'UPDATE', 'DELETE', 'CREATE', 'DROP', 'ALTER',
    'TRUNCATE', 'GRANT', 'REVOKE', 'COMMIT', 'ROLLBACK', 'SAVEPOINT',
    'BACKUP', "MOUNT",
]);

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
            let fields = splitFields(stmt.text);
            let firstField = fields[0];

            // Handle aliased commands
            const aliasedCommand = process.env.alias(firstField);
            if (aliasedCommand) {
                firstField = aliasedCommand[0]
                fields = [...aliasedCommand, ...fields.slice(1)];
            }

            console.println(stmt.text);
            if (SQL_VERBS.has(firstField.toUpperCase())) {
                // Handle SQL commands
                process.exec("sql.js", stmt.text);
            } else {
                // Handle other commands
                process.exec(firstField, ...fields.slice(1));
            }
            console.println(``);
        }
    } catch (err) {
        console.println(`Error executing statements: ${err.message}`);
    }
}
