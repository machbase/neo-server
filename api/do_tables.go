package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-client/api"
)

func ifThenElse(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func ListTablesSql(showAll bool, descriptiveType bool) string {
	return api.SqlTidy(
		`SELECT
			j.DB_NAME as DATABASE_NAME,
			u.NAME as USER_NAME,
			j.NAME as TABLE_NAME,
			j.ID as TABLE_ID,`,
		ifThenElse(descriptiveType, `
			case j.TYPE
				when 0 then 'Log'
				when 1 then 'Fixed'
				when 3 then 'Volatile'
				when 4 then 'Lookup'
				when 5 then 'KeyValue'
				when 6 then 'Tag'
				else ''
			end as TABLE_TYPE,
			case j.FLAG
				when 1 then 'Data'
				when 2 then 'Rollup'
				when 4 then 'Meta'
				when 8 then 'Stat'
				else ''
			end as TABLE_FLAG`,
			`
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG`),
		`FROM
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
			u.USER_ID = j.USER_ID`,
		ifThenElse(showAll, "", "AND SUBSTR(j.NAME, 1, 1) <> '_'"),
		`ORDER by j.NAME`)
}

func ListTablesWalk(ctx context.Context, conn api.Conn, showAll bool, callback func(*TableInfo) bool) {
	sqlText := ListTablesSql(showAll, false)
	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(&TableInfo{err: err})
		return
	}
	defer rows.Close()

	for rows.Next() {
		ti := &TableInfo{}
		ti.err = rows.Scan(&ti.Database, &ti.User, &ti.Name, &ti.Id, &ti.Type, &ti.Flag)
		if !callback(ti) {
			return
		}
	}
}

func ListTables(ctx context.Context, conn api.Conn, showAll bool) (ret []*TableInfo, cause error) {
	ListTablesWalk(ctx, conn, showAll, func(ti *TableInfo) bool {
		if ti.err == nil && ti != nil {
			ret = append(ret, ti)
		}
		cause = ti.err
		return ti.err == nil
	})
	return
}

func QueryTableType(ctx context.Context, conn api.Conn, fullTableName string) (api.TableType, error) {
	_, userName, tableName := TableName(fullTableName).Split()
	sql := "select type from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRow(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	if r.Err() != nil {
		return -1, r.Err()
	}
	var ret api.TableType
	if err := r.Scan(&ret); err != nil {
		return -1, err
	}
	return ret, nil
}

func TruncateTableIfExists(ctx context.Context, conn api.Conn, fullTableName string, truncate bool) (exists bool, truncated bool, err error) {
	exists, err = api.ExistsTable(ctx, conn, fullTableName)
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
		err = fmt.Errorf("table '%s' doesn't exist, %s", fullTableName, err0.Error())
		return
	}
	if tableType == api.TableTypeLog {
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

func ListLsmIndexesWalk(ctx context.Context, conn api.Conn, callback func(*LsmIndexInfo) bool) {
	sqlText := `select 
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
	order by 1, 2, 3`
	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(&LsmIndexInfo{err: err})
		return
	}
	defer rows.Close()
	for rows.Next() {
		rec := &LsmIndexInfo{}
		rec.err = rows.Scan(&rec.TableName, &rec.IndexName, &rec.Level, &rec.Count)
		if !callback(rec) {
			return
		}
	}
}

func ListRollupGapWalk(ctx context.Context, conn api.Conn, callback func(*RollupGapInfo) bool) {
	r := conn.QueryRow(ctx, "SELECT count(DATABASE_ID) FROM V$ROLLUP")
	if err := r.Err(); err != nil && strings.Contains(err.Error(), "DATABASE_ID") {
		// neo version < 8.0.60 (19 Sep 2025) does not have DATABASE_ID column in V$ROLLUP
		listRollupGapWalk_pre_8_0_60(ctx, conn, callback)
	} else {
		listRollupGapWalk_since_8_0_60(ctx, conn, callback)
	}
}

func listRollupGapWalk_pre_8_0_60(ctx context.Context, conn api.Conn, callback func(*RollupGapInfo) bool) {
	sqlText := api.SqlTidy(`SELECT
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
	ORDER BY SRC_TABLE`)

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(&RollupGapInfo{err: err})
		return
	}
	defer rows.Close()

	for rows.Next() {
		rec := &RollupGapInfo{}
		var lastElapsedMs float64
		rec.err = rows.Scan(&rec.SrcTable, &rec.RollupTable, &rec.SrcEndRID, &rec.RollupEndRID, &rec.Gap, &lastElapsedMs)
		rec.LastElapsed = time.Duration(lastElapsedMs) * time.Millisecond
		if !callback(rec) {
			return
		}
	}
}

