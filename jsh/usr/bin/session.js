'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const { parseAndRun, newMachCliClient } = require('/usr/lib/opts');

const optionHelp = { type: 'boolean', short: 'h', description: 'Show this help message', default: false }

const defaultConfig = {
    usage: 'Usage: session <command> [options]',
    options: {
        help: optionHelp,
    }
};

const listConfig = {
    func: doList,
    command: 'list',
    usage: 'session list',
    description: 'List all sessions',
    allowNegative: true,
    options: {
        help: optionHelp,
        all: { type: 'boolean', description: 'Include details' },
        ...pretty.TableArgOptions,
    }
}

const killConfig = {
    func: doKill,
    command: 'kill',
    usage: 'session kill <id>',
    description: 'Force to close the session by session ID',
    options: {
        help: optionHelp,
        force: { type: 'boolean', description: 'Force kill the session', default: false },
    },
    positionals: [
        { name: 'id', description: 'ID of the session to kill' },
    ],
}

const statConfig = {
    func: doStat,
    command: 'stat',
    usage: 'session stat [options]',
    description: 'Show detailed information about sessions',
    allowNegative: true,
    options: {
        help: optionHelp,
        reset: { type: 'boolean', description: 'Reset statistics after showing', default: false },
        ...pretty.TableArgOptions,
    }
}

const limitConfig = {
    func: doLimit,
    command: 'limit',
    usage: 'session limit',
    description: 'Get session limits',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    }
}

const setLimitConfig = {
    func: doSetLimit,
    command: 'set-limit',
    usage: 'session set-limit [options]',
    description: 'Set session limits',
    options: {
        help: optionHelp,
        maxOpenConn: { type: 'integer', description: 'Maximum number of open connections to the database' },
        maxIdleConn: { type: 'integer', description: 'Maximum number of idle connections to the database' },
        connMaxIdleTime: { type: 'string', description: 'Maximum idle time for a connection (e.g., "30s", "5m")' },
        connMaxLifetime: { type: 'string', description: 'Maximum lifetime for a connection (e.g., "1h", "24h")' },
    }
}

parseAndRun(process.argv.slice(2), defaultConfig, [
    listConfig,
    killConfig,
    statConfig,
    limitConfig,
    setLimitConfig,
]);

