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

type RollupGapInfo struct {
	SrcTable     string        `json:"src_table"`
	RollupTable  string        `json:"rollup_table"`
	SrcEndRID    int64         `json:"src_end_rid"`
	RollupEndRID int64         `json:"rollup_end_rid"`
	Gap          int64         `json:"gap"`
	LastElapsed  time.Duration `json:"last_time"`
	err          error         `json:"-"`
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
