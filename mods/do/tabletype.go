package do

import (
	"context"
	"strings"

	"github.com/machbase/neo-server/api"
)

func TableType(ctx context.Context, conn api.Conn, tableName string) (api.TableType, error) {
	r := conn.QueryRow(ctx, "select type from M$SYS_TABLES where name = ?", strings.ToUpper(tableName))
	var typ = 0
	if err := r.Scan(&typ); err != nil {
		return api.TableType(-1), err
	}
	return api.TableType(typ), nil
}
