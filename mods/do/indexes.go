package do

import (
	"context"
	"fmt"

	spi "github.com/machbase/neo-spi"
)

func Indexes(ctx context.Context, conn spi.Conn) ([]*IndexInfo, error) {
	ret := []*IndexInfo{}

	sqlText := `select 
			u.name as USER_NAME,
			a.database_id DBID,
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
		when 11 then 'TAG'
		else 'LSM' end as INDEX_TYPE
		from
			m$sys_tables a, 
			m$sys_indexes b, 
			m$sys_index_columns c, 
			m$sys_users u
		where
			a.id = b.table_id
		and b.id = c.index_id
		and a.user_id = u.user_id
		order by a.database_id, a.name, b.name`

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		nfo := &IndexInfo{}
		err = rows.Scan(&nfo.UserName, &nfo.DatabaseId, &nfo.TableName, &nfo.ColumnName, &nfo.IndexName, &nfo.IndexType)
		if err != nil {
			rows.Close()
			return nil, err
		}
		ret = append(ret, nfo)
	}
	rows.Close()

	dbs := map[int]string{-1: "MACHBASEDB"}
	for _, r := range ret {
		name, ok := dbs[r.DatabaseId]
		if ok {
			r.DatabaseName = name
		} else {
			row := conn.QueryRow(ctx, "select MOUNTDB from V$STORAGE_MOUNT_DATABASES where BACKUP_TBSID = ?", r.DatabaseId)
			if err := row.Scan(&name); err != nil {
				r.DatabaseName = fmt.Sprintf("[%d]", r.DatabaseId)
			} else {
				dbs[r.DatabaseId] = name
				r.DatabaseName = name
			}
		}
	}
	return ret, nil
}

type IndexInfo struct {
	UserName     string
	DatabaseId   int
	DatabaseName string
	TableName    string
	ColumnName   string
	IndexName    string
	IndexType    string
}
