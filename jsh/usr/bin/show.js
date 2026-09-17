'use strict';

const process = require('process');
const pretty = require('pretty');
const { parseAndRun, newMachCliClient } = require('/usr/lib/opts');
const help = require('/usr/share/help/neo-shell/show');

const commandFunctions = {
    info: showInfo,
    license: showLicense,
    ports: showPorts,
    users: showUsers,
    databases: showDatabases,
    tables: showTables,
    table: showTable,
    'meta-tables': showMetaTables,
    'virtual-tables': showVirtualTables,
    sessions: showSessions,
    statements: showStatements,
    indexes: showIndexes,
    index: showIndex,
    storage: showStorage,
    'table-usage': showTableUsage,
    lsm: showLsm,
    indexgap: showIndexGap,
    rollupgap: showRollupGap,
    tagindexgap: showTagIndexGap,
    tags: showTags,
    tagstat: showTagStat,
};

const commandConfigs = Object.keys(help.commands).map((name) => ({
    ...help.commands[name],
    command: name,
    func: commandFunctions[name],
}));

parseAndRun(process.argv.slice(2), help, commandConfigs);

function _show(line, config) {
    const neoapi = require('/usr/lib/neoapi');
    const timeformat = config.timeformat || 'DATETIME';
    const tz = config.tz || 'Local';
    const client = new neoapi.Client(config);
    const showText = String(line).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    // executeTql() runs on the server over HTTP, independent of this process's mach
    // DSN, so the shell's `use <database>` selection must be forwarded explicitly.
    const { getCurrentDatabase } = require('@jsh/session');
    const database = getCurrentDatabase();
    const useArg = database ? `use('${database}'), ` : '';
    client.executeTql(`
            SQL(${useArg}'show ${showText}')
            JSON(timeformat('${timeformat}'), tz('${tz}'))
        `)
        .then((rsp) => {
            // Defense in depth: executeTql() rejects on {success:false} bodies, but a
            // response whose Content-Type carries a charset suffix (e.g. the IsDbSink
            // path) still resolves here instead of parsing as JSON, so surface the
            // server-reported reason before falling back to the generic message.
            if (rsp && rsp.success === false) {
                console.println('Error:', rsp.reason || 'TQL execution failed');
                return;
            }
            if (!rsp || !rsp.data || !rsp.data.rows) {
                console.println('Invalid response from server');
                return;
            }
            let box = pretty.Table(config);
            let columns = [];
            let visibility = [];
            let types = [];
            let cfgs = [];
            let fmts = [];
            rsp.data.columns.forEach((col, colIdx) => {
                if (config.columns && config.columns[col]) {
                    let c = config.columns[col];
                    if (c.hidden) {
                        visibility.push(false);
                        return;
                    }
                    columns.push(col);
                    types.push(rsp.data.types[colIdx]);
                    visibility.push(true);
                    cfgs.push({ align: c.align, alignHeader: c.alignHeader });
                    if (c.formatter) {
                        fmts.push(c.formatter);
                    } else {
                        fmts.push((v) => v);
                    }
                } else {
                    columns.push(col);
                    types.push(rsp.data.types[colIdx]);
                    visibility.push(true);
                    cfgs.push({});
                    fmts.push((v) => v);
                }
            });
            box.appendHeader(columns);
            box.setColumnTypes(types);
            box.setColumnConfigs(cfgs);
            rsp.data.rows.forEach((row) => {
                row = row.filter((v, j) => visibility[j]);
                let values = row.map((v, j) => fmts[j](v));
                box.appendRow(values);
            });
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function showInfo(config, args) {
    _show('info', config);
}

function showLicense(config, args) {
    _show('license', config);
}

function showPorts(config, args) {
    _show(`ports ${args.service ? args.service : ''}`, config);
}

function showUsers(config, args) {
    _show(`users${showClauses(args.clause)}`, config);
}

function showDatabases(config, args) {
    _show(`databases${showClauses(args.clause)}`, config);
}

function showTables(config, args) {
    _show(`tables${showClauses(args.clause)}${config.all ? ' WITH ALL' : ''}`, config);
}

function showTable(config, args) {
    _show(`table ${config.all ? '--all' : ''} ${args.table}`, config);
}

function showMetaTables(config, args) {
    _show(`meta-tables${showClauses(args.clause)}`, config);
}

function showVirtualTables(config, args) {
    _show(`virtual-tables${showClauses(args.clause)}`, config);
}

function showSessions(config, args) {
    _show(`sessions${showClauses(args.clause)}`, config);
}

function showStatements(config, args) {
    let queryFormat;
    if (config.long) {
        queryFormat = (v) => v;
    } else {
        queryFormat = (v) => {
            if (v == null) return '';
            let s = String(v).replace(/\s+/g, ' ').trim();
            if (s.length === 0) return '';

            const maxLen = 72;
            if (s.length <= maxLen) return s;

            let cut = s.slice(0, maxLen);
            const lastSpace = cut.lastIndexOf(' ');
            if (lastSpace > 0) {
                cut = cut.slice(0, lastSpace);
            }
            return cut + '...';
        }
    }
    config.columns = {
        'RECORD_SIZE': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Bytes(v) },
        'QUERY': { align: pretty.Align.left, alignHeader: pretty.Align.left, formatter: queryFormat },
    }
    _show(`statements${showClauses(args.clause)}`, config);
}

function showIndexes(config, args) {
    _show(`indexes${showClauses(args.clause)}`, config);
}

function showClauses(clauses) {
    if (!clauses || clauses.length === 0) return '';
    const output = [];
    for (let index = 0; index < clauses.length; index++) {
        const clause = String(clauses[index]);
        if (clause.toLowerCase() === 'like') {
            if (index + 1 >= clauses.length) {
                throw new Error('LIKE pattern must not be empty');
            }
            const pattern = String(clauses[++index]);
            if (pattern.length === 0) {
                throw new Error('LIKE pattern must not be empty');
            }
            output.push(`LIKE '${pattern.replace(/'/g, "''")}'`);
            continue;
        }
        output.push(clause);
    }
    return ` ${output.join(' ')}`;
}

function showIndex(config, args) {
    _show('index ' + args.index, config);
}

function showStorage(config, args) {
    config.columns = {
        'TABLE_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'DATA_SIZE': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Bytes(v) },
        'INDEX_SIZE': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Bytes(v) },
        'TOTAL_SIZE': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Bytes(v) }
    };
    _show(`storage${showClauses(args.clause)}`, config);
}

