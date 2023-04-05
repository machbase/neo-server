package do

import (
	"fmt"
	"strings"

	spi "github.com/machbase/neo-spi"
)

func Insert(db spi.Database, tableName string, columns []string, rows [][]any) spi.Result {
	if len(rows) == 0 {
		// fields의 value가 string 일 경우 입력할 값이 없을 수 있다.
		return &InsertResult{
			rowsAffected: 0,
			message:      "no row inserted",
		}
	}

	vf := make([]string, len(columns))
	for i := range vf {
		vf[i] = "?"
	}
	valuesPlaces := strings.Join(vf, ",")
	columnsPhrase := strings.Join(columns, ",")

	sqlText := fmt.Sprintf("insert into %s (%s) values(%s)", tableName, columnsPhrase, valuesPlaces)
	var nrows int64
	for _, rec := range rows {
		result := db.Exec(sqlText, rec...)
		if result.Err() != nil {
			return &InsertResult{
				err:          result.Err(),
				rowsAffected: nrows,
				message:      "batch inserts aborted by error",
			}
		}
		nrows++
	}

	message := fmt.Sprintf("%d rows inserted", nrows)
	if nrows == 0 {
		message = "no row inserted"
	} else if nrows == 1 {
		message = "a row inserted"
	}
	return &InsertResult{
		rowsAffected: nrows,
		message:      message,
	}
}

// implements spi.Result
type InsertResult struct {
	err          error
	rowsAffected int64
	message      string
}

func (ir *InsertResult) Err() error {
	return ir.err
}

func (ir *InsertResult) RowsAffected() int64 {
	return ir.rowsAffected
}

func (ir *InsertResult) Message() string {
	return ir.message
}