function doList(config, args) {
    const client = new neoapi.Client(config);
    client.listSessions()
        .then((lst) => {
            let db, conn, neoRows, sessRows;
            let sess = {};
            for (const s of lst) {
                sess[s.id] = s;
            }
            try {
                db = newMachCliClient(config);
                conn = db.connect();

                let box = pretty.Table(config);
                box.setTimeformat('DATETIME');
                box.appendHeader(["ID", "USER_NAME", "USER_ID", "LOGIN_TIME", "TYPE", "USER_IP"]);
                sessRows = conn.query(`SELECT ID, USER_NAME, USER_ID, USER_IP, CLIENT_TYPE, LOGIN_TIME FROM V$SESSION`);
                for (const row of sessRows) {
                    if (!sess[row.ID]) {
                        box.append([
                            row.ID,
                            row.USER_NAME,
                            row.USER_ID,
                            row.LOGIN_TIME,
                            row.CLIENT_TYPE,
                            row.USER_IP,
                        ]);
                    }
                }
                if (box.length() > 0) {
                    console.println(box.render());
                }

                box = pretty.Table(config);
                box.setTimeformat('DATETIME');
                box.appendHeader(["ID", "USER_NAME", "USER_ID", "STMT_COUNT", "LOGIN_TIME", "LAST", "LAST SQL"]);
                neoRows = conn.query(`SELECT ID, USER_NAME, USER_ID, STMT_COUNT FROM V$NEO_SESSION`);
                for (row of neoRows) {
                    let o = sess[row.ID];
                    if (o) {
                        let loginTime = new Date(o.loginTime).toLocaleString();
                        let last = new Date(o.latestSqlTime).toLocaleString();
                        box.append([
                            row.ID,
                            row.USER_NAME,
                            row.USER_ID,
                            row.STMT_COUNT,
                            loginTime,
                            last,
                            o.lastSQL ? o.lastSQL : '',
                        ]);
                    } else {
                        box.append([
                            row.ID,
                            row.USER_NAME,
                            row.USER_ID,
                            row.STMT_COUNT,
                            "",
                            "",
                            "",
                        ]);
                    }
                }
                if (box.length() > 0) {
                    console.println(box.render());
                }

            }
            catch (err) {
                console.println('Error:', err.message, err.stack);
            }
            finally {
                if (sessRows) {
                    sessRows.close();
                }
                if (neoRows) {
                    neoRows.close();
                }
                if (conn) {
                    conn.close();
                }
                if (db) {
                    db.close();
                }
            }
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function doKill(config, args) {
    const client = new neoapi.Client(config);
    const id = args.id;
    const force = args.force || false;
    client.killSession(id, force)
        .then(() => {
            console.println(`Session '${id}' cancelled`);
        })
        .catch((err) => {
            console.println(`Session '${id}', failed cancel:`, err.message);
        });
}

function doStat(config, args) {
    const client = new neoapi.Client(config);
    const reset = args.reset || false;
    client.statSession(reset)
        .then((statz) => {
            let box = pretty.Table(config);
            box.appendHeader(["METRIC", "VALUE"]);
            box.setColumnConfigs([
                { align: pretty.Align.left, alignHeader: pretty.Align.left },
                { align: pretty.Align.right, alignHeader: pretty.Align.left }]);
            box.append(["MAX OPEN CONN", pretty.Ints(statz.maxOpenConnections)]);
            box.append(["OPEN CONN", pretty.Ints(statz.openConnections)]);
            box.append(["IDLE", pretty.Ints(statz.idle)]);
            box.append(["IN USE", pretty.Ints(statz.inUse)]);
            box.append(["MAX IDLE CLOSED", pretty.Ints(statz.maxIdleClosed)]);
            box.append(["MAX IDLE TIME CLOSED", pretty.Ints(statz.maxIdleTimeClosed)]);
            box.append(["MAX LIFETIME CLOSED", pretty.Ints(statz.maxLifetimeClosed)]);
            box.append(["WAIT COUNT", pretty.Ints(statz.waitCount)]);
            box.append(["WAIT DURATION (AVG)", statz.waitAvgDuration]);
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function doLimit(config, args) {
    const client = new neoapi.Client(config);
    client.getSessionLimit()
        .then((limits) => {
            const box = pretty.Table(config);
            box.appendHeader(["NAME", "VALUE"]);
            box.setColumnConfigs([
                { align: pretty.Align.left, alignHeader: pretty.Align.left },
                { align: pretty.Align.right, alignHeader: pretty.Align.left }]);
            const numberOrUnlimited = (v) => {
                if (v < 0) {
                    return 'unlimited';
                }
                return pretty.Ints(v);
            }
            box.append(["MAX OPEN CONN", numberOrUnlimited(limits.maxOpenConn)]);
            box.append(["MAX IDLE CONN", numberOrUnlimited(limits.maxIdleConn)]);
            box.append(["CONN MAX IDLE TIME", pretty.Durations(limits.connMaxIdleTime)]);
            box.append(["CONN MAX LIFETIME", pretty.Durations(limits.connMaxLifetime)]);
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function doSetLimit(config, args) {
    let limits = {};
    if (config.maxOpenConn !== undefined) {
        limits.maxOpenConn = config.maxOpenConn;
    }
    if (config.maxIdleConn !== undefined) {
        limits.maxIdleConn = config.maxIdleConn;
    }
    if (config.connMaxIdleTime !== undefined) {
        limits.connMaxIdleTime = config.connMaxIdleTime;
    }
    if (config.connMaxLifetime !== undefined) {
        limits.connMaxLifetime = config.connMaxLifetime;
    }
    const client = new neoapi.Client(config);
    client.setSessionLimit(limits)
        .then(() => {
            console.println('Session limits updated successfully.');
        })
        .catch((err) => {
            console.println('Error updating session limits:', err.message);
        });
}