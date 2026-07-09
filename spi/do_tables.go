package spi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-client/api"
)

type TableInfo struct {
	Database string        `json:"database"`       // M$SYS_TABLES.DATABASE_ID
	User     string        `json:"user"`           // M$SYS_USERS.NAME
	Name     string        `json:"name"`           // M$SYS_TABLES.NAME
	Id       int64         `json:"id"`             // M$SYS_TABLES.ID
	Type     api.TableType `json:"type"`           // M$SYS_TABLES.TYPE
	Flag     api.TableFlag `json:"flag,omitempty"` // M$SYS_TABLES.FLAG
	err      error         `json:"-"`
}

func (ti *TableInfo) Kind() string {
	desc := "undef"
	switch ti.Type {
	case api.TableTypeLog:
		desc = "Log Table"
	case api.TableTypeFixed:
		desc = "Fixed Table"
	case api.TableTypeVolatile:
		desc = "Volatile Table"
	case api.TableTypeLookup:
		desc = "Lookup Table"
	case api.TableTypeKeyValue:
		desc = "KeyValue Table"
	case api.TableTypeTag:
		desc = "Tag Table"
	}
	switch ti.Flag {
	case api.TableFlagData:
		desc += " (data)"
	case api.TableFlagRollup:
		desc += " (rollup)"
	case api.TableFlagMeta:
		desc += " (meta)"
	case api.TableFlagStat:
		desc += " (stat)"
	}
	return desc
}

func (ti *TableInfo) Err() error {
	return ti.err
}

func (ti *TableInfo) Columns() api.Columns {
	return api.Columns{
		{Name: "DATABASE", DataType: api.DataTypeString},
		{Name: "USER", DataType: api.DataTypeString},
		{Name: "NAME", DataType: api.DataTypeString},
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "FLAG", DataType: api.DataTypeString},
	}
}

func (ti *TableInfo) Values() []interface{} {
	return []interface{}{ti.Database, ti.User, ti.Name, ti.Id, ti.Type.ShortString(), ti.Flag.String()}
}

func ifThenElse(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func ListTablesSql(showAll bool, descriptiveType bool) string {
	return SqlTidy(
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
	_, userName, tableName := api.TableName(fullTableName).Split()
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

type LsmIndexInfo struct {
	TableName string `json:"table_name"`
	IndexName string `json:"index_name"`
	Level     int64  `json:"level"`
	Count     int64  `json:"count"`
	err       error  `json:"-"`
}

func (li *LsmIndexInfo) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "LEVEL", DataType: api.DataTypeInt64},
		{Name: "COUNT", DataType: api.DataTypeInt64},
	}
}

func (li *LsmIndexInfo) Values() []interface{} {
	return []interface{}{
		li.TableName, li.IndexName, li.Level, li.Count,
	}
}

func (li *LsmIndexInfo) Err() error {
	return li.err
}

func ListLsmIndexesInfo(ctx context.Context, conn api.Conn) ([]*LsmIndexInfo, error) {
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
		return nil, err
	}
	defer rows.Close()
	var result []*LsmIndexInfo
	for rows.Next() {
		rec := &LsmIndexInfo{}
		rec.err = rows.Scan(&rec.TableName, &rec.IndexName, &rec.Level, &rec.Count)
		if rec.err != nil {
			return nil, rec.err
		}
		result = append(result, rec)
	}
	return result, nil
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

type RollupGapInfo struct {
	SrcTable     string        `json:"src_table"`
	RollupTable  string        `json:"rollup_table"`
	SrcEndRID    int64         `json:"src_end_rid"`
	RollupEndRID int64         `json:"rollup_end_rid"`
	Gap          int64         `json:"gap"`
	LastElapsed  time.Duration `json:"last_time"`
	err          error         `json:"-"`
}

