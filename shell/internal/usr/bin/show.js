'use strict';

const process = require('process');
const neoapi = require('/usr/lib/neoapi');
const pretty = require('/usr/lib/pretty');
const { parseAndRun } = require('/usr/lib/opts');

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
    options: {
        help: optionHelp,
        ...pretty.TableArgOptions,
    },
};

const tagindexgapConfig = {
    func: showTagIndexGap,
    command: 'tagindexgap',
    usage: 'show tagindexgap',
    description: 'Show tag index gap information',
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

function showInfo(config, args) {
    const client = new neoapi.Client(config);
    client.getServerInfo()
        .then((nfo) => {
            let box = pretty.Table(config);
            box.appendHeader(['NAME', 'VALUE']);
            box.appendRow(box.row('build.version', `v${nfo.version.major || 0}.${nfo.version.minor || 0}.${nfo.version.patch || 0}`));
            box.appendRow(box.row('build.hash', nfo.version.gitSHA));
            box.appendRow(box.row('build.timestamp', nfo.version.buildTimestamp));
            box.appendRow(box.row('build.engine', nfo.version.engine));

            box.appendRow(box.row('runtime.os', nfo.runtime.OS));
            box.appendRow(box.row('runtime.arch', nfo.runtime.arch));
            box.appendRow(box.row('runtime.pid', nfo.runtime.pid));
            box.appendRow(box.row('runtime.uptime', pretty.Durations(nfo.runtime.uptimeInSecond * 1e9)));
            box.appendRow(box.row('runtime.processes', nfo.runtime.processes));
            box.appendRow(box.row('runtime.goroutines', nfo.runtime.goroutines));

            box.appendRow(box.row('mem.malloc', pretty.Ints(nfo.runtime.mem.mallocs)));
            box.appendRow(box.row('mem.frees', pretty.Ints(nfo.runtime.mem.frees)));
            box.appendRow(box.row('mem.lives', pretty.Ints(nfo.runtime.mem.lives)));

            box.appendRow(box.row('mem.sys', pretty.Bytes(nfo.runtime.mem.sys)));
            box.appendRow(box.row('mem.heap_sys', pretty.Bytes(nfo.runtime.mem.heap_sys)));
            box.appendRow(box.row('mem.heap_alloc', pretty.Bytes(nfo.runtime.mem.heap_alloc)));
            box.appendRow(box.row('mem.heap_in_use', pretty.Bytes(nfo.runtime.mem.heap_in_use)));
            box.appendRow(box.row('mem.stack_sys', pretty.Bytes(nfo.runtime.mem.stack_sys)));
            box.appendRow(box.row('mem.stack_in_use', pretty.Bytes(nfo.runtime.mem.stack_in_use)));
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function showPorts(config, args) {
    const client = new neoapi.Client(config);
    const service = args.service ? args.service : '';
    client.getServicePorts(service)
        .then((data) => {
            let box = pretty.Table(config);
            box.appendHeader(['SERVICE', 'PORT']);
            for (const s of data) {
                box.append([s.Service, s.Address]);
            }
            console.println(box.render());
        })
        .catch((err) => {
            console.println('Error:', err.message);
        });
}

function showLicense(config, args) {
    const { Client } = require('/usr/lib/machcli');
    let db, conn, row;
    try {
        db = new Client(config);
        conn = db.connect();
        row = conn.queryRow("SELECT " +
            "ID, TYPE, CUSTOMER, PROJECT, COUNTRY_CODE, INSTALL_DATE, ISSUE_DATE, VIOLATE_STATUS, VIOLATE_MSG " +
            "FROM V$LICENSE_INFO");

        let box = pretty.Table(config);
        box.appendHeader(["ID", "TYPE", "CUSTOMER", "PROJECT", "COUNTRY_CODE", "INSTALL_DATE", " ISSUE_DATE", "STATUS"]);
        box.append([
            row.ID, row.TYPE, row.CUSTOMER, row.PROJECT, row.COUNTRY_CODE,
            row.INSTALL_DATE, row.ISSUE_DATE, row.VIOLATE_STATUS === 0 ? "VALID" : "INVALID"]);
        console.println(box.render());
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        conn && conn.close();
        db && db.close();
    }
}

function showUsers(config, args) {
    const { Client } = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new Client(config);
        conn = db.connect();
        rows = conn.query("SELECT USER_ID, NAME FROM M$SYS_USERS");

        let box = pretty.Table(config);

        box.appendHeader(["USER_ID", "NAME"]);
        for (const row of rows) {
            box.append([row.USER_ID, row.NAME]);
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

function showTables(config, args) {
    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
        conn = db.connect();
        rows = conn.query(`SELECT
    j.DB_NAME as DATABASE_NAME,
    u.NAME as USER_NAME,
    j.NAME as TABLE_NAME,
    j.ID as TABLE_ID,
    j.TYPE as TABLE_TYPE,
    j.FLAG as TABLE_FLAG
FROM
    M$SYS_USERS u,
    (
        select
            a.ID as ID,
            a.NAME as NAME,
            a.USER_ID as USER_ID,
            a.TYPE as TYPE,
            a.FLAG as FLAG,
            case a.DATABASE_ID
                when -1 then 'MACHBASEDB'
                else d.MOUNTDB
            end as DB_NAME
        from
            M$SYS_TABLES a
        left join
            V$STORAGE_MOUNT_DATABASES d
        on
            a.DATABASE_ID = d.BACKUP_TBSID
    ) as j
WHERE
    u.USER_ID = j.USER_ID
    ${config.all ? '' : "AND SUBSTR(j.NAME, 1, 1) <> '_'"}
ORDER BY
    j.NAME`);

        let box = pretty.Table(config);
        box.appendHeader(["DATABASE_NAME", "USER_NAME", "TABLE_NAME", "TABLE_ID", "TABLE_TYPE", "TABLE_FLAG"]);
        for (const row of rows) {
            box.append([
                row.DATABASE_NAME, row.USER_NAME, row.TABLE_NAME, row.TABLE_ID,
                machcli.stringTableType(row.TABLE_TYPE),
                machcli.stringTableFlag(row.TABLE_FLAG)
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

function showTable(config, args) {
    if (!args.table) {
        console.println('Error: table name is required');
        process.exit(1);
    }
    const tableName = args.table;
    const machcli = require('/usr/lib/machcli');
    let db, conn;
    try {
        db = new machcli.Client(config);
        conn = db.connect();

        let names = db.normalizeTableName(tableName)
        if (names[2].startsWith('V$') || names[2].startsWith('M$')) {
            describeMVTable(conn, names, config);
        } else {
            describeTable(conn, names, config);
        }
    } catch (err) {
        console.println("Error: ", err.message, err.stack());
    } finally {
        conn && conn.close();
        db && db.close();
    }
}

function describeTable(conn, names, config) {
    const machcli = require('/usr/lib/machcli');

    const dbName = names[0];
    const userName = names[1];
    const tableName = names[2];
    let dbId = -1;

    if (dbName != '' && dbName != 'MACHBASEDB') {
        let row = conn.queryRow(`SELECT BACKUP_TBSID FROM V$STORAGE_MOUNT_DATABASES WHERE MOUNTDB='${dbName}'`);
        if (!row || !row.BACKUP_TBSID) {
            throw new Error(`Database '${dbName}' not found`);
        }
        dbId = row.BACKUP_TBSID;
    }

    let describeSqlText = `SELECT
			j.ID as TABLE_ID,
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG,
			j.COLCOUNT as TABLE_COLCOUNT
		FROM
			M$SYS_USERS u,
			M$SYS_TABLES j
		WHERE
			u.NAME = ?
		AND j.USER_ID = u.USER_ID
		AND j.DATABASE_ID = ?
		AND j.NAME = ?`;

    let row;
    try {
        row = conn.queryRow(describeSqlText, userName, dbId, tableName);
    } catch {
        row = null;
    }
    if (!row || !row.TABLE_ID) {
        throw new Error(`Table '${tableName}' not found`);
    }

    const tableId = row.TABLE_ID;
    const tableType = row.TABLE_TYPE;
    const tableFlag = row.TABLE_FLAG;
    const tableColcount = row.TABLE_COLCOUNT;
    let rows;

    const tableTypeLabel = machcli.stringTableDescription(tableType, tableFlag);
    console.println(`${tableName} (ID: ${tableId}, ${tableTypeLabel} Table)`);
    try {
        let box = pretty.Table(config);
        box.appendHeader(["NAME", "TYPE", "LENGTH", "FLAG", "INDEX"]);

        let indexes = indexesOfTable(conn, tableId, dbId);

        rows = conn.query(`SELECT NAME, TYPE, LENGTH, ID, FLAG FROM M$SYS_COLUMNS WHERE TABLE_ID = ? AND DATABASE_ID = ? ORDER BY ID`, tableId, dbId);
        for (const col of rows) {
            let colName = col.NAME;
            if (!config.all && colName.startsWith('_')) {
                continue;
            }
            let colType = machcli.stringColumnType(col.TYPE);
            let colWidth = machcli.columnWidth(col.TYPE, col.LENGTH);
            let colFlag = machcli.stringColumnFlag(col.FLAG);
            let colIndexes = [];
            for (let idxDesc of indexes) {
                for (let indexedCol of idxDesc.cols) {
                    if (colName === indexedCol) {
                        colIndexes.push(idxDesc.name);
                        break
                    }
                }
            }
            box.appendRow(box.row(colName, colType, colWidth, colFlag, colIndexes.join(",")));
        };
        console.println(box.render());
    } finally {
        rows && rows.close();
    }
}

function describeMVTable(conn, names, config) {
    const machcli = require('/usr/lib/machcli');

    const dbName = names[0];
    const userName = names[1];
    const tableName = names[2];

    let tablesTable = 'V$TABLES';
    let columnsTable = 'V$COLUMNS';
    if (tableName.startsWith('M$')) {
        tablesTable = 'M$TABLES';
        columnsTable = 'M$COLUMNS';
    }

    let rows;
    try {
        let r = conn.queryRow(`SELECT NAME, TYPE, FLAG, ID, COLCOUNT FROM ${tablesTable} WHERE NAME = ?`, tableName);
        if (!r || !r.ID) {
            throw new Error(`Table '${tableName}' not found`);
        }

        console.println(`${tableName} (ID: ${r.ID}, ${machcli.stringTableDescription(r.TYPE, r.FLAG)} Table)`);
        let box = pretty.Table(config);
        box.appendHeader(["NAME", "TYPE", "LENGTH", "FLAG", "INDEX"]);

        rows = conn.query(`SELECT NAME, TYPE, LENGTH, ID FROM ${columnsTable} WHERE TABLE_ID = ? ORDER BY ID`, r.ID);
        for (const col of rows) {
            let colName = col.NAME;
            if (!config.all && colName.startsWith('_')) {
                continue;
            }
            let colType = machcli.stringColumnType(col.TYPE);
            let colWidth = machcli.columnWidth(col.TYPE, col.LENGTH);
            let colFlag = machcli.stringColumnFlag(col.FLAG);
            let colIndexes = [];
            box.appendRow(box.row(colName, colType, colWidth, colFlag, colIndexes.join(",")));
        };
        console.println(box.render());
    } finally {
        rows && rows.close();
    }
}


function indexesOfTable(conn, tableId, dbId) {
    let indexes = [];
    let rows;

    try {
        rows = conn.query(`SELECT NAME, TYPE, ID FROM M$SYS_INDEXES WHERE TABLE_ID = ? AND DATABASE_ID = ?`, tableId, dbId);
        for (const r of rows) {
            let idx = { name: r.NAME, type: r.TYPE, id: r.ID, cols: [] };

            let colsRows = conn.query(`SELECT NAME FROM M$SYS_INDEX_COLUMNS WHERE INDEX_ID = ? AND DATABASE_ID = ? ORDER BY COL_ID`, idx.id, dbId);
            for (const col of colsRows) {
                idx.cols.push(col.NAME);
            }
            colsRows.close();
            indexes.push(idx);
        }
    } catch (err) {
        console.error("Error: indexesOfTable:", err.message);
    } finally {
        rows && rows.close();
    }
    return indexes;
}

function showMetaTables(config, args) {
    showMVTables('M$TABLES', config, args);
}

function showVirtualTables(config, args) {
    showMVTables('V$TABLES', config, args);
}

function showMVTables(tableName, config, args) {
    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
        conn = db.connect();
        rows = conn.query(`SELECT NAME, TYPE, FLAG, ID FROM ${tableName} ORDER BY ID`);

        let box = pretty.Table(config);
        box.appendHeader(["ID", "NAME", "TYPE"]);
        for (const row of rows) {
            box.append([
                row.ID,
                row.NAME,
                machcli.stringTableDescription(row.TYPE, row.FLAG),
            ]);
        }
        console.println(box.render());
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        conn && conn.close();
        db && db.close();
    }
}

function showSessions(config, args) {
    let box = pretty.Table(config);
    box.setTimeformat('DATETIME');
    box.appendHeader(["ID", "USER_ID", "USER_NAME", "TYPE", "LOGIN_TIME", "USER_IP", "MAX_QPX_MEM", "STMT_COUNT"]);

    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
        conn = db.connect();
        rows = conn.query(`SELECT ID, USER_ID, LOGIN_TIME, CLIENT_TYPE, USER_NAME, USER_IP, MAX_QPX_MEM FROM V$SESSION`);

        for (const row of rows) {
            box.append([
                row.ID,
                row.USER_ID,
                row.USER_NAME,
                row.CLIENT_TYPE,
                row.LOGIN_TIME,
                row.USER_IP,
                pretty.Bytes(row.MAX_QPX_MEM),
                "",
            ]);
        }
        rows && rows.close();

        rows = conn.query(`SELECT ID, USER_ID, USER_NAME, STMT_COUNT FROM V$NEO_SESSION`);
        for (const row of rows) {
            box.append([
                row.ID,
                row.USER_ID,
                row.USER_NAME,
                "neo",  // TYPE
                "",  // LOGIN_TIME
                "", // USER_IP
                "", // MAX_QPX_MEM
                row.STMT_COUNT
            ]);
        }
        rows && rows.close();
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        conn && conn.close();
        db && db.close();
    }
    console.println(box.render());
}

function showStatements(config, args) {
    const machcli = require('/usr/lib/machcli');
    let db, conn, stmtRows, neoRows;
    try {
        db = new machcli.Client(config);
        conn = db.connect();

        let box = pretty.Table(config);
        box.appendHeader(["ID", "SESSION_ID", "STATE", "TYPE", "RECORD_SIZE", "APPEND_SUCCESS_CNT", "APPEND_FAILURE_CNT", "QUERY"]);

        stmtRows = conn.query(`SELECT ID, SESS_ID, STATE, RECORD_SIZE, QUERY FROM V$STMT`);
        for (const row of stmtRows) {
            box.append([
                row.ID,
                row.SESS_ID,
                row.STATE,
                "-",
                pretty.Bytes(row.RECORD_SIZE),
                "-",
                "-",
                row.QUERY
            ]);
        }
        neoRows = conn.query(`SELECT ID, SESS_ID, STATE, QUERY, APPEND_SUCCESS_CNT, APPEND_FAILURE_CNT FROM V$NEO_STMT`);
        for (const row of neoRows) {
            box.append([
                row.ID,
                row.SESS_ID,
                row.STATE,
                "neo",
                row.APPEND_SUCCESS_CNT,
                row.APPEND_FAILURE_CNT,
                row.QUERY
            ]);
        }
        console.println(box.render());
    } catch (err) {
        console.println("Error: ", err.message);
    } finally {
        stmtRows && stmtRows.close();
        neoRows && neoRows.close();
        conn && conn.close();
        db && db.close();
    }
}

function showIndexes(config, args) {
    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
        conn = db.connect();
        rows = conn.query(`
		SELECT
			u.name as USER_NAME,
			j.DB_NAME as DATABASE_NAME,
			j.TABLE_NAME as TABLE_NAME,
			c.name as COLUMN_NAME,
			b.name as INDEX_NAME,
			b.id as INDEX_ID,
			case b.type
				when 1 then 'BITMAP'
				when 2 then 'KEYWORD'
				when 5 then 'REDBLACK'
				when 6 then 'LSM'
				when 8 then 'REDBLACK'
				when 9 then 'KEYWORD_LSM'
				when 11 then 'TAG'
				else 'LSM' 
			end as INDEX_TYPE,
			case b.key_compress
				when 0 then 'UNCOMPRESS'
				else 'COMPRESSED'
			end as KEY_COMPRESS,
			b.max_level as MAX_LEVEL,
			b.part_value_count as PART_VALUE_COUNT,
			case b.bitmap_encode
				when 0 then 'EQUAL'
				else 'RANGE'
			end as BITMAP_ENCODE
		FROM
			m$sys_indexes b, 
			m$sys_index_columns c, 
			m$sys_users u,
			(
				select
					case a.DATABASE_ID
						when -1 then 'MACHBASEDB'
						else d.MOUNTDB
					end as DB_NAME,
					a.name as TABLE_NAME,
					a.id as TABLE_ID,
					a.USER_ID as USER_ID
				from
					M$SYS_TABLES a
				left join
					V$STORAGE_MOUNT_DATABASES d
				on
					a.DATABASE_ID = d.BACKUP_TBSID
			) as j
		WHERE
			j.TABLE_ID = b.TABLE_ID
		AND b.ID = c.INDEX_ID
		AND j.USER_ID = u.USER_ID
		ORDER BY
			j.DB_NAME, j.TABLE_NAME, b.ID
	`);

        let box = pretty.Table(config);
        box.appendHeader(["USER_NAME", "DB", "TABLE_NAME", "COLUMN_NAME", "INDEX_NAME", "INDEX_TYPE"]);
        for (const row of rows) {
            box.append([
                row.USER_NAME,
                row.DATABASE_NAME,
                row.TABLE_NAME,
                row.COLUMN_NAME,
                row.INDEX_NAME,
                row.INDEX_TYPE
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

function showIndex(config, args) {
    if (!args.index) {
        console.println('Error: index name is required');
        process.exit(1);
    }
    const indexName = args.index;
    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
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
        and b.name = '${indexName}'`);

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
    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
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
    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
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
    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
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
    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
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
    const machcli = require('/usr/lib/machcli');
    let db, conn, rows;
    try {
        db = new machcli.Client(config);
        conn = db.connect();
        rows = conn.query(`SELECT
            ID,
            INDEX_STATE AS STATUS,
            TABLE_END_RID - DISK_INDEX_END_RID AS DISK_GAP,
            TABLE_END_RID - MEMORY_INDEX_END_RID AS MEMORY_GAP
        FROM
            V$STORAGE_TAG_TABLES
        ORDER BY 1`);

        let box = pretty.Table(config);
        box.appendHeader(["ID", "STATUS", "DISK_GAP", "MEMORY_GAP"]); // tag table
        box.setColumnConfigs([
            { align: pretty.Align.right, alignHeader: pretty.Align.left },
            { align: pretty.Align.left, alignHeader: pretty.Align.left },
            { align: pretty.Align.right, alignHeader: pretty.Align.left },
            { align: pretty.Align.right, alignHeader: pretty.Align.left }]);

        for (const row of rows) {
            box.append([
                row.ID,
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
    const machcli = require('/usr/lib/machcli');
    let db, conn;

    try {
        db = new machcli.Client(config);
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
                row.GAP,
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
            A.DATABASE_ID=C.DATABASE_ID
        AND A.DATABASE_ID=-1
        AND	A.ID=B.ID
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
                row.GAP,
                pretty.Durations(row.LAST_ELAPSED * 1e6)
            ]);
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

    const machcli = require('/usr/lib/machcli');
    let db, conn, colsRows, tagsRows;
    try {
        db = new machcli.Client(config);
        conn = db.connect();

        let names = db.normalizeTableName(tableName);

        let dbId = machcli.queryDatabaseId(conn, names[0]);
        let tableType = machcli.queryTableType(conn, names);
        if (tableType !== machcli.TABLE_TYPE_TAG) {
            console.println(`Error: table '${tableName}' is not a tag table`);
            process.exit(1);
        }

        let hasSummarized = false;

        colsRows = conn.query(`SELECT
                j.ID as TABLE_ID,
                j.TYPE as TABLE_TYPE,
                j.FLAG as TABLE_FLAG,
                j.COLCOUNT as TABLE_COLCOUNT
            FROM
                M$SYS_USERS u,
                M$SYS_TABLES j
            WHERE
                u.NAME = ?
            AND j.USER_ID = u.USER_ID
            AND j.DATABASE_ID = ?
            AND j.NAME = ?`, names[1], dbId, names[2]);
        for (const col of colsRows) {
            if ((col.TABLE_FLAG & machcli.TABLE_FLAG_SUMMARIZED) !== 0) {
                hasSummarized = true;
                break;
            }
        }
        colsRows.close();
        colsRows = null;

        let box = pretty.Table(config);
        box.setStringEscape(true);
        if (hasSummarized) {
            box.appendHeader(["_ID", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME", "RECENT_ROW_TIME", "MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME"]);
        } else {
            box.appendHeader(["_ID", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME", "RECENT_ROW_TIME", "MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME"]);
        }

        if (tags.length > 0) {
            tagsRows = conn.query(`SELECT _ID, NAME FROM ${names[0]}.${names[1]}._${names[2]}_META WHERE NAME IN (${tags.map(() => '?').join(',')})`, ...tags);
        } else {
            tagsRows = conn.query(`SELECT _ID, NAME FROM ${names[0]}.${names[1]}._${names[2]}_META`);
        }
        for (const row of tagsRows) {
            try {
                let stat = conn.queryRow(`SELECT
                    NAME, ROW_COUNT,
                    MIN_TIME, MAX_TIME,
                    MIN_VALUE, MIN_VALUE_TIME, MAX_VALUE, MAX_VALUE_TIME,
                    RECENT_ROW_TIME
                FROM
                    ${names[0]}.${names[1]}.V$${names[2]}_STAT
                WHERE NAME = ?`, row.NAME);

                if (hasSummarized) {
                    box.append([
                        row._ID,
                        row.NAME,
                        stat.ROW_COUNT,
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
                        row.NAME,
                        stat.ROW_COUNT,
                        stat.MIN_TIME,
                        stat.MAX_TIME,
                        stat.RECENT_ROW_TIME,
                        stat.MIN_VALUE,
                        stat.MIN_VALUE_TIME,
                        stat.MAX_VALUE,
                        stat.MAX_VALUE_TIME
                    ]);
                }
            } catch (err) {
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
        colsRows && colsRows.close();
        tagsRows && tagsRows.close();
        conn && conn.close();
        db && db.close();
    }
}
