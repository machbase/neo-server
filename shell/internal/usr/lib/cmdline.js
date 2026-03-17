'use strict';

const { splitFields } = require('util');

// Keep quoted strings intact for `sql`, `explain`, `bridge exec` and `bridge query` commands;
// otherwise, the SQL text may be parsed incorrectly.
// Example: explain select * from table where name='John Doe'
function sqlArgument(fields, line) {
    const firstField = fields[0];
    let firstFieldLower = firstField.toLowerCase();
    if (firstFieldLower.endsWith('.js')) {
        firstFieldLower = firstFieldLower.substring(0, firstFieldLower.length - 3);
    }
    if (firstFieldLower !== 'sql' &&
        firstFieldLower !== 'explain' &&
        firstFieldLower !== 'bridge') {
        return fields;
    }
    if (firstFieldLower === 'bridge') {
        // bridge exec <bridge-name> <sql-text>
        if (fields.length < 4) {
            return fields;
        }
        const secondFieldLower = fields[1].toLowerCase();
        if (secondFieldLower !== 'exec' && secondFieldLower !== 'query') {
            return fields;
        }
    }

    fields = fields.slice(1);
    // find sql verb in the line, and split the rest as sql args
    const lineUpper = line.toUpperCase();
    for (const verb of SQL_VERBS.values()) {
        const index = lineUpper.indexOf(verb);
        if (index < 0) {
            continue;
        }
        const sqlText = line.substring(index);
        // find and remove fields after the verb
        for (let i = 0; i < fields.length; i++) {
            if (fields[i].toUpperCase() === verb) {
                fields = [firstField, ...fields.slice(0, i), sqlText];
                return fields;
            }
        }
    }
    // if no sql verb found, join all args as a sql text.
    const sqlArgs = fields.join(' ');
    return [firstField, sqlArgs];
}

const SQL_VERBS = new Set([
    'ALTER', 'BACKUP', 'CREATE', 'COMMIT', 'DELETE', 'DROP',
    'EXEC', 'GRANT', 'INSERT', 'MOUNT', 'REVOKE', 'ROLLBACK',
    'SAVEPOINT', 'SELECT', 'TRUNCATE', 'UNMOUNT', 'UPDATE',
]);

function splitCmdLine(env, line) {
    let fields = splitFields(line);
    let firstField = fields[0];

    // Handle aliased commands
    const aliasedCommand = env.alias(firstField);
    if (aliasedCommand) {
        firstField = aliasedCommand[0]
        fields = [...aliasedCommand, ...fields.slice(1)];
    }

    // Handle SQL commands
    if (SQL_VERBS.has(firstField.toUpperCase())) {
        firstField = "sql";
        fields = [firstField, line]; // normalize to sql.js command
    }
    // Keep quoted strings intact for sql-text;
    fields = sqlArgument(fields, line);
    return fields;
}

// split multi-line batch file into individual commands,
// 1. skip empty lines and comments(starts with -- or #)
// 2. split by semicolon, but keep semicolons in quoted strings intact
// 3. the last line may not end with semicolon, but should still be treated as a command
// returns objects with text, isComment, beginLine and endLine properties
function splitBatchLines(content) {
    const lines = String(content).split(/\r?\n/);
    const commands = [];
    let currentCommand = '';
    let beginLine = 0;
    let endLine = 0;
    let inSingleQuote = false;
    let inDoubleQuote = false;

    function pushCommand() {
        const text = currentCommand.trim();
        if (text === '') {
            currentCommand = '';
            beginLine = 0;
            endLine = 0;
            return;
        }
        commands.push({
            text,
            isComment: false,
            beginLine,
            endLine,
        });
        currentCommand = '';
        beginLine = 0;
        endLine = 0;
    }

    for (let lineIndex = 0; lineIndex < lines.length; lineIndex++) {
        const lineNumber = lineIndex + 1;
        const line = lines[lineIndex];
        const trimmed = line.trim();
        if (!inSingleQuote && !inDoubleQuote &&
            (trimmed === '' || line.startsWith('--') || line.startsWith('#'))) {
            continue; // skip empty lines and comments
        }

        for (let i = 0; i < line.length; i++) {
            const char = line[i];
            const hasContent = /\S/.test(char);

            if (beginLine === 0 && hasContent) {
                beginLine = lineNumber;
            }
            if (hasContent) {
                endLine = lineNumber;
            }

            if (char === "'" && !inDoubleQuote) {
                if (inSingleQuote && line[i + 1] === "'") {
                    currentCommand += "''";
                    i++;
                    continue;
                }
                inSingleQuote = !inSingleQuote;
            } else if (char === '"' && !inSingleQuote) {
                if (inDoubleQuote && line[i + 1] === '"') {
                    currentCommand += '""';
                    i++;
                    continue;
                }
                inDoubleQuote = !inDoubleQuote;
            }

            currentCommand += char;

            if (char === ';' && !inSingleQuote && !inDoubleQuote) {
                pushCommand();
            }
        }

        if (currentCommand !== '') {
            currentCommand += '\n';
        }
    }

    pushCommand();
    return commands;
}

module.exports = {
    splitCmdLine,
    splitBatchLines,
}