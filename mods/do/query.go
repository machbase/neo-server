package do

import (
	"fmt"

	spi "github.com/machbase/neo-spi"
)

type QueryContext struct {
	DB           spi.Database
	OnFetchStart func(spi.Columns)
	OnFetch      func(rownum int64, values []any) bool
	OnFetchEnd   func()
}

func Query(ctx *QueryContext, sqlText string, args ...any) (string, error) {
	rows, err := ctx.DB.Query(sqlText, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	if !rows.IsFetchable() {
		return rows.Message(), nil
	}

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	if ctx.OnFetchStart != nil {
		ctx.OnFetchStart(cols)
	}
	if ctx.OnFetchEnd != nil {
		defer ctx.OnFetchEnd()
	}

	var nrow int64
	for rows.Next() {
		rec := cols.MakeBuffer()
		err = rows.Scan(rec...)
		if err != nil {
			return "", err
		}
		nrow++
		if ctx.OnFetch != nil && !ctx.OnFetch(nrow, rec) {
			break
		}
	}

	var usermsg = rows.Message()
	if nrow == 0 {
		usermsg = "no row fetched."
	} else if nrow == 1 {
		usermsg = "a row fetched."
	} else {
		usermsg = fmt.Sprintf("%d rows fetched.", nrow)
	}
	return usermsg, nil
}