func (rgi *RollupGapInfo) Columns() api.Columns {
	return api.Columns{
		{Name: "SRC_TABLE", DataType: api.DataTypeString},
		{Name: "ROLLUP_TABLE", DataType: api.DataTypeString},
		{Name: "SRC_END_RID", DataType: api.DataTypeInt64},
		{Name: "ROLLUP_END_RID", DataType: api.DataTypeInt64},
		{Name: "GAP", DataType: api.DataTypeInt64},
		{Name: "LAST_TIME", DataType: api.DataTypeInt64},
	}
}

func (rgi *RollupGapInfo) Values() []interface{} {
	return []interface{}{
		rgi.SrcTable, rgi.RollupTable, rgi.SrcEndRID, rgi.RollupEndRID, rgi.Gap, rgi.LastElapsed,
	}
}

func (rgi *RollupGapInfo) Err() error {
	return rgi.err
}

func ListRollupGap(ctx context.Context, conn api.Conn) ([]*RollupGapInfo, error) {
	var ret []*RollupGapInfo
	ListRollupGapWalk(ctx, conn, func(rgi *RollupGapInfo) bool {
		if rgi.err == nil && rgi != nil {
			ret = append(ret, rgi)
		}
		return rgi.err == nil
	})
	return ret, nil
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
	sqlText := SqlTidy(`SELECT
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
	sqlText := SqlTidy(`SELECT
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

type StorageInfo struct {
	TableName string `json:"table_name"`
	DataSize  int64  `json:"data_size"`
	IndexSize int64  `json:"index_size"`
	TotalSize int64  `json:"total_size"`
	err       error  `json:"-"`
}

func (si *StorageInfo) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "DATA_SIZE", DataType: api.DataTypeInt64},
		{Name: "INDEX_SIZE", DataType: api.DataTypeInt64},
		{Name: "TOTAL_SIZE", DataType: api.DataTypeInt64},
	}
}

func (si *StorageInfo) Values() []interface{} {
	return []interface{}{
		si.TableName, si.DataSize, si.IndexSize, si.TotalSize,
	}
}

func (si *StorageInfo) Err() error {
	return si.err
}

func ListStorage(ctx context.Context, conn api.Conn) ([]*StorageInfo, error) {
	var ret []*StorageInfo
	ListStorageWalk(ctx, conn, func(si *StorageInfo) bool {
		if si.err == nil && si != nil {
			ret = append(ret, si)
		}
		return si.err == nil
	})
	return ret, nil
}

func ListStorageWalk(ctx context.Context, conn api.Conn, callback func(*StorageInfo) bool) {
	sqlText := SqlTidy(`select
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

type TableUsageInfo struct {
	TableName    string `json:"table_name"`
	StorageUsage int64  `json:"storage_usage"`
	err          error  `json:"-"`
}

func (tui *TableUsageInfo) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "STORAGE_USAGE", DataType: api.DataTypeInt64},
	}
}

func (tui *TableUsageInfo) Values() []interface{} {
	return []interface{}{
		tui.TableName, tui.StorageUsage,
	}
}

func (tui *TableUsageInfo) Err() error {
	return tui.err
}

func ListTableUsage(ctx context.Context, conn api.Conn) ([]*TableUsageInfo, error) {
	var ret []*TableUsageInfo
	ListTableUsageWalk(ctx, conn, func(tui *TableUsageInfo) bool {
		if tui.err == nil && tui != nil {
			ret = append(ret, tui)
		}
		return tui.err == nil
	})
	return ret, nil
}

func ListTableUsageWalk(ctx context.Context, conn api.Conn, callback func(*TableUsageInfo) bool) {
	sqlText := SqlTidy(`SELECT
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

type StatementInfo struct {
	ID                 int64  `json:"id"`                   // v$stmt, v$neo_stmt
	SessionID          int64  `json:"session_id"`           // v$stmt, v$neo_stmt
	State              string `json:"state"`                // v$stmt, v$neo_stmt
	Query              string `json:"query"`                // v$stmt, v$neo_stmt
	RecordSize         int64  `json:"record_size"`          // v$stmt
	IsNeo              bool   `json:"is_neo"`               // v$neo_stmt
	AppendSuccessCount int64  `json:"append_success_count"` // v$neo_stmt
	AppendFailureCount int64  `json:"append_failure_count"` // v$neo_stmt
	err                error  `json:"-"`
}

func (si *StatementInfo) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "SESSION_ID", DataType: api.DataTypeInt64},
		{Name: "STATE", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "RECORD_SIZE", DataType: api.DataTypeInt64},
		{Name: "APPEND_SUCCESS_CNT", DataType: api.DataTypeInt64},
		{Name: "APPEND_FAILURE_CNT", DataType: api.DataTypeInt64},
		{Name: "QUERY", DataType: api.DataTypeString},
	}
}

