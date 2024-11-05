package api

import (
	"context"
)

func ListIndexesSql() string {
	return SqlTidy(`
		SELECT
			u.name as USER_NAME,
			j.DB_NAME as DATABASE_NAME,
			j.TABLE_NAME as TABLE_NAME,
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
				else 'LSM' 
			end as INDEX_TYPE,
			b.id as INDEX_ID
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
}

func ListIndexesWalk(ctx context.Context, conn Conn, callback func(*IndexInfo, error) bool) {
	sqlText := ListIndexesSql()
	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback(nil, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		nfo := &IndexInfo{Cols: make([]string, 1)}
		err = rows.Scan(&nfo.User, &nfo.Database, &nfo.Table, &nfo.Cols[0], &nfo.Name, &nfo.Type, &nfo.Id)
		if !callback(nfo, err) {
			return
		}
	}
}

func ListIndexes(ctx context.Context, conn Conn) (ret []*IndexInfo, cause error) {
	ListIndexesWalk(ctx, conn, func(ii *IndexInfo, err error) bool {
		if err == nil && ii != nil {
			ret = append(ret, ii)
		}
		cause = err
		return err == nil
	})
	return
}
