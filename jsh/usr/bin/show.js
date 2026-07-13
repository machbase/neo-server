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
    description: 'Display storage statistics',
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
    description: 'Display LSM tree status',
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
            if (!rsp || !rsp.data || !rsp.data.rows || rsp.data.rows.length === 0) {
                console.println('No server info available');
                return;
            }
            const nfo = rsp.data.rows[0];
            let box = pretty.Table(config);
            box.appendHeader(rsp.data.columns);
            box.setColumnTypes(rsp.data.types);
            let cfgs = [];
            for (const typ of rsp.data.types) {
                cfgs.push({});
            }
            box.setColumnConfigs(cfgs);
            for (const row of rsp.data.rows) {
                box.appendRow(row);
            }
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
    if (!args.index) {
        console.println('Error: index name is required');
        process.exit(1);
    }
    const indexName = args.index;
    let db, conn, rows;
    try {
        db = newMachCliClient(config);
        conn = db.connect();
        rows = conn.query(`select 
            a.name as TABLE_NAME,
            c.name as COLUMN_NAME,
            b.name as INDEX_NAME,
            case b.type
                when 1 then 'BITMAP'
                when 2 then 'KEYWORD'
                when 5 then 'REDBLACK'
                when 6 then 'LSM'
                when 8 then 'REDBLACK'
                when 9 then 'KEYWORD_LSM'
                else 'LSM' end 
            as INDEX_TYPE,
            case b.key_compress
                when 0 then 'UNCOMPRESSED'
                else 'COMPRESSED' end 
            as KEY_COMPRESS,
            b.max_level as MAX_LEVEL,
            b.part_value_count as PART_VALUE_COUNT,
            case b.bitmap_encode
                when 0 then 'EQUAL'
                else 'RANGE' end 
            as BITMAP_ENCODE
        from
            m$sys_tables a,
            m$sys_indexes b,
            m$sys_index_columns c
        where
            a.id = b.table_id 
        and b.id = c.index_id
        and b.name = '${indexName.toUpperCase()}'`);

        let box = pretty.Table(config);
        box.appendHeader(["TABLE_NAME", "COLUMN_NAME", "INDEX_NAME", "INDEX_TYPE", "KEY_COMPRESS", "MAX_LEVEL", "PART_VALUE_COUNT", "BITMAP_ENCODE"]);
        for (const row of rows) {
            box.append([
                row.TABLE_NAME,
                row.COLUMN_NAME,
                row.INDEX_NAME,
                row.INDEX_TYPE,
                row.KEY_COMPRESS,
                row.MAX_LEVEL,
                row.PART_VALUE_COUNT,
                row.BITMAP_ENCODE
            ]);
        }
        console.println(box.render());
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        rows && rows.close();
        conn && conn.close();
        db && db.close();
    }
}

function showStorage(config, args) {
    let db, conn, rows;
    try {
        db = newMachCliClient(config);
        conn = db.connect();
        rows = conn.query(`select
            a.table_name as TABLE_NAME,
            a.data_size as DATA_SIZE,
            case b.index_size 
                when b.index_size then b.index_size 
                else 0 end 
            as INDEX_SIZE,
            case a.data_size + b.index_size 
                when a.data_size + b.index_size then a.data_size + b.index_size 
                else a.data_size end 
            as TOTAL_SIZE
        from
            (select
                a.name as table_name,
                sum(b.storage_usage) as data_size
            from
                m$sys_tables a,
                v$storage_tables b
            where a.id = b.id
            group by a.name
            ) as a LEFT OUTER JOIN
            (select
                a.name,
                sum(b.disk_file_size) as index_size
            from
                m$sys_tables a,
                v$storage_dc_table_indexes b
            where a.id = b.table_id
            group by a.name) as b
        on a.table_name = b.name
        order by a.table_name`);

        let box = pretty.Table(config);
        box.appendHeader(["TABLE_NAME", "DATA_SIZE", "INDEX_SIZE", "TOTAL_SIZE"]);
        box.setColumnConfigs([
            { align: pretty.Align.left, alignHeader: pretty.Align.left },
            { align: pretty.Align.right, alignHeader: pretty.Align.left },
            { align: pretty.Align.right, alignHeader: pretty.Align.left },
            { align: pretty.Align.right, alignHeader: pretty.Align.left }]);
        for (const row of rows) {
            box.append([
                row.TABLE_NAME,
                pretty.Bytes(row.DATA_SIZE),
                pretty.Bytes(row.INDEX_SIZE),
                pretty.Bytes(row.TOTAL_SIZE)
            ]);
        }
        console.println(box.render());
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        rows && rows.close();
        conn && conn.close();
        db && db.close();
    }
}

