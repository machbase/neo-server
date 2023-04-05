package do

import (
	"fmt"
	"strings"

	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

func ExistsTable(db spi.Database, tableName string) (bool, error) {
	r := db.QueryRow("select count(*) from M$SYS_TABLES where name = ?", strings.ToUpper(tableName))
	var count = 0
	if err := r.Scan(&count); err != nil {
		return false, err
	}
	return (count == 1), nil
}

func ExistsTableOrCreate(db spi.Database, tableName string, create bool, truncate bool) (exists bool, created bool, truncated bool, err error) {
	exists, err = ExistsTable(db, tableName)
	if err != nil {
		return
	}
	if !exists {
		// CREATE TABLE
		if create {
			// TODO table type and columns customization
			ddl := fmt.Sprintf("create tag table %s (name varchar(100) primary key, time datetime basetime, value double)", tableName)
			result := db.Exec(ddl)
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
		tableType, err0 := TableType(db, tableName)
		if err0 != nil {
			err = errors.Wrap(err0, fmt.Sprintf("table '%s' doesn't exist", tableName))
			return
		}
		if tableType == spi.LogTableType {
			result := db.Exec(fmt.Sprintf("truncate table %s", tableName))
			if result.Err() != nil {
				err = result.Err()
				return
			}
			truncated = true
		} else {
			result := db.Exec(fmt.Sprintf("delete from %s", tableName))
			if result.Err() != nil {
				err = result.Err()
				return
			}
			truncated = true
		}
	}
	return
}
