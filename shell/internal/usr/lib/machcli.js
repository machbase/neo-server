'use strict';

const _machcli = require('@jsh/machcli');
const { getMachCliConfig } = require('@jsh/session');

class Client {
    constructor(conf) {
        this.db = _machcli.NewDatabase(JSON.stringify({ ...getMachCliConfig(), ...conf }));
        this.ctx = this.db.ctx;
    }
    close() {
        this.db.close();
    }
    connect() {
        let conn = this.db.connect();
        return new Connection(this.ctx, conn);
    }
    normalizeTableName(tableName) {
        return this.db.normalizeTableName(tableName);
    }
    user() {
        return this.db.user();
    }
}

class Connection {
    constructor(ctx, dbConn) {
        this.ctx = ctx;
        this.conn = dbConn;
    }
    close() {
        this.conn.close();
    }
    explain() {
        let plan = this.conn.explain(this.ctx, ...arguments);
        return plan;
    }
    query() {
        let rows = this.conn.query(this.ctx, ...arguments);
        return new Rows(this.ctx, rows);
    }
    queryRow() {
        let row = this.conn.queryRow(this.ctx, ...arguments);
        let value = { _ROWNUM: 1 };
        value.err = () => { row.err(); };
        if (row.err()) {
            throw new Error(row.err());
        }
        let cols = row.columns();
        let names = cols.names();
        let buffer = cols.makeBuffer();
        row.scan(...buffer);
        for (let i = 0; i < names.length; i++) {
            value[names[i]] = _machcli.Unbox(buffer[i]);
        }
        return value;
    }
    exec() {
        let result = this.conn.exec(this.ctx, ...arguments);
        if (result.err()) {
            throw new Error(result.err());
        }
        return {
            rowsAffected: result.rowsAffected(),
            message: result.message()
        };
    }
    append() {
        let appender = this.conn.appender(this.ctx, ...arguments);
        return appender
    }
}

class Rows {
    constructor(ctx, dbRows) {
        this.ctx = ctx;
        this.rows = dbRows;
        this.cols = dbRows.columns();
        this.columnNames = this.cols.names();
        this.columnTypes = this.cols.dataTypes();
        this.rownum = 0;
        this.message = dbRows.message();
    }
    close() {
        this.rows.close();
    }
    next() {
        let hasNext = this.rows.next(this.ctx);
        if (!hasNext) {
            return { done: true };
        }
        let buffer = this.cols.makeBuffer();
        this.rows.scan(...buffer);
        this.rownum += 1;
        let row = new Row(this.cols, buffer);
        return { value: row, done: false };
    }
    [Symbol.iterator]() {
        return {
            next: () => {
                return this.next();
            }
        };
    }
}

class Row {
    constructor(cols, buffer) {
        this.buffer = buffer;
        this.names = cols.names();

        for (let i = 0; i < this.names.length; i++) {
            this[this.names[i]] = _machcli.Unbox(buffer[i]);
        }
    }
    [Symbol.iterator]() {
        let index = 0;
        return {
            next: () => {
                if (index < this.names.length) {
                    let key = this.names[index];
                    let val = _machcli.Unbox(this.buffer[index]);
                    index += 1;
                    return { key: key, value: val, done: false };
                } else {
                    return { done: true };
                }
            }
        };
    }
}

function queryDatabaseId(conn, dbName) {
    if (dbName === '' || dbName === 'MACHBASEDB') {
        return -1;
    }
    let row = conn.queryRow(`SELECT BACKUP_TBSID FROM V$STORAGE_MOUNT_DATABASES WHERE MOUNTDB='${dbName}'`);
    if (!row || !row.BACKUP_TBSID) {
        throw new Error(`Database '${dbName}' not found`);
    }
    return row.BACKUP_TBSID;
}

function queryTableType(conn, names) {
    const userName = names[1];
    const tableName = names[2];
    const sql = "select type from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
    const r = conn.queryRow(sql, userName, tableName);
    return r.TYPE;
}

const TableType = {
    Log: 0,
    Fixed: 1,
    Volatile: 3,
    Lookup: 4,
    KeyValue: 5,
    Tag: 6,
};

