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
        if (secondFieldLower === 'exec' || secondFieldLower === 'query') {
            const bridgeName = fields[2];
            const sqlText = extractBridgeSqlText(line);
            if (!sqlText) {
                return fields;
            }
            return [firstFieldLower, secondFieldLower, bridgeName, sqlText];
        } else {
            return fields;
        }
    }

    fields = fields.slice(1);
    if (firstFieldLower === 'sql' && fields.length === 1 && fields[0] === line) {
        return [firstField, trimOuterMatchingQuotes(line.trim())];
    }
    // find the first SQL verb field and keep everything before it split like splitFields()
    for (let i = 0; i < fields.length; i++) {
        const upperField = fields[i].toUpperCase();
        for (const verb of SQL_VERBS.values()) {
            if (upperField !== verb && !upperField.startsWith(verb + ' ')) {
                continue;
            }
            const sqlText = extractSqlText(line, i + 1);
            return [firstField, ...fields.slice(0, i), sqlText];
        }
    }
    // if no sql verb found, join all args as a sql text.
    const sqlArgs = fields.join(' ');
    return [firstField, sqlArgs];
}

function extractSqlText(line, leadingFieldCount) {
    return trimOuterMatchingQuotes(skipFieldCount(line, leadingFieldCount).trim());
}

function extractBridgeSqlText(line) {
    const remainder = extractSqlText(line, 3);
    return remainder;
}

function skipFieldCount(line, fieldCount) {
    let index = 0;
    let consumed = 0;
    const text = String(line);

    while (index < text.length && consumed < fieldCount) {
        while (index < text.length && isWhitespace(text[index])) {
            index++;
        }
        if (index >= text.length) {
            return '';
        }
        index = skipField(text, index);
        consumed++;
    }

    return text.substring(index);
}

function skipField(text, index) {
    let inQuote = false;
    let quoteChar = '';

    while (index < text.length) {
        const char = text[index];
        if (inQuote) {
            if (char === quoteChar) {
                inQuote = false;
                quoteChar = '';
            }
            index++;
            continue;
        }
        if (char === '"' || char === "'") {
            inQuote = true;
            quoteChar = char;
            index++;
            continue;
        }
        if (isWhitespace(char)) {
            break;
        }
        index++;
    }

    return index;
}

function trimOuterMatchingQuotes(text) {
    if (text.length < 2) {
        return text;
    }
    const first = text[0];
    const last = text[text.length - 1];
    if ((first === '"' || first === "'") && first === last) {
        return text.substring(1, text.length - 1);
    }
    return text;
}

function isWhitespace(char) {
    return char === ' ' || char === '\t' || char === '\n' || char === '\r';
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