'use strict';

// Registry of shell commands that mutate the current execution context
// (user/password, server, current database) in-process instead of being
// delegated to a child process via process.exec(). Both the batch runner
// (usr/bin/run.js) and the interactive shell (usr/bin/neo-shell.js) consult
// this registry before falling back to process.exec(), so that a single
// implementation is shared between both execution modes. usr/bin/help.js also
// consults it so "help connect"/"help use" don't need a standalone executable.
const session = require('@jsh/session');

const commands = {
    connect: {
        run: connectHandler,
        description: 'Connect to a database',
        usage: 'Usage: connect [options] [user:password@]host[:port]',
    },
    use: {
        run: useHandler,
        description: 'Select the current database',
        usage: 'Usage: use <database>',
    },
};

// tryHandle(fields, env) -> number|null
// Returns an exit code if `fields` matched a context command (handled in-process),
// or null if the caller should fall back to process.exec().
function tryHandle(fields, env) {
    if (!fields || fields.length === 0) {
        return null;
    }
    const command = commands[fields[0].toLowerCase()];
    if (!command) {
        return null;
    }
    return command.run(fields.slice(1), env);
}

// printHelp(name) -> boolean
// Prints the usage text for a context command and returns true, or returns
// false if `name` is not a context command (caller should look elsewhere).
function printHelp(name) {
    const command = commands[name && name.toLowerCase()];
    if (!command) {
        return false;
    }
    console.println(command.usage);
    return true;
}

// describeAll() -> [{ name, description }]
// Used by help.js to list context commands alongside other commands.
function describeAll() {
    return Object.keys(commands).map((name) => ({ name, description: commands[name].description }));
}

// parseConnection(connection, env) -> { host, user, password, hostChanged }
// Mirrors the `connect [user:password@]host[:port]` / `connect user/password` syntax.
// Any part that is not given falls back to the current session's env values.
function parseConnection(connection, env) {
    let host = env.get('NEOSHELL_HOST');
    let user = env.get('NEOSHELL_USER') || 'sys';
    let password = env.get('NEOSHELL_PASSWORD') || 'manager';
    let hostChanged = false;

    if (connection && connection.length > 0) {
        const atIdx = connection.indexOf('@');
        const slashIdx = connection.indexOf('/');
        if (atIdx > 0) {
            const authPart = connection.substring(0, atIdx);
            const hostPart = connection.substring(atIdx + 1);
            const colonIdx = authPart.indexOf(':');
            if (colonIdx > 0) {
                user = authPart.substring(0, colonIdx);
                password = authPart.substring(colonIdx + 1);
            } else {
                user = authPart;
            }
            if (hostPart && hostPart.length > 0) {
                host = hostPart;
                hostChanged = true;
            }
        } else if (slashIdx > 0) {
            user = connection.substring(0, slashIdx);
            password = connection.substring(slashIdx + 1);
        } else {
            host = connection;
            hostChanged = true;
        }
    }
    return { host, user, password, hostChanged };
}

// connectHandler(args, env) -> number
// `connect [user:password@]host[:port]` or `connect user/password`.
// Switches the current process's session in-place: a plain user/password change
// reuses the existing mach host (session.switchUser), while a host change
// re-runs the full entry-time discovery (session.reconnect).
function connectHandler(args, env) {
    if (args.includes('-h') || args.includes('--help')) {
        console.println(commands.connect.usage);
        return 0;
    }
    const connection = args[0] || '';
    const { host, user, password, hostChanged } = parseConnection(connection, env);
    try {
        const err = hostChanged ? session.reconnect(host, user, password) : session.switchUser(user, password);
        if (err !== undefined) {
            console.println("Error: failed to connect:", err);
            return 1;
        }
    } catch (e) {
        console.println("Error: failed to connect:", e.message);
        return 1;
    }
    if (hostChanged) {
        env.set('NEOSHELL_HOST', host);
    }
    env.set('NEOSHELL_USER', user);
    env.set('NEOSHELL_PASSWORD', password);
    return 0;
}

// useHandler(args, env) -> number
// `use <database>`. Selects the machbase database that subsequent mach
// connections (in this process and its child processes) will open against.
function useHandler(args, env) {
    if (args && (args[0] === '-h' || args[0] === '--help')) {
        console.println(commands.use.usage);
        return 0;
    }
    if (!args || args.length === 0) {
        console.println(commands.use.usage);
        return 1;
    }
    const database = args[0];
    session.useDatabase(database);
    env.set('NEOSHELL_DATABASE', database);
    return 0;
}

module.exports = { tryHandle, printHelp, describeAll };