function stringTableType(typ) {
    switch (typ) {
        case TableType.Fixed:
            return "Fixed";
        case TableType.Log:
            return "Log";
        case TableType.Volatile:
            return "Volatile";
        case TableType.Lookup:
            return "Lookup";
        case TableType.KeyValue:
            return "KeyValue";
        case TableType.Tag:
            return "Tag";
        default:
            return `UndefinedTable-${typ}`;
    }
}

const TableFlag = {
    None: 0,
    Data: 1,
    Rollup: 2,
    Meta: 4,
    Stat: 8,
};

function stringTableFlag(flag) {
    switch (flag) {
        case TableFlag.None:
            return "";
        case TableFlag.Data:
            return "Data";
        case TableFlag.Rollup:
            return "Rollup";
        case TableFlag.Meta:
            return "Meta";
        case TableFlag.Stat:
            return "Stat";
        default:
            return `UndefinedTableFlag-${flag}`;
    }
}

function stringTableDescription(typ, flag) {
    let desc = stringTableType(typ);
    let flagStr = stringTableFlag(flag);
    if (flagStr.length > 0) {
        desc += ` (${flagStr.toLowerCase()})`;
    }
    return desc;
}

const ColumnType = {
    Short: 4,
    UShort: 104,
    Integer: 8,
    UInteger: 108,
    Long: 12,
    ULong: 112,
    Float: 16,
    Double: 20,
    Varchar: 5,
    Text: 49,
    Clob: 53,
    Blob: 57,
    Binary: 97,
    Datetime: 6,
    IPv4: 32,
    IPv6: 36,
    JSON: 61,
    Unknown: 0,
}

function stringColumnType(colType) {
    switch (colType) {
        case ColumnType.Short:
            return "short";
        case ColumnType.UShort:
            return "ushort";
        case ColumnType.Integer:
            return "integer";
        case ColumnType.UInteger:
            return "uinteger";
        case ColumnType.Long:
            return "long";
        case ColumnType.ULong:
            return "ulong";
        case ColumnType.Float:
            return "float";
        case ColumnType.Double:
            return "double";
        case ColumnType.Datetime:
            return "datetime";
        case ColumnType.Varchar:
            return "varchar";
        case ColumnType.IPv4:
            return "ipv4";
        case ColumnType.IPv6:
            return "ipv6";
        case ColumnType.Text:
            return "text";
        case ColumnType.Clob:
            return "clob";
        case ColumnType.Blob:
            return "blob";
        case ColumnType.Binary:
            return "binary";
        case ColumnType.JSON:
            return "json";
        default:
            return `UndefinedColumnType-${colType}`;
    }
}

function columnWidth(colType, length) {
    switch (colType) {
        case ColumnType.Short:
            return 6;
        case ColumnType.UShort:
            return 5;
        case ColumnType.Integer:
            return 11;
        case ColumnType.UInteger:
            return 10;
        case ColumnType.Long:
            return 20;
        case ColumnType.ULong:
            return 20;
        case ColumnType.Float:
            return 17;
        case ColumnType.Double:
            return 17;
        case ColumnType.IPv4:
            return 15;
        case ColumnType.IPv6:
            return 45;
        case ColumnType.Datetime:
            return 31;
        default:
            return length;
    }
}

const ColumnFlag = {
    TagName: 0x08000000,
    Basetime: 0x01000000,
    Summarized: 0x02000000,
    MetaColumn: 0x04000000,
}

function stringColumnFlag(flag) {
    let flags = [];
    if (flag & ColumnFlag.Basetime) {
        flags.push("basetime");
    }
    if (flag & ColumnFlag.Summarized) {
        flags.push("summarized");
    }
    if (flag & ColumnFlag.MetaColumn) {
        flags.push("meta");
    }
    if (flag & ColumnFlag.TagName) {
        flags.push("tag name");
    }
    return flags.join(",");
}

module.exports = {
    Client,
    queryDatabaseId,
    queryTableType,
    stringTableType,
    TableType,
    stringTableFlag,
    TableFlag,
    stringTableDescription,
    ColumnType,
    stringColumnType,
    columnWidth,
    ColumnFlag,
    stringColumnFlag,
};