function showTableUsage(config, args) {
    let db, conn, rows;
    try {
        db = newMachCliClient(config);
        conn = db.connect();
        rows = conn.query(`SELECT
            a.NAME as TABLE_NAME,
            t.STORAGE_USAGE as STORAGE_USAGE
        FROM
            M$SYS_TABLES a,
            M$SYS_USERS u,
            V$STORAGE_TABLES t
        WHERE
            a.user_id = u.user_id
        AND t.ID = a.id
        ORDER BY a.NAME`);

        let box = pretty.Table(config);
        box.appendHeader(["TABLE_NAME", "STORAGE_USAGE"]);
        box.setColumnConfigs([
            { align: pretty.Align.left, alignHeader: pretty.Align.left },
            { align: pretty.Align.right, alignHeader: pretty.Align.left }]);

        for (const row of rows) {
            box.append([
                row.TABLE_NAME,
                pretty.Bytes(row.STORAGE_USAGE),
            ]);
        }
        console.println(box.render());
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        rows && rows.close();
        conn && conn.close();
        db && db.close();
    }
}

function showLsm(config, args) {
    let db, conn, rows;
    try {
        db = newMachCliClient(config);
        conn = db.connect();
        rows = conn.query(`select 
            b.name as TABLE_NAME,
            c.name as INDEX_NAME,
            a.level as LEVEL,
            a.end_rid - a.begin_rid as COUNT
        from
            v$storage_dc_lsmindex_levels a,
            m$sys_tables b, m$sys_indexes c
        where
            c.id = a.index_id 
        and b.id = a.table_id
        order by 1, 2, 3`);

        let box = pretty.Table(config);
        box.appendHeader(["TABLE_NAME", "INDEX_NAME", "LEVEL", "COUNT"]);
        for (const row of rows) {
            box.append([
                row.TABLE_NAME,
                row.INDEX_NAME,
                row.LEVEL,
                row.COUNT
            ]);
        }
        console.println(box.render());
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        rows && rows.close();
        conn && conn.close();
        db && db.close();
    }
}

function showIndexGap(config, args) {
    let db, conn, rows;
    try {
        db = newMachCliClient(config);
        conn = db.connect();
        rows = conn.query(`select
            c.id,
            b.name as TABLE_NAME, 
            c.name as INDEX_NAME, 
            a.table_end_rid - a.end_rid as GAP
        from
            v$storage_dc_table_indexes a,
            m$sys_tables b,
            m$sys_indexes c
        where
            a.id = c.id 
        and c.table_id = b.id 
        order by 3 desc`);

        let box = pretty.Table(config);
        box.appendHeader(["ID", "TABLE", "INDEX", "GAP"]);
        for (const row of rows) {
            box.append([
                row.id,
                row.TABLE_NAME,
                row.INDEX_NAME,
                row.GAP
            ]);
        }
        console.println(box.render());
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        rows && rows.close();
        conn && conn.close();
        db && db.close();
    }
}

