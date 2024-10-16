package do

import (
	"context"
	"strings"

	"github.com/machbase/neo-server/api"
)

// deprecated
func tableType(ctx context.Context, conn api.Conn, tableName string) (api.TableType, error) {
	r := conn.QueryRow(ctx, "select type from M$SYS_TABLES where name = ?", strings.ToUpper(tableName))
	var typ = 0
	if err := r.Scan(&typ); err != nil {
		return api.TableType(-1), err
	}
	return api.TableType(typ), nil
}

func TableType(ctx context.Context, conn api.Conn, fullTableName string) (api.TableType, error) {
	_, userName, tableName := tokenizeFullTableName(fullTableName)
	sql := "select type from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRow(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	var ret int
	if err := r.Scan(&ret); err != nil {
		return -1, err
	}
	return api.TableType(ret), nil
}
