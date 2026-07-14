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
        long: { type: 'boolean', short: 'l', description: 'Show full SQL statements', default: false },
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
    const timeformat = config.timeformat || 'DATETIME';
    const tz = config.tz || 'Local';
    const client = new neoapi.Client(config);
    client.executeTql(`
            SQL('show ${line}')
            JSON(timeformat('${timeformat}'), tz('${tz}'))
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
    _show('rollupgap', config);
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
    _show(`tags ${args.table} ${args.tag && args.tag.length > 0 ? args.tag.join(' ') : ''}`, config);
}
