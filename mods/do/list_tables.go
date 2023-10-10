package do

import (
	"context"

	spi "github.com/machbase/neo-spi"
)

func ListTables(ctx context.Context, conn spi.Conn) []string {
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
