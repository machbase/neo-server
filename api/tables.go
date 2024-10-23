package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/machbase/neo-server/api/types"
	"github.com/pkg/errors"
)

type TableInfo struct {
	Database string          `json:"database"`
	User     string          `json:"user"`
	Name     string          `json:"name"`
	Type     types.TableType `json:"type"`
	Flag     types.TableFlag `json:"flag"`
}

func (ti *TableInfo) Kind() string {
	return TableTypeDescription(ti.Type, ti.Flag)
}

func Tables(ctx context.Context, conn Conn, callback func(*TableInfo, error) bool) {
	sqlText := SqlTidy(
		`SELECT
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
		`)

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

func TableType(ctx context.Context, conn Conn, fullTableName string) (types.TableType, error) {
	_, userName, tableName := TokenizeFullTableName(fullTableName)
	sql := "select type from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRow(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	var ret types.TableType
	if err := r.Scan(&ret); err != nil {
		return -1, err
	}
	return ret, nil
}

func ExistsTable(ctx context.Context, conn Conn, fullTableName string) (bool, error) {
	_, userName, tableName := TokenizeFullTableName(fullTableName)
	sql := "select count(*) from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRow(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
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
	if truncate {
		tableType, err0 := TableType(ctx, conn, fullTableName)
		if err0 != nil {
			err = errors.Wrap(err0, fmt.Sprintf("table '%s' doesn't exist", fullTableName))
			return
		}
		if tableType == types.TableTypeLog {
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
	}
	return
}
