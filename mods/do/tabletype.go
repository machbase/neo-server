package do

import (
	"strings"

	spi "github.com/machbase/neo-spi"
)

func TableType(db spi.Database, tableName string) (spi.TableType, error) {
	r := db.QueryRow("select type from M$SYS_TABLES where name = ?", strings.ToUpper(tableName))
	var typ = 0
	if err := r.Scan(&typ); err != nil {
		return spi.TableType(-1), err
	}
	return spi.TableType(typ), nil
}
