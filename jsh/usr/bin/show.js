'use strict';

const process = require('process');
const pretty = require('pretty');
const neoapi = require('/usr/lib/neoapi');
const machcli = require('machcli');
const { parseAndRun, newMachCliClient } = require('/usr/lib/opts');

const optionHelp = { type: 'boolean', short: 'h', description: 'Show this help message', default: false }

const defaultConfig = {
    usage: 'Usage: show <command> [options]',
    options: {
        help: optionHelp,
    }
};

const infoConfig = {
    func: showInfo,
    command: 'info',
    usage: 'show info',
    description: 'Display server information',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    }
};

const licenseConfig = {
    func: showLicense,
    command: 'license',
    usage: 'show license',
    description: 'Display license information',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    }
};

const portsConfig = {
    func: showPorts,
    command: 'ports',
    usage: 'show ports [service]',
    description: 'Display service ports configuration',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
    positionals: [
        { name: 'service', optional: true, description: 'Service name to filter' }
    ],
};

const usersConfig = {
    func: showUsers,
    command: 'users',
    usage: 'show users',
    description: 'List all database users',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const tablesConfig = {
    func: showTables,
    command: 'tables',
    usage: 'show tables [-a]',
    description: 'List tables',
    allowNegative: true,
    options: {
        help: optionHelp,
        all: { type: 'boolean', short: 'a', description: 'Show all hidden tables', default: false },
        ...pretty.TableArgOptions,
    },
};

const tableConfig = {
    func: showTable,
    command: 'table',
    usage: 'show table [-a] <table>',
    description: 'Show table schema and details',
    allowNegative: true,
    options: {
        help: optionHelp,
        all: { type: 'boolean', short: 'a', description: 'Show all hidden columns', default: false },
        ...pretty.TableArgOptions,
    },
    positionals: [
        { name: 'table', description: 'Table name' }
    ],
};

const metaTablesConfig = {
    func: showMetaTables,
    command: 'meta-tables',
    usage: 'show meta-tables',
    description: 'List meta/system tables',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const virtualTablesConfig = {
    func: showVirtualTables,
    command: 'virtual-tables',
    usage: 'show virtual-tables',
    description: 'List virtual tables',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const sessionsConfig = {
    func: showSessions,
    command: 'sessions',
    usage: 'show sessions',
    description: 'List active database sessions',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const statementsConfig = {
    func: showStatements,
    command: 'statements',
    usage: 'show statements',
    description: 'List currently running SQL statements',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const indexesConfig = {
    func: showIndexes,
    command: 'indexes',
    usage: 'show indexes',
    description: 'List all indexes',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const indexConfig = {
    func: showIndex,
    command: 'index',
    usage: 'show index <index>',
    description: 'Show index structure and details',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
    positionals: [
        { name: 'index', description: 'Index name' }
    ],
};

const storageConfig = {
    func: showStorage,
    command: 'storage',
    usage: 'show storage',
    description: 'Show storage statistics',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const tableUsageConfig = {
    func: showTableUsage,
    command: 'table-usage',
    usage: 'show table-usage',
    description: 'Show storage usage by table',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const lsmConfig = {
    func: showLsm,
    command: 'lsm',
    usage: 'show lsm',
    description: 'Show LSM index status',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const indexgapConfig = {
    func: showIndexGap,
    command: 'indexgap',
    usage: 'show indexgap',
    description: 'Show index gap information',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const rollupgapConfig = {
    func: showRollupGap,
    command: 'rollupgap',
    usage: 'show rollupgap',
    description: 'Show rollup gap information',
    allowNegative: true,
    options: {
        help: optionHelp,
        long: { type: 'boolean', short: 'l', description: 'Show running state', default: false },
        ...pretty.TableArgOptions,
    },
};

const tagindexgapConfig = {
    func: showTagIndexGap,
    command: 'tagindexgap',
    usage: 'show tagindexgap',
    description: 'Show tag index gap information',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const tagsConfig = {
    func: showTags,
    command: 'tags',
    usage: 'show tags <table> [tag...]',
    description: 'List all/specific tags in the specified table',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
    positionals: [
        { name: 'table', description: 'Table name' },
        { name: 'tag', variadic: true, optional: true, description: 'Tag names' }
    ],
};

const tagstatConfig = {
    func: showTagStat,
    command: 'tagstat',
    usage: 'show tagstat <table> [tag...]',
    description: 'Show statistics for the specific tags',
    allowNegative: true,
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
    positionals: [
        { name: 'table', description: 'Table name' },
        { name: 'tag', variadic: true, optional: true, description: 'Tag names' }
    ],
};

parseAndRun(process.argv.slice(2), defaultConfig, [
    infoConfig,
    licenseConfig,
    portsConfig,
    usersConfig,
    tablesConfig,
    tableConfig,
    metaTablesConfig,
    virtualTablesConfig,
    sessionsConfig,
    statementsConfig,
    indexesConfig,
    indexConfig,
    storageConfig,
    tableUsageConfig,
    lsmConfig,
    indexgapConfig,
    rollupgapConfig,
    tagindexgapConfig,
    tagsConfig,
    tagstatConfig,
]);

function _show(line, config) {
    const client = new neoapi.Client(config);
    client.executeTql(`
            SQL('show ${line}')
            JSON(timeformat('DATETIME'), tz('Local'))
        `)
        .then((rsp) => {
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
    _show('users', config);
}

function showTables(config, args) {
    _show(`tables ${config.all ? '--all' : ''}`, config);
}

function showTable(config, args) {
    _show(`table ${config.all ? '--all' : ''} ${args.table}`, config);
}


function showMetaTables(config, args) {
    _show(`meta-tables`, config);
}

function showVirtualTables(config, args) {
    _show(`virtual-tables`, config);
}

function showSessions(config, args) {
    _show('sessions', config);
}

function showStatements(config, args) {
    _show('statements', config);
}

function showIndexes(config, args) {
    _show('indexes', config);
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
    _show('storage', config);
}

function showTableUsage(config, args) {
    config.columns = {
        'TABLE_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'STORAGE_USAGE': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Bytes(v) }
    };
    _show('table-usage', config);
}

function showLsm(config, args) {
    _show('lsm', config);
}

function showIndexGap(config, args) {
    config.columns = {
        'INDEX_ID': { align: pretty.Align.right, alignHeader: pretty.Align.left },
        'TABLE_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'INDEX_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'GAP': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Ints(v) },
    };
    _show('indexgap', config);
}

function showTagIndexGap(config, args) {
    config.columns = {
        'TABLE_ID': { align: pretty.Align.right, alignHeader: pretty.Align.left },
        'TABLE_NAME': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'STATUS': { align: pretty.Align.left, alignHeader: pretty.Align.left },
        'DISK_GAP': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Ints(v) },
        'MEMORY_GAP': { align: pretty.Align.right, alignHeader: pretty.Align.left, formatter: (v) => pretty.Ints(v) },
    };
    _show('tagindexgap', config);
}

function showRollupGap(config, args) {
    const elapsedFormatter = (v) => {
        let dur = pretty.Durations(v * 1e6); // convert milliseconds to nanoseconds
        if (dur === '0ns') dur = '0ms'; // since last_elapased_msec is in milliseconds
        return dur;
    }
    const lasttimeFormatter = (v) => {
        if (!v) return '';
        let d = new Date(v);
        if (d.getTime() == 0) return '';
        return d
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
        'LAST_WAKEUP_TIME': { align: pretty.Align.left, alignHeader: pretty.Align.left, hidden: !config.long, formatter: lasttimeFormatter },  // LAST_WAKEUP_TIME
        'NEXT_WAKEUP_TIME': { align: pretty.Align.left, alignHeader: pretty.Align.left, hidden: !config.long, formatter: lasttimeFormatter },  // NEXT_WAKEUP_TIME
    };
    _show('rollupgap', config);
}

function showTagStat(config, args) {
    showTags(config, args);
}

function showTags(config, args) {
    if (!args || !args.table || args.table.trim() === '') {
        console.println('Error: table name is required');
        process.exit(1);
    }
    const tableName = args.table;
    const tags = args.tag && args.tag.length > 0 ? args.tag : [];

    let db, conn, tagsRows;
    try {
        db = newMachCliClient(config);
        conn = db.connect();

        let names = db.normalizeTableName(tableName);

        let dbId = machcli.queryDatabaseId(conn, names[0]);
        let tableType = machcli.queryTableType(conn, names);
        if (tableType !== machcli.TABLE_TYPE_TAG) {
            console.println(`Error: table '${tableName}' is not a tag table`);
            process.exit(1);
        }
        let meta = {
            hasSummarized: false,
            tagNameColumn: 'NAME'
        };

        let result = conn.queryRow(`SELECT
                j.ID as TABLE_ID,
                j.TYPE as TABLE_TYPE,
                j.FLAG as TABLE_FLAG,
                j.COLCOUNT as TABLE_COLCOUNT,
                c.NAME as TAG_COLUMN_NAME
            FROM
                M$SYS_USERS u,
                M$SYS_TABLES j,
                M$SYS_COLUMNS c
            WHERE
                u.NAME = ?
            AND j.USER_ID = u.USER_ID
            AND j.DATABASE_ID = ?
            AND j.NAME = ?
            AND c.DATABASE_ID = ?
            AND c.TABLE_ID = j.ID
            AND c.FLAG = ?`, names[1], dbId, names[2], dbId, machcli.ColumnFlag.TagName);

        if (result) {
            if ((result.TABLE_FLAG & machcli.TABLE_FLAG_SUMMARIZED) !== 0) {
                meta.hasSummarized = true;
            }
            meta.tagNameColumn = result.TAG_COLUMN_NAME;
        }

        let box = pretty.Table(config);
        box.setStringEscape(true);
        if (meta.hasSummarized) {
            box.appendHeader(["_ID", meta.tagNameColumn, "ROW_COUNT", "MIN_TIME", "MAX_TIME", "RECENT_ROW_TIME", "MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME"]);
            box.setColumnConfigs([
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // _ID
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // tag name
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // ROW_COUNT
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // MIN_TIME
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // MAX_TIME
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // RECENT_ROW_TIME
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // MIN_VALUE
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // MIN_VALUE_TIME
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // MAX_VALUE
                { align: pretty.Align.left, alignHeader: pretty.Align.left }]); // MAX_VALUE_TIME
        } else {
            box.appendHeader(["_ID", meta.tagNameColumn, "ROW_COUNT", "MIN_TIME", "MAX_TIME", "RECENT_ROW_TIME"]);
            box.setColumnConfigs([
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // _ID
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // tag name
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // ROW_COUNT
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // MIN_TIME
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // MAX_TIME
                { align: pretty.Align.left, alignHeader: pretty.Align.left }]); // RECENT_ROW_TIME
        }

        if (tags.length > 0) {
            tagsRows = conn.query(`SELECT _ID, ${meta.tagNameColumn} FROM ${names[0]}.${names[1]}._${names[2]}_META WHERE ${meta.tagNameColumn} IN (${tags.map(() => '?').join(',')})`, ...tags);
        } else {
            tagsRows = conn.query(`SELECT _ID, ${meta.tagNameColumn} FROM ${names[0]}.${names[1]}._${names[2]}_META`);
        }
        for (const row of tagsRows) {
            const tagName = row[meta.tagNameColumn];
            try {
                let stat = conn.queryRow(`SELECT
                    NAME, ROW_COUNT,
                    MIN_TIME, MAX_TIME,
                    MIN_VALUE, MIN_VALUE_TIME, MAX_VALUE, MAX_VALUE_TIME,
                    RECENT_ROW_TIME
                FROM
                    ${names[0]}.${names[1]}.V$${names[2]}_STAT
                WHERE NAME = ?`, tagName);

                if (meta.hasSummarized) {
                    box.append([
                        row._ID,
                        stat.NAME,
                        pretty.Ints(stat.ROW_COUNT),
                        stat.MIN_TIME,
                        stat.MAX_TIME,
                        stat.RECENT_ROW_TIME,
                        stat.MIN_VALUE,
                        stat.MIN_VALUE_TIME,
                        stat.MAX_VALUE,
                        stat.MAX_VALUE_TIME
                    ]);
                } else {
                    box.append([
                        row._ID,
                        stat.NAME,
                        pretty.Ints(stat.ROW_COUNT),
                        stat.MIN_TIME,
                        stat.MAX_TIME,
                        stat.RECENT_ROW_TIME,
                    ]);
                }
            } catch (err) {
                err && console.error("Debug: showTagStat:", err.message);
                // in case of no stats available for the tag
                // for example, the tag name is not a printable string, most likely broken data
                box.append([row._ID, row.NAME, null, null, null, null, null, null, null, null]);
            }
            if (box.requirePageRender()) {
                // render page
                console.println(box.render());
                // wait for user input to continue if pause is enabled
                if (!box.pauseAndWait()) {
                    break;
                }
            }
        }
        if (box.length() > 0) {
            console.println(box.render());
        }
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        tagsRows && tagsRows.close();
        conn && conn.close();
        db && db.close();
    }
}
