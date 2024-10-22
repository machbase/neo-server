package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/machbase/neo-server/api/types"
	"github.com/pkg/errors"
)

type TableInfo struct {
	Database string `json:"database"`
	User     string `json:"user"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	Flag     int    `json:"flag"`
}

func Tables(ctx context.Context, conn Conn, callback func(*TableInfo, error) bool) {
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

	rows, err := conn.Query(ctx, sqlText)
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

// deprecated
func tableType(ctx context.Context, conn Conn, tableName string) (types.TableType, error) {
	r := conn.QueryRow(ctx, "select type from M$SYS_TABLES where name = ?", strings.ToUpper(tableName))
	var typ = 0
	if err := r.Scan(&typ); err != nil {
		return types.TableType(-1), err
	}
	return types.TableType(typ), nil
}

func TableType(ctx context.Context, conn Conn, fullTableName string) (types.TableType, error) {
	_, userName, tableName := tokenizeFullTableName(fullTableName)
	sql := "select type from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRow(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	var ret int
	if err := r.Scan(&ret); err != nil {
		return -1, err
	}
	return types.TableType(ret), nil
}

func ListTables(ctx context.Context, conn Conn) []string {
	rows, err := conn.Query(ctx, "select NAME, TYPE, FLAG from M$SYS_TABLES order by NAME")
	if err != nil {
		return nil
	}
	defer rows.Close()
	rt := []string{}
	for rows.Next() {
		var name string
		var typ int
		var flg int
		rows.Scan(&name, &typ, &flg)
		rt = append(rt, name)
	}
	return rt
}

func ExistsTable(ctx context.Context, conn Conn, fullTableName string) (bool, error) {
	_, userName, tableName := tokenizeFullTableName(fullTableName)
	sql := "select count(*) from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRow(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	var count = 0
	if err := r.Scan(&count); err != nil {
		return false, err
	}
	return (count == 1), nil
}

func ExistsTableOrCreate(ctx context.Context, conn Conn, tableName string, create bool, truncate bool) (exists bool, created bool, truncated bool, err error) {
	exists, err = ExistsTable(ctx, conn, tableName)
	if err != nil {
		return
	}
	if !exists {
		// CREATE TABLE
		if create {
			// TODO table type and columns customization
			ddl := fmt.Sprintf("create tag table %s (name varchar(100) primary key, time datetime basetime, value double)", tableName)
			result := conn.Exec(ctx, ddl)
			if result.Err() != nil {
				err = result.Err()
				return
			}
			created = true
			// do not truncate newly created table.
			truncate = false
		} else {
			return
		}
	}

	// TRUNCATE TABLE
	if truncate {
		tableType, err0 := tableType(ctx, conn, tableName)
		if err0 != nil {
			err = errors.Wrap(err0, fmt.Sprintf("table '%s' doesn't exist", tableName))
			return
		}
		if tableType == types.TableTypeLog {
			result := conn.Exec(ctx, fmt.Sprintf("truncate table %s", tableName))
			if result.Err() != nil {
				err = result.Err()
				return
			}
			truncated = true
		} else {
			result := conn.Exec(ctx, fmt.Sprintf("delete from %s", tableName))
			if result.Err() != nil {
				err = result.Err()
				return
			}
			truncated = true
		}
	}
	return
}
