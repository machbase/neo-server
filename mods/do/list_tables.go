package do

import spi "github.com/machbase/neo-spi"

func ListTables(db spi.Database) []string {
	rows, err := db.Query("select NAME, TYPE, FLAG from M$SYS_TABLES order by NAME")
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
