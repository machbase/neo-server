package do

import (
	"context"
	"strings"

	spi "github.com/machbase/neo-spi"
)

func TableType(ctx context.Context, conn spi.Conn, tableName string) (spi.TableType, error) {
	r := conn.QueryRow(ctx, "select type from M$SYS_TABLES where name = ?", strings.ToUpper(tableName))
	var typ = 0
	if err := r.Scan(&typ); err != nil {
		return spi.TableType(-1), err
	}
	return spi.TableType(typ), nil
}