function showTagIndexGap(config, args) {
    let db, conn, rows;
    try {
        db = newMachCliClient(config);
        conn = db.connect();
        rows = conn.query(`select 
            t.NAME AS TABLE_NAME,
            i.INDEX_STATE AS STATUS,
            i.TABLE_END_RID - i.DISK_INDEX_END_RID AS DISK_GAP,
            i.TABLE_END_RID - i.MEMORY_INDEX_END_RID AS MEMORY_GAP
        from
            M$SYS_TABLES t,
            V$STORAGE_TAG_INDEX i
        where
            t.ID = i.TABLE_ID
        order by id`);

        let box = pretty.Table(config);
        box.appendHeader(["TABLE_NAME", "STATUS", "DISK_GAP", "MEMORY_GAP"]); // tag table
        box.setColumnConfigs([
            { align: pretty.Align.left, alignHeader: pretty.Align.left },
            { align: pretty.Align.left, alignHeader: pretty.Align.left },
            { align: pretty.Align.right, alignHeader: pretty.Align.left },
            { align: pretty.Align.right, alignHeader: pretty.Align.left }]);

        for (const row of rows) {
            box.append([
                row.TABLE_NAME,
                row.STATUS,
                pretty.Ints(row.DISK_GAP),
                pretty.Ints(row.MEMORY_GAP)
            ]);
        }
        console.println(box.render());
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        rows && rows.close();
        conn && conn.close();
        db && db.close();
    }
}

function showRollupGap(config, args) {
    let db, conn;

    try {
        db = newMachCliClient(config);
        conn = db.connect();
        // check if DATABASE_ID column exists in V$ROLLUP
        let r = conn.queryRow(`SELECT count(*) AS CNT
            FROM
                V$TABLES t,
                V$COLUMNS c
            WHERE
                t.NAME = 'V$ROLLUP'
            AND c.TABLE_ID = t.ID
            AND c.NAME = 'DATABASE_ID'`)
        if (r.CNT === 0) {
            // neo version < 8.0.60 (19 Sep 2025) does not have DATABASE_ID column in V$ROLLUP
            showRollupGap_prev_8_0_60(config, conn);
        } else {
            showRollupGap_since_8_0_60(config, conn);
        }
    } finally {
        conn && conn.close();
        db && db.close();
    }
}

function showRollupGap_prev_8_0_60(config, conn) {
    let rows;
    try {
        rows = conn.query(`SELECT
            C.SOURCE_TABLE AS SRC_TABLE,
            C.ROLLUP_TABLE,
            B.TABLE_END_RID AS SRC_END_RID,
            C.END_RID AS ROLLUP_END_RID,
            B.TABLE_END_RID - C.END_RID AS GAP,
            C.LAST_ELAPSED_MSEC AS LAST_ELAPSED
        FROM
            M$SYS_TABLES A,
            V$STORAGE_TAG_TABLES B,
            V$ROLLUP C
        WHERE
            A.ID=B.ID
        AND A.NAME=C.SOURCE_TABLE
        ORDER BY SRC_TABLE`);

        let box = pretty.Table(config);
        box.appendHeader(["SRC_TABLE", "ROLLUP_TABLE", "SRC_END_RID", "ROLLUP_END_RID", "GAP", "LAST_ELAPSED"]);
        for (const row of rows) {
            box.append([
                row.SRC_TABLE,
                row.ROLLUP_TABLE,
                row.SRC_END_RID,
                row.ROLLUP_END_RID,
                pretty.Ints(row.GAP),
                pretty.Durations(row.LAST_ELAPSED * 1e6)
            ]);
        }
        console.println(box.render());
    } finally {
        rows && rows.close();
    }
}