func (si *StatementInfo) Values() []interface{} {
	var typ string
	var recordSize any
	var appendSuccessCount any
	var appendFailureCount any
	if si.IsNeo {
		typ = "neo"
		appendSuccessCount = si.AppendSuccessCount
		appendFailureCount = si.AppendFailureCount
	} else {
		typ = ""
		recordSize = si.RecordSize
	}
	return []interface{}{
		si.ID, si.SessionID, si.State, typ, recordSize, appendSuccessCount, appendFailureCount, si.Query,
	}
}

func (si *StatementInfo) Err() error {
	return si.err
}

func ListStatements(ctx context.Context, conn api.Conn) ([]*StatementInfo, error) {
	var ret []*StatementInfo
	ListStatementsWalk(ctx, conn, func(si *StatementInfo) bool {
		if si.err == nil && si != nil {
			ret = append(ret, si)
		}
		return si.err == nil
	})
	return ret, nil
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

type SessionInfo struct {
	ID        int64     `json:"id"`          // v$session, v$neo_session
	UserID    int64     `json:"user_id"`     // v$session, v$neo_session
	UserName  string    `json:"user_name"`   // v$session, v$neo_session
	LoginTime time.Time `json:"login_time"`  // v$session
	MaxQPXMem int64     `json:"max_qpx_mem"` // v$session
	IsNeo     bool      `json:"is_neo"`      // v$neo_session
	StmtCount int64     `json:"stmt_count"`  // v$neo_session
	err       error     `json:"-"`
}

func (si *SessionInfo) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "USER_ID", DataType: api.DataTypeInt64},
		{Name: "USER_NAME", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "LOGIN_TIME", DataType: api.DataTypeDatetime},
		{Name: "MAX_QPX_MEM", DataType: api.DataTypeInt64},
		{Name: "STMT_COUNT", DataType: api.DataTypeInt64},
	}
}

func (si *SessionInfo) Values() []interface{} {
	if si.IsNeo {
		return []any{si.ID, si.UserID, si.UserName, "neo", nil, nil, si.StmtCount}
	} else {
		return []any{si.ID, si.UserID, si.UserName, "CLI", si.LoginTime, si.MaxQPXMem, nil}
	}
}

func (si *SessionInfo) Err() error {
	return si.err
}

func ListSessions(ctx context.Context, conn api.Conn) ([]*SessionInfo, error) {
	var ret []*SessionInfo
	ListSessionsWalk(ctx, conn, func(si *SessionInfo) bool {
		if si.err == nil && si != nil {
			ret = append(ret, si)
		}
		return si.err == nil
	})
	return ret, nil
}

func ListSessionsWalk(ctx context.Context, conn api.Conn, callback func(*SessionInfo) bool) {
	rows, err := conn.Query(ctx, `SELECT ID, USER_ID, USER_NAME, LOGIN_TIME, MAX_QPX_MEM FROM V$SESSION`)
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
		rec.err = rows.Scan(&rec.ID, &rec.UserID, &rec.UserName, &rec.LoginTime, &rec.MaxQPXMem)
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
