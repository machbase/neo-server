'use strict';

const { ReadLine } = require('readline');
const process = require('process');
const { splitFields } = require('util');
const env = process.env;

const actor = {};
if (!actor.user) {
    actor.user = env.get('NEOSHELL_USER');
    if (!actor.user) {
        actor.user = 'sys';
    } else {
        actor.user = actor.user.toLowerCase();
    }
}
if (!actor.password) {
    actor.password = env.get('NEOSHELL_PASSWORD');
    if (!actor.password) {
        actor.password = 'manager';
    }
}

actor.prompt = (lineno) => {
    if (lineno == 0) {
        let n = new Date();
        let date = n.getFullYear() + "-" + String(n.getMonth() + 1).padStart(2, '0') + "-" + String(n.getDate()).padStart(2, '0');
        let datetime = date + " " + n.toLocaleTimeString();
        return `\x1b[33m${actor.user} \x1b[32mmachbase-neo\x1b[0m \x1b[34m${datetime}\x1b[0m\n\x1b[31m>\x1b[0m `;
    } else {
        //return "\x1b[31m>\x1b[0m ";
        return "  ";
    }
};

const SQL_VERBS = new Set([
    'ALTER', 'BACKUP', 'CREATE', 'COMMIT', 'DELETE', 'DROP',
    'EXEC', 'GRANT', 'INSERT', 'MOUNT', 'REVOKE', 'ROLLBACK',
    'SAVEPOINT', 'SELECT', 'TRUNCATE', 'UNMOUNT', 'UPDATE',
]);

actor.submitOnEnterWhen = (lines, idx) => {
    let maybe = lines.join('').trim().toLowerCase();
    if (maybe === 'exit' || maybe === 'quit' || maybe === 'help') {
        return true;
    }
    if (lines.length == 1 && (maybe == "" || maybe.startsWith('\\'))) {
        return true;
    }
    return lines[idx].endsWith(";");
};

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

actor.process = (line) => {
    const orgLine = line; // keep original line for history

    line = line.trim(); // trim whitespace
    line = line.replace(/;+\s*$/g, ''); // remove trailing semicolons
    line = line.trim(); // trim whitespace
    if (line.toLowerCase() === 'exit' || line.toLowerCase() === 'quit') {
        process.exit(0);
    }
    else if (line.toLowerCase() === 'clear') {
        console.print('\x1b[2J\x1b[H');
        return;
    }

    if (actor.addHistory) {
        actor.addHistory(orgLine);
    }

    try {
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

        // Handle backslash commands
        if (firstField === '\\') {
            // Execute jsh shell
            process.exec("/sbin/shell.js", ...fields);
            return;
        }

        if (firstField.startsWith('\\')) {
            // Execute js command (backslash prefix without semicolon)
            const command = firstField.substring(1);
            const args = fields.slice(1);
            process.exec(command, ...args);
            return;
        }

        // Execute regular js commands (with semicolon)
        const args = fields.slice(1);
        process.exec(firstField, ...args);
    } catch (e) {
        console.println("Process:", e.message);
    }
};

const r = new ReadLine({
    history: 'neo-shell-history',
    prompt: actor.prompt,
    submitOnEnterWhen: actor.submitOnEnterWhen,
});

actor.addHistory = (line) => {
    try {
        r.addHistory(line);
    } catch (e) {
        console.println("AddHistory:", e.message);
    }
};

while (true) {
    try {
        let line = r.readLine();
        if (line instanceof Error) {
            throw line;
        }
        if (line === "" || line === null) {
            continue;
        }
        actor.process(line);
    } catch (e) {
        console.println(e.message);
    }
}
