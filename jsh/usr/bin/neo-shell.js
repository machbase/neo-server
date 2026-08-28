'use strict';

const { ReadLine } = require('readline');
const process = require('process');
const { splitCmdLine } = require('/usr/lib/cmdline');
const contextCommands = require('/usr/lib/context_cmds');
const { getCurrentDatabase } = require('@jsh/session');
const env = process.env;

const actor = {};

// resolveDisplayUser() reads the current user from env every time it is called
// (rather than caching it once) so that an in-process `connect` immediately
// updates the prompt.
function resolveDisplayUser() {
    let user = env.get('NEOSHELL_USER');
    if (!user) {
        return 'sys';
    }
    user = user.toLowerCase();
    // If user is `sys as <other>`, extract the proxy username part after "as"
    const asIndex = user.indexOf(' as ');
    if (asIndex !== -1) {
        user = user.substring(asIndex + 4).trim();
    }
    return user;
}

// resolveDisplayDatabase() mirrors resolveDisplayUser(): read live so an in-process
// `use <database>` immediately updates the prompt. Returns '' when no database was
// selected (i.e. the server's default database applies), so the prompt stays as-is.
function resolveDisplayDatabase() {
    const database = getCurrentDatabase();
    return database ? database.toLowerCase() : '';
}

actor.prompt = (lineno) => {
    if (lineno == 0) {
        let n = new Date();
        let date = n.getFullYear() + "-" + String(n.getMonth() + 1).padStart(2, '0') + "-" + String(n.getDate()).padStart(2, '0');
        let datetime = date + " " + n.toLocaleTimeString();
        const database = resolveDisplayDatabase();
        const who = database ? `${resolveDisplayUser()}\x1b[36m@${database}\x1b[33m` : resolveDisplayUser();
        return `\x1b[33m${who} \x1b[32mmachbase-neo\x1b[0m \x1b[34m${datetime}\x1b[0m\n\x1b[31m>\x1b[0m `;
    } else {
        //return "\x1b[31m>\x1b[0m ";
        return "  ";
    }
};

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
        const fields = splitCmdLine(env, line);
        const firstField = fields[0];

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
            if (contextCommands.tryHandle([command, ...args], env) !== null) {
                return;
            }
            process.exec(command.toLowerCase(), ...args);
            return;
        }

        // Context commands (connect/use) mutate the current process's session
        // in-place instead of being delegated to a child process.
        if (contextCommands.tryHandle(fields, env) !== null) {
            return;
        }

        // Execute regular js commands (with semicolon)
        const args = fields.slice(1);
        process.exec(firstField.toLowerCase(), ...args);
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
