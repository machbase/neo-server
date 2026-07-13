package spi

import (
	"context"
	"fmt"
	"strings"

	"github.com/machbase/neo-client/api"
)

type IndexInfo struct {
	Id             int64  `json:"id"`
	Database       string `json:"database"`
	DatabaseId     int64  `json:"database_id,omitempty"`
	User           string `json:"user"`
	TableName      string `json:"table_name"`
	ColumnName     string `json:"column_name"`
	IndexName      string `json:"index_name"`
	IndexType      string `json:"index_type"`
	KeyCompress    string `json:"key_compress"`
	MaxLevel       int64  `json:"max_level"`
	PartValueCount int64  `json:"part_value_count"`
	BitMapEncode   string `json:"bitmap_encode"`
	err            error  `json:"-"`
}

var listIndexesSql = SqlTidy(`
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
	`)

func ListIndexesWalk(ctx context.Context, conn api.Conn, callback func(*IndexInfo) bool) {
	rows, err := conn.Query(ctx, listIndexesSql)
	if err != nil {
		callback(&IndexInfo{err: err})
		return
	}
	defer rows.Close()

	for rows.Next() {
		nfo := &IndexInfo{}
		nfo.err = rows.Scan(
			&nfo.User, &nfo.Database, &nfo.TableName, &nfo.ColumnName,
			&nfo.IndexName, &nfo.Id, &nfo.IndexType, &nfo.KeyCompress,
			&nfo.MaxLevel, &nfo.PartValueCount, &nfo.BitMapEncode)
		if !callback(nfo) {
			return
		}
	}
}

func ListIndexes(ctx context.Context, conn api.Conn) (ret []*IndexInfo, cause error) {
	ListIndexesWalk(ctx, conn, func(ii *IndexInfo) bool {
		if ii.err == nil && ii != nil {
			ret = append(ret, ii)
		}
		cause = ii.err
		return ii.err == nil
	})
	return
}

func DescribeIndex(ctx context.Context, conn api.Conn, name string) (*IndexInfo, error) {
	sqlText := `select 
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
	and b.name = '%s'`
	sqlText = fmt.Sprintf(sqlText, strings.ToUpper(name))
	row := conn.QueryRow(ctx, sqlText)
	if row.Err() != nil {
		return nil, row.Err()
	}
	nfo := &IndexInfo{}
	nfo.err = row.Scan(
		&nfo.TableName, &nfo.ColumnName, &nfo.IndexName, &nfo.IndexType,
		&nfo.KeyCompress, &nfo.MaxLevel, &nfo.PartValueCount, &nfo.BitMapEncode)
	return nfo, nfo.err
}

type IndexGapInfo struct {
	ID         int64  `json:"id"`         // indexgap, tagindexgap
	TableName  string `json:"table_name"` // indexgap
	IndexName  string `json:"index_name"` // indexgap
	Gap        int64  `json:"gap"`        // indexgap
	IsTagIndex bool   `json:"is_tag_index"`
	Status     string `json:"status"`     // tagindexgap
	DiskGap    int64  `json:"disk_gap"`   // tagindexgap
	MemoryGap  int64  `json:"memory_gap"` // tagindexgap
	err        error  `json:"-"`
}

func (igi *IndexGapInfo) Columns() api.Columns {
	if igi.IsTagIndex {
		return api.Columns{
			{Name: "ID", DataType: api.DataTypeInt64},
			{Name: "STATUS", DataType: api.DataTypeString},
			{Name: "DISK_GAP", DataType: api.DataTypeInt64},
			{Name: "MEMORY_GAP", DataType: api.DataTypeInt64},
		}
	} else {
		return api.Columns{
			{Name: "ID", DataType: api.DataTypeInt64},
			{Name: "TABLE", DataType: api.DataTypeString},
			{Name: "INDEX", DataType: api.DataTypeString},
			{Name: "GAP", DataType: api.DataTypeInt64},
		}
	}
}

func (igi *IndexGapInfo) Values() []interface{} {
	if igi.IsTagIndex {
		return []interface{}{
			igi.ID, igi.Status, igi.DiskGap, igi.MemoryGap,
		}
	} else {
		return []interface{}{
			igi.ID, igi.TableName, igi.IndexName, igi.Gap,
		}
	}
}

func (igi *IndexGapInfo) Err() error {
	return igi.err
}

func ListIndexGap(ctx context.Context, conn api.Conn) ([]*IndexGapInfo, error) {
	var ret []*IndexGapInfo
	ListIndexGapWalk(ctx, conn, func(igi *IndexGapInfo) bool {
		if igi.err == nil && igi != nil {
			ret = append(ret, igi)
		}
		return igi.err == nil
	})
	return ret, nil
}

func ListTagIndexGap(ctx context.Context, conn api.Conn) ([]*IndexGapInfo, error) {
	var ret []*IndexGapInfo
	ListTagIndexGapWalk(ctx, conn, func(igi *IndexGapInfo) bool {
		if igi.err == nil && igi != nil {
			ret = append(ret, igi)
		}
		return igi.err == nil
	})
	return ret, nil
}

func ListIndexGapWalk(ctx context.Context, conn api.Conn, callback func(*IndexGapInfo) bool) {
	sqlText := SqlTidy(`select
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
	order by 3 desc`)

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(&IndexGapInfo{err: err})
	}
	defer rows.Close()

	for rows.Next() {
		rec := &IndexGapInfo{}
		rec.err = rows.Scan(&rec.ID, &rec.TableName, &rec.IndexName, &rec.Gap)
		if !callback(rec) {
			return
		}
	}
}

func ListTagIndexGapWalk(ctx context.Context, conn api.Conn, callback func(*IndexGapInfo) bool) {
	sqlText := SqlTidy(`SELECT
		ID,
		INDEX_STATE AS STATUS,
		TABLE_END_RID - DISK_INDEX_END_RID AS DISK_GAP,
		TABLE_END_RID - MEMORY_INDEX_END_RID AS MEMORY_GAP
	FROM
		V$STORAGE_TAG_TABLES
	ORDER BY 1`)

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(&IndexGapInfo{err: err, IsTagIndex: true})
		return
	}
	defer rows.Close()

	for rows.Next() {
		rec := &IndexGapInfo{IsTagIndex: true}
		err := rows.Scan(&rec.ID, &rec.Status, &rec.DiskGap, &rec.MemoryGap)
		if err != nil {
			rec.err = err
		}
		if !callback(rec) {
			return
		}
	}
}
