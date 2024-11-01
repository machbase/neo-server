package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

func TranslateShowTables() string {
	sqlText := SqlTidy(`
		SELECT
			j.DB_NAME as DB,
			u.NAME as USER_NAME,
			j.NAME as NAME,
			j.ID as ID,
			j.TYPE as TYPE,
			j.FLAG as FLAG
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
		ORDER by j.NAME
		`)
	return sqlText
}

func Tables(ctx context.Context, conn Conn, callback func(*TableInfo, error) bool) {
	sqlText := TranslateShowTables()
	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(nil, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		ti := &TableInfo{}
		err := rows.Scan(&ti.Database, &ti.User, &ti.Name, &ti.Id, &ti.Type, &ti.Flag)
		if err != nil {
			callback(nil, err)
			return
		}
		if !callback(ti, nil) {
			return
		}
	}
}

func QueryTableType(ctx context.Context, conn Conn, fullTableName string) (TableType, error) {
	_, userName, tableName := TokenizeFullTableName(fullTableName)
	sql := "select type from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRow(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	if r.Err() != nil {
		return -1, r.Err()
	}
	var ret TableType
	if err := r.Scan(&ret); err != nil {
		return -1, err
	}
	return ret, nil
}

func ExistsTable(ctx context.Context, conn Conn, fullTableName string) (bool, error) {
	_, userName, tableName := TokenizeFullTableName(fullTableName)
	sql := "select count(*) from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRow(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	if err := r.Err(); err != nil {
		fmt.Println("error", err.Error())
		return false, err
	}
	var count = 0
	if err := r.Scan(&count); err != nil {
		return false, err
	}
	return (count == 1), nil
}

func ExistsTableTruncate(ctx context.Context, conn Conn, fullTableName string, truncate bool) (exists bool, truncated bool, err error) {
	exists, err = ExistsTable(ctx, conn, fullTableName)
	if err != nil {
		return
	}
	if !exists {
		return
	}

	// TRUNCATE TABLE
	if !truncate {
		return
	}
	tableType, err0 := QueryTableType(ctx, conn, fullTableName)
	if err0 != nil {
		err = errors.Wrap(err0, fmt.Sprintf("table '%s' doesn't exist", fullTableName))
		return
	}
	if tableType == TableTypeLog {
		result := conn.Exec(ctx, fmt.Sprintf("truncate table %s", fullTableName))
		if result.Err() != nil {
			err = result.Err()
			return
		}
		truncated = true
	} else {
		result := conn.Exec(ctx, fmt.Sprintf("delete from %s", fullTableName))
		if result.Err() != nil {
			err = result.Err()
			return
		}
		truncated = true
	}
	return
}

// returns dbName, userName, tableName
func TokenizeFullTableName(name string) (string, string, string) {
	tableName := strings.ToUpper(name)
	userName := "SYS"
	dbName := "MACHBASEDB"
	toks := strings.Split(tableName, ".")
	if len(toks) == 2 {
		userName = toks[0]
		tableName = toks[1]
	} else if len(toks) == 3 {
		dbName = toks[0]
		userName = toks[1]
		tableName = toks[2]
	}
	return dbName, userName, tableName
}

// Describe retrieves the result of 'desc table'.
//
// If includeHiddenColumns is true, the result includes hidden columns those name start with '_'
// such as "_RID" and "_ARRIVAL_TIME".
func DescribeTable(ctx context.Context, conn Conn, name string, includeHiddenColumns bool) (*TableDescription, error) {
	_, _, tableName := TableName(name).Split()
	if strings.HasPrefix(tableName, "V$") {
		return describe_mv(ctx, conn, TableName(name), includeHiddenColumns)
	} else if strings.HasPrefix(tableName, "M$") {
		return describe_mv(ctx, conn, TableName(name), includeHiddenColumns)
	} else {
		return describe(ctx, conn, TableName(name), includeHiddenColumns)
	}
}

