package api

import (
	"context"
	"fmt"

	"github.com/machbase/neo-server/api/types"
)

type QueryContext struct {
	Conn         Conn
	Ctx          context.Context
	OnFetchStart func(types.Columns)
	OnFetch      func(rownum int64, values []any) bool
	OnFetchEnd   func()
	OnExecuted   func(userMessage string, rowsAffected int64) // callback if query is not a fetchable (e.g: create/drop table)
}

func Query(ctx *QueryContext, sqlText string, args ...any) (string, error) {
	rows, err := ctx.Conn.Query(ctx.Ctx, sqlText, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	if !rows.IsFetchable() {
		if ctx.OnExecuted != nil {
			ctx.OnExecuted(rows.Message(), rows.RowsAffected())
		}
		return rows.Message(), nil
	}

	cols, err := RowsColumns(rows)
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
		rec, err := cols.MakeBuffer()
		if err != nil {
			return "", err
		}
		err = rows.Scan(rec...)
		if err != nil {
			return "", err
		}
		nrow++
		if ctx.OnFetch != nil && !ctx.OnFetch(nrow, rec) {
			break
		}
	}

	var userMessage = rows.Message()
	if nrow == 0 {
		userMessage = "no row fetched."
	} else if nrow == 1 {
		userMessage = "a row fetched."
	} else {
		userMessage = fmt.Sprintf("%d rows fetched.", nrow)
	}
	return userMessage, nil
}