function showRollupGap_since_8_0_60(config, conn) {
    let rows;
    try {
        rows = conn.query(`SELECT
            U.NAME AS USER_NAME,
            C.ROLLUP_NAME AS ROLLUP_NAME,
            C.SOURCE_TABLE AS SRC_TABLE,
            C.ROLLUP_TABLE,
            B.TABLE_END_RID AS SRC_END_RID,
            C.END_RID AS ROLLUP_END_RID,
            B.TABLE_END_RID - C.END_RID AS GAP,
            CASE C.RUN_STATE WHEN 'I' THEN 'INIT' WHEN 'S' THEN 'SLEEPING' WHEN 'R' THEN 'RUNNING' ELSE 'UNKNOWN' END AS RUN_STATE,
            C.LAST_ELAPSED_MSEC AS LAST_ELAPSED_MSEC,
            C.LAST_WAKEUP_TIME AS LAST_WAKEUP_TIME,
            C.NEXT_WAKEUP_TIME AS NEXT_WAKEUP_TIME
        FROM
            M$SYS_TABLES A,
            V$STORAGE_TAG_TABLES B,
            V$ROLLUP C,
            M$SYS_USERS U
        WHERE 
            A.ID=B.ID
        AND A.DATABASE_ID=C.DATABASE_ID
        AND A.DATABASE_ID=-1
        AND A.NAME=C.SOURCE_TABLE
        AND A.USER_ID=C.USER_ID
        AND U.USER_ID=C.USER_ID
        AND B.TABLE_END_RID <> 0
        ORDER BY U.USER_ID, SRC_TABLE;`);

        let box = pretty.Table(config);
        if (config.long) {
            box.appendHeader(["USER_NAME", "ROLLUP_NAME", "SRC_TABLE", "ROLLUP_TABLE", "SRC_END_RID", "ROLLUP_END_RID", "GAP", "STATE", "LAST_ELAPSED", "LAST_WAKEUP_TIME", "NEXT_WAKEUP_TIME"]);
            box.setColumnConfigs([
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // USER_NAME
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // ROLLUP_NAME
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // SRC_TABLE
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // ROLLUP_TABLE
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // SRC_END_RID
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // ROLLUP_END_RID
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // GAP
                { align: pretty.Align.left, alignHeader: pretty.Align.left },  // STATE
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // LAST_ELAPSED
                { align: pretty.Align.left, alignHeader: pretty.Align.left },  // LAST_WAKEUP_TIME
                { align: pretty.Align.left, alignHeader: pretty.Align.left },  // NEXT_WAKEUP_TIME
            ]);
            for (const row of rows) {
                let elapsed = pretty.Durations(row.LAST_ELAPSED_MSEC * 1e6);
                if (elapsed == '0ns') elapsed = '0ms'; // since last_elapsed is in milliseconds
                let lastWakeTime = row.LAST_WAKEUP_TIME && row.LAST_WAKEUP_TIME.unixNano() > 0 ? row.LAST_WAKEUP_TIME : '';
                let nextWakeTime = row.NEXT_WAKEUP_TIME && row.NEXT_WAKEUP_TIME.unixNano() > 0 ? row.NEXT_WAKEUP_TIME : '';
                box.append([
                    row.USER_NAME,
                    row.ROLLUP_NAME,
                    row.SRC_TABLE,
                    row.ROLLUP_TABLE,
                    row.SRC_END_RID,
                    row.ROLLUP_END_RID,
                    pretty.Ints(row.GAP),
                    row.RUN_STATE,
                    elapsed,
                    lastWakeTime,
                    nextWakeTime
                ]);
            }
        } else {
            box.appendHeader(["ROLLUP_NAME", "SRC_TABLE", "ROLLUP_TABLE", "SRC_END_RID", "ROLLUP_END_RID", "GAP", "LAST_ELAPSED"]);
            box.setColumnConfigs([
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // ROLLUP_NAME
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // SRC_TABLE
                { align: pretty.Align.left, alignHeader: pretty.Align.left }, // ROLLUP_TABLE
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // SRC_END_RID
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // ROLLUP_END_RID
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // GAP
                { align: pretty.Align.right, alignHeader: pretty.Align.left }, // LAST_ELAPSED
            ]);
            for (const row of rows) {
                let elapsed = pretty.Durations(row.LAST_ELAPSED_MSEC * 1e6);
                if (elapsed == '0ns') elapsed = '0ms'; // since last_elapsed is in milliseconds
                box.append([
                    row.ROLLUP_NAME,
                    row.SRC_TABLE,
                    row.ROLLUP_TABLE,
                    row.SRC_END_RID,
                    row.ROLLUP_END_RID,
                    pretty.Ints(row.GAP),
                    elapsed
                ]);
            }
        }
        console.println(box.render());
    } finally {
        rows && rows.close();
    }
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