func describe(ctx context.Context, conn Conn, name TableName, includeHiddenColumns bool) (*TableDescription, error) {
	d := &TableDescription{}
	var colCount int

	dbName, userName, tableName := name.Split()
	dbId := -1

	if dbName != "" && dbName != "MACHBASEDB" {
		row := conn.QueryRow(ctx, "select BACKUP_TBSID from V$STORAGE_MOUNT_DATABASES where MOUNTDB = ?", dbName)
		if err := row.Scan(&dbId); err != nil {
			return nil, err
		}
	}

	describeSqlText := SqlTidy(
		`SELECT
			j.ID as TABLE_ID,
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG,
			j.COLCOUNT as TABLE_COLCOUNT
		from
			M$SYS_USERS u,
			M$SYS_TABLES j
		where
			u.NAME = ?
		and j.USER_ID = u.USER_ID
		and j.DATABASE_ID = ?
		and j.NAME = ?`)

	r := conn.QueryRow(ctx, describeSqlText, userName, dbId, tableName)
	if r.Err() != nil {
		return nil, r.Err()
	}
	if err := r.Scan(&d.Id, &d.Type, &d.Flag, &colCount); err != nil {
		return nil, err
	}
	d.Database = dbName
	d.User = userName
	d.Name = tableName

	rows, err := conn.Query(ctx, "select name, type, length, id, flag from M$SYS_COLUMNS where table_id = ? AND database_id = ? order by id", d.Id, dbId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		col := &Column{}
		err = rows.Scan(&col.Name, &col.Type, &col.Length, &col.Id, &col.Flag)
		if err != nil {
			return nil, err
		}
		if !includeHiddenColumns && strings.HasPrefix(col.Name, "_") {
			continue
		}
		col.DataType = col.Type.DataType()
		d.Columns = append(d.Columns, col)
	}
	if indexes, err := describe_idx(ctx, conn, d.Id, dbId); err != nil {
		return nil, err
	} else {
		d.Indexes = indexes
	}
	return d, nil
}

func describe_idx(ctx context.Context, conn Conn, tableId int64, dbId int) ([]*IndexDescription, error) {
	rows, err := conn.Query(ctx, `select name, type, id from M$SYS_INDEXES where table_id = ? AND database_id = ?`, tableId, dbId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := []*IndexDescription{}
	for rows.Next() {
		d := &IndexDescription{}
		var indexType int
		if err = rows.Scan(&d.Name, &indexType, &d.Id); err != nil {
			return nil, err
		}
		d.Type = IndexType(indexType)
		idxCols, err := conn.Query(ctx, `select name from M$SYS_INDEX_COLUMNS where index_id = ? AND database_id = ? order by col_id`, d.Id, dbId)
		if err != nil {
			return nil, err
		}
		for idxCols.Next() {
			var col string
			if err = idxCols.Scan(&col); err != nil {
				idxCols.Close()
				return nil, err
			}
			d.Cols = append(d.Cols, col)
		}
		idxCols.Close()
		indexes = append(indexes, d)
	}
	return indexes, nil
}

func describe_mv(ctx context.Context, conn Conn, name TableName, includeHiddenColumns bool) (*TableDescription, error) {
	d := &TableDescription{}
	var tableType int
	var colCount int

	d.Database, d.User, d.Name = name.Split()
	tablesTable := "M$SYS_TABLES"
	columnsTable := "M$SYS_COLUMNS"
	if strings.HasPrefix(d.Name, "V$") {
		tablesTable = "V$TABLES"
		columnsTable = "V$COLUMNS"
	} else if strings.HasPrefix(d.Name, "M$") {
		tablesTable = "M$TABLES"
		columnsTable = "M$COLUMNS"
	}
	r := conn.QueryRow(ctx, fmt.Sprintf("select name, type, flag, id, colcount from %s where name = ?", tablesTable), d.Name)
	if err := r.Scan(&d.Name, &tableType, &d.Flag, &d.Id, &colCount); err != nil {
		return nil, err
	}
	d.Type = TableType(tableType)

	rows, err := conn.Query(ctx, fmt.Sprintf(`select name, type, length, id from %s where table_id = ? order by id`, columnsTable), d.Id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		col := &Column{}
		err = rows.Scan(&col.Name, &col.Type, &col.Length, &col.Id)
		if err != nil {
			return nil, err
		}
		if !includeHiddenColumns && strings.HasPrefix(col.Name, "_") {
			continue
		}
		col.DataType = col.Type.DataType()
		d.Columns = append(d.Columns, col)
	}
	return d, nil
}

// TableTypeDescription converts the given TableType and flag into string representation.
func TableTypeDescription(typ TableType, flag TableFlag) string {
	desc := "undef"
	switch typ {
	case TableTypeLog:
		desc = "Log Table"
	case TableTypeFixed:
		desc = "Fixed Table"
	case TableTypeVolatile:
		desc = "Volatile Table"
	case TableTypeLookup:
		desc = "Lookup Table"
	case TableTypeKeyValue:
		desc = "KeyValue Table"
	case TableTypeTag:
		desc = "Tag Table"
	}
	switch flag {
	case TableFlagData:
		desc += " (data)"
	case TableFlagRollup:
		desc += " (rollup)"
	case TableFlagMeta:
		desc += " (meta)"
	case TableFlagStat:
		desc += " (stat)"
	}
	return desc
}