func listRollupGapWalk_since_8_0_60(ctx context.Context, conn api.Conn, callback func(*RollupGapInfo) bool) {
	sqlText := api.SqlTidy(`SELECT
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
	ORDER BY SRC_TABLE`)

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(&RollupGapInfo{err: err})
		return
	}
	defer rows.Close()

	for rows.Next() {
		rec := &RollupGapInfo{}
		var lastElapsedMs float64
		rec.err = rows.Scan(&rec.SrcTable, &rec.RollupTable, &rec.SrcEndRID, &rec.RollupEndRID, &rec.Gap, &lastElapsedMs)
		rec.LastElapsed = time.Duration(lastElapsedMs) * time.Millisecond
		if !callback(rec) {
			return
		}
	}
}

func ListStorageWalk(ctx context.Context, conn api.Conn, callback func(*StorageInfo) bool) {
	sqlText := api.SqlTidy(`select
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
	order by a.table_name`)

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(&StorageInfo{err: err})
		return
	}
	defer rows.Close()

	for rows.Next() {
		rec := &StorageInfo{}
		rec.err = rows.Scan(&rec.TableName, &rec.DataSize, &rec.IndexSize, &rec.TotalSize)
		if !callback(rec) {
			return
		}
	}
}

func ListTableUsageWalk(ctx context.Context, conn api.Conn, callback func(*TableUsageInfo) bool) {
	sqlText := api.SqlTidy(`SELECT
		a.NAME as TABLE_NAME,
		t.STORAGE_USAGE as STORAGE_USAGE
	FROM
		M$SYS_TABLES a,
		M$SYS_USERS u,
		V$STORAGE_TABLES t
	WHERE
		a.user_id = u.user_id
	AND t.ID = a.id
	ORDER BY a.NAME`)

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(&TableUsageInfo{err: err})
		return
	}
	defer rows.Close()

	for rows.Next() {
		rec := &TableUsageInfo{}
		rec.err = rows.Scan(&rec.TableName, &rec.StorageUsage)
		if !callback(rec) {
			return
		}
	}
}

func ListStatementsWalk(ctx context.Context, conn api.Conn, callback func(*StatementInfo) bool) {
	stmtRows, err := conn.Query(ctx, "SELECT ID, SESS_ID, STATE, RECORD_SIZE, QUERY FROM V$STMT")
	if err != nil {
		callback(&StatementInfo{err: err})
		return
	}
	defer stmtRows.Close()

	for stmtRows.Next() {
		rec := &StatementInfo{}
		rec.err = stmtRows.Scan(&rec.ID, &rec.SessionID, &rec.State, &rec.RecordSize, &rec.Query)
		if !callback(rec) {
			return
		}
	}

	neoRows, err := conn.Query(ctx, "SELECT ID, SESS_ID, STATE, QUERY, APPEND_SUCCESS_CNT, APPEND_FAILURE_CNT FROM V$NEO_STMT")
	if err != nil {
		callback(&StatementInfo{err: err, IsNeo: true})
		return
	}
	defer neoRows.Close()

	for neoRows.Next() {
		rec := &StatementInfo{IsNeo: true}
		rec.err = neoRows.Scan(&rec.ID, &rec.SessionID, &rec.State, &rec.Query, &rec.AppendSuccessCount, &rec.AppendFailureCount)
		if !callback(rec) {
			return
		}
	}
}

func ListSessionsWalk(ctx context.Context, conn api.Conn, callback func(*SessionInfo) bool) {
	rows, err := conn.Query(ctx, `SELECT ID, USER_ID, USER_NAME, MAX_QPX_MEM FROM V$SESSION`)
	if err != nil {
		callback(&SessionInfo{err: err})
		return
	}
	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()
	for rows.Next() {
		rec := &SessionInfo{}
		rec.err = rows.Scan(&rec.ID, &rec.UserID, &rec.UserName, &rec.MaxQPXMem)
		if !callback(rec) {
			return
		}
	}
	rows.Close()
	rows = nil

	neoRows, err := conn.Query(ctx, "SELECT ID, USER_ID, USER_NAME, STMT_COUNT FROM V$NEO_SESSION")
	if err != nil {
		callback(&SessionInfo{err: err})
		return
	}
	defer func() {
		if neoRows != nil {
			neoRows.Close()
		}
	}()

	for neoRows.Next() {
		rec := &SessionInfo{IsNeo: true}
		rec.err = neoRows.Scan(&rec.ID, &rec.UserID, &rec.UserName, &rec.StmtCount)
		if !callback(rec) {
			return
		}
	}
	neoRows.Close()
	neoRows = nil
}
