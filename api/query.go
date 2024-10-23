package api

import (
	"context"
	"fmt"

	"github.com/machbase/neo-server/api/types"
)

type Query struct {
	// Begin is called after when query is executed successfully
	// and before when the first row is fetched.
	Begin func(q *Query)

	// Next is called for each row fetched. If Next returns false,
	// the remains of rows are ignored and fetch loop is stopped.
	Next func(q *Query, rownum int64, values []any) bool

	// End is called after when the query is finished, Or non-query execution is finished.
	End func(q *Query, userMessage string, numRows int64)

	isFetch bool
	columns types.Columns
}

func (qc *Query) Execute(ctx context.Context, conn Conn, sqlText string, args ...any) error {
	return query(qc, ctx, conn, sqlText, args...)
}

func (qc *Query) IsFetch() bool {
	return qc.isFetch
}

// Columns returns the columns of the query result.
// If the sqlText was not a select query, it returns nil.
func (qc *Query) Columns() types.Columns {
	return qc.columns
}

func query(query *Query, ctx context.Context, conn Conn, sqlText string, args ...any) error {
	rows, err := conn.Query(ctx, sqlText, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	query.isFetch = rows.IsFetchable()

	var numRows int64
	var userMessage string

	if !query.isFetch {
		if query.Begin != nil {
			query.Begin(query)
		}
		userMessage = rows.Message()
		numRows = rows.RowsAffected()
		if query.End != nil {
			defer query.End(query, userMessage, numRows)
		}
		return nil
	}

	if cols, err := RowsColumns(rows); err != nil {
		return err
	} else {
		query.columns = cols
	}
	if query.Begin != nil {
		query.Begin(query)
	}
	if query.End != nil {
		defer func() {
			if numRows == 0 {
				userMessage = "no row fetched."
			} else if numRows == 1 {
				userMessage = "a row fetched."
			} else {
				userMessage = fmt.Sprintf("%d rows fetched.", numRows)
			}
			query.End(query, userMessage, numRows)
		}()
	}

	for rows.Next() {
		rec, err := query.columns.MakeBuffer()
		if err != nil {
			return err
		}
		err = rows.Scan(rec...)
		if err != nil {
			return err
		}
		numRows++
		if query.Next != nil && !query.Next(query, numRows, rec) {
			break
		}
	}

	return nil
}
