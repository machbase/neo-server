'use strict';

const { ReadLine } = require('readline');
const process = require('process');
const { splitCmdLine } = require('/usr/lib/cmdline');
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