function showTableUsage(config, args) {
    config.columns = {
        'TABLE_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'STORAGE_USAGE': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Bytes(v) }
    };
    _show(`table-usage${showClauses(args.clause)}`, config);
}

function showLsm(config, args) {
    _show(`lsm${showClauses(args.clause)}`, config);
}

function showIndexGap(config, args) {
    config.columns = {
        'INDEX_ID': { align: pretty.Align.right, alignHeader: pretty.Align.left },
        'TABLE_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'INDEX_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'GAP': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Ints(v) },
    };
    _show(`indexgap${showClauses(args.clause)}`, config);
}

function showTagIndexGap(config, args) {
    config.columns = {
        'TABLE_ID': { align: pretty.Align.right, alignHeader: pretty.Align.left },
        'TABLE_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'STATUS': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'DISK_GAP': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Ints(v) },
        'MEMORY_GAP': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Ints(v) },
    };
    _show(`tagindexgap${showClauses(args.clause)}`, config);
}

function showRollupGap(config, args) {
    const elapsedFormatter = (v) => {
        let dur = pretty.Durations(v * 1e6); // convert milliseconds to nanoseconds
        if (dur === '0ns') dur = '0ms'; // since last_elapased_msec is in milliseconds
        return dur;
    }
    let lasttimeFormatter = (v) => {
        if (!v) return '';
        let d = new Date(v);
        if (d.getTime() == 0) return '';
        return d
    }
    if (config.timeformat) {
        let tf = config.timeformat.toUpperCase();
        if (tf !== 'DEFAULT' && tf !== 'DATETIME') {
            lasttimeFormatter = (v) => { return v; }
        }
    }

    config.columns = {
        'USER_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left, hidden: !config.long }, // USER_NAME
        'ROLLUP_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left }, // ROLLUP_NAME
        'SRC_TABLE': { align: pretty.Align.left, alignHeader: pretty.Align.left }, // SRC_TABLE
        'ROLLUP_TABLE': { align: pretty.Align.left, alignHeader: pretty.Align.left }, // ROLLUP_TABLE
        'SRC_END_RID': { align: pretty.Align.right, alignHeader: pretty.Align.left }, // SRC_END_RID
        'ROLLUP_END_RID': { align: pretty.Align.right, alignHeader: pretty.Align.left }, // ROLLUP_END_RID
        'GAP': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Ints(v) }, // GAP
        'RUN_STATE': { align: pretty.Align.left, alignHeader: pretty.Align.left, hidden: !config.long },  // STATE
        'LAST_ELAPSED_MSEC': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: elapsedFormatter }, // LAST_ELAPSED_MSEC
        'LAST_WAKEUP_TIME': { align: pretty.Align.right, alignHeader: pretty.Align.left, hidden: !config.long, formatter: lasttimeFormatter },  // LAST_WAKEUP_TIME
        'NEXT_WAKEUP_TIME': { align: pretty.Align.right, alignHeader: pretty.Align.left, hidden: !config.long, formatter: lasttimeFormatter },  // NEXT_WAKEUP_TIME
    };
    _show(`rollupgap${showClauses(args.clause)}`, config);
}

function showTagStat(config, args) {
    showTags(config, args);
}

function showTags(config, args) {
    config.columns = {
        'ROW_COUNT': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Ints(v) },
        'MIN_TIME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'MAX_TIME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'RECENT_ROW_TIME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
    };
    _show(`tags ${args.table}${showClauses(args.tag)}`, config);
}
