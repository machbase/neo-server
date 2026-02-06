'use strict';

const process = require('process');
const neoapi = require('/usr/lib/neoapi');
const machcli = require('/usr/lib/machcli');
const pretty = require('/usr/lib/pretty');
const { parseAndRun } = require('/usr/lib/opts');

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
        conn: { type: 'integer', description: 'Maximum number of concurrent sessions' },
        query: { type: 'integer', description: 'Maximum number of the concurrent queries' },
        pool: { type: 'integer', description: 'Maximum number of the connection pool size' },
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
            let box = pretty.Table(config);
            box.setTitle('V$NEO_SESSION');
            box.appendHeader(["ID", "USER_NAME", "USER_ID", "STMT_COUNT", "CREATED", "LAST", "LAST SQL"]);
            try {
                db = new machcli.Client(config);
                conn = db.connect();
                neoRows = conn.query(`SELECT ID, USER_NAME, USER_ID, STMT_COUNT FROM V$NEO_SESSION`);
                for (row of neoRows) {
                    let o = sess[row.ID];
                    if (o) {
                        let created = new Date(o.creTime).toLocaleString();
                        let last = new Date(o.latestSqlTime).toLocaleString();
                        box.append([
                            row.ID,
                            row.USER_NAME,
                            row.USER_ID,
                            row.STMT_COUNT,
                            created,
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

                box = pretty.Table(config);
                box.setTitle('V$SESSION');
                box.appendHeader(["ID", "USER_NAME", "USER_ID", "CREATED", "TYPE", "USER_IP"]);
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
                console.println(box.render());
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
            let connUse = statz.conns;
            let connWaitTimePerUse = 0;
            let connUseTimePerUse = 0;
            if (connUse > 0) {
                connWaitTimePerUse = statz.connWaitTime / connUse;
                connUseTimePerUse = statz.connUseTime / connUse;
            }
            let box = pretty.Table(config);
            box.appendHeader(["METRIC", "VALUE"]);
            box.setColumnConfigs([
                { align: pretty.Align.left, alignHeader: pretty.Align.left },
                { align: pretty.Align.right, alignHeader: pretty.Align.left }]);
            box.append(["CONNS", pretty.Ints(statz.connsInUse)]);
            box.append(["CONNS_USED", pretty.Ints(statz.conns)]);
            box.append(["CONNS_WAIT_AVG", pretty.Durations(connWaitTimePerUse)]);
            box.append(["CONNS_USE_AVG", pretty.Durations(connUseTimePerUse)]);
            box.append(["STMTS", pretty.Ints(statz.stmtsInUse)]);
            box.append(["STMTS_USED", pretty.Ints(statz.stmts)]);
            box.append(["APPENDERS", pretty.Ints(statz.appendersInUse)]);
            box.append(["APPENDERS_USED", pretty.Ints(statz.appenders)]);
            box.append(["RAW_CONNS", pretty.Ints(statz.rawConns)]);
            box.append(["QUERY_EXEC_HWM", pretty.Durations(statz.queryExecHwm)])
            box.append(["QUERY_EXEC_AVG", pretty.Durations(statz.queryExecAvg)]);
            box.append(["QUERY_WAIT_HWM", pretty.Durations(statz.queryWaitHwm)]);
            box.append(["QUERY_WAIT_AVG", pretty.Durations(statz.queryWaitAvg)]);
            box.append(["QUERY_FETCH_HWM", pretty.Durations(statz.queryFetchHwm)]);
            box.append(["QUERY_FETCH_AVG", pretty.Durations(statz.queryFetchAvg)]);
            box.append(["QUERY_HWM", pretty.Durations(statz.queryHwm)]);
            box.append(["QUERY_HWM_EXEC", pretty.Durations(statz.queryHwmExec)]);
            box.append(["QUERY_HWM_WAIT", pretty.Durations(statz.queryHwmWait)]);
            box.append(["QUERY_HWM_FETCH", pretty.Durations(statz.queryHwmFetch)]);
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
            box.append(["POOL SIZE", numberOrUnlimited(limits.MaxPoolSize)]);
            box.append(["CONN LIMIT", numberOrUnlimited(limits.MaxOpenConn)]);
            box.append(["CONN REMAINS", numberOrUnlimited(limits.RemainedOpenConn)]);
            box.append(["QUERY LIMIT", numberOrUnlimited(limits.MaxOpenQuery)]);
            box.append(["QUERY REMAINS", numberOrUnlimited(limits.RemainedOpenQuery)]);
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function doSetLimit(config, args) {
    let limits = {};
    if (config.conn !== undefined) {
        limits.MaxOpenConn = config.conn;
    }
    if (config.query !== undefined) {
        limits.MaxOpenQuery = config.query;
    }
    if (config.pool !== undefined) {
        limits.MaxPoolSize = config.pool;
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