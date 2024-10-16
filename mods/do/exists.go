package do

import (
	"context"
	"fmt"
	"strings"

	"github.com/machbase/neo-server/api"
	"github.com/pkg/errors"
)

func ExistsTable(ctx context.Context, conn api.Conn, fullTableName string) (bool, error) {
	_, userName, tableName := tokenizeFullTableName(fullTableName)
	sql := "select count(*) from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRow(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	var count = 0
	if err := r.Scan(&count); err != nil {
		return false, err
	}
	return (count == 1), nil
}

func ExistsTableOrCreate(ctx context.Context, conn api.Conn, tableName string, create bool, truncate bool) (exists bool, created bool, truncated bool, err error) {
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
		if tableType == api.LogTableType {
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
