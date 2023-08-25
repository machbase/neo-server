package do

import (
	"fmt"
	"strings"

	spi "github.com/machbase/neo-spi"
)

func Tags(db spi.Database, table string, callback func(string, error) bool) {
	sqlText := fmt.Sprintf("select * from _%s_META", strings.ToUpper(table))
	rows, err := db.Query(sqlText)
	if err != nil {
		callback("", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		name := ""
		err := rows.Scan(&name)
		if err != nil {
			callback("", err)
			return
		}
		if !callback(name, nil) {
			return
		}
	}
}
