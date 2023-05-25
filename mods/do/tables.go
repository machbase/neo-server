package do

import (
	spi "github.com/machbase/neo-spi"
)

type TableInfo struct {
	Database string `json:"database"`
	User     string `json:"user"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	Flag     int    `json:"flag"`
}

func Tables(db spi.Database, callback func(*TableInfo, error) bool) {
	sqlText := `SELECT
			j.DB_NAME as DB_NAME,
			u.NAME as USER_NAME,
			j.NAME as TABLE_NAME,
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG
		from
			M$SYS_USERS u,
			(select
				a.NAME as NAME,
				a.USER_ID as USER_ID,
				a.TYPE as TYPE,
				a.FLAG as FLAG,
				case a.DATABASE_ID
					when -1 then 'MACHBASEDB'
					else d.MOUNTDB
				end as DB_NAME
			from M$SYS_TABLES a
				left join V$STORAGE_MOUNT_DATABASES d on a.DATABASE_ID = d.BACKUP_TBSID) as j
		where
			u.USER_ID = j.USER_ID
		order by j.NAME
		`

	rows, err := db.Query(sqlText)
	if err != nil {
		callback(nil, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		ti := &TableInfo{}
		err := rows.Scan(&ti.Database, &ti.User, &ti.Name, &ti.Type, &ti.Flag)
		if err != nil {
			callback(nil, err)
			return
		}
		if !callback(ti, nil) {
			return
		}
	}
}
