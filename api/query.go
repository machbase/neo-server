package api

import (
	"context"
	"fmt"
)

type Query struct {
	// Begin is called after when query is executed successfully
	// and before when the first row is fetched.
	Begin func(q *Query)

	// Next is called for each row fetched. If Next returns false,
	// the remains of rows are ignored and fetch loop is stopped.
	Next func(q *Query, rownum int64) bool

	// End is called after when the query is finished, Or non-query execution is finished.
	End func(q *Query)

	isFetch     bool
	columns     Columns
	err         error
	userMessage string
	rowNum      int64
	rows        Rows
}

func (qc *Query) IsFetch() bool {
	return qc.isFetch
}

func (qc *Query) Err() error {
	return qc.err
}

func (qc *Query) RowNum() int64 {
	return qc.rowNum
}

func (qc *Query) UserMessage() string {
	return qc.userMessage
}

func (qc *Query) Scan(values ...any) error {
	err := qc.rows.Scan(values...)
	if err != nil {
		qc.err = err
		return err
	}
	return nil
}

// Columns returns the columns of the query result.
// If the sqlText was not a select query, it returns nil.
func (qc *Query) Columns() Columns {
	return qc.columns
}

func (qc *Query) Execute(ctx context.Context, conn Conn, sqlText string, args ...any) error {
	if r, err := conn.Query(ctx, sqlText, args...); err != nil {
		return err
	} else {
		qc.rows = r
	}
	defer qc.rows.Close()

	qc.isFetch = qc.rows.IsFetchable()

	if !qc.isFetch {
		if qc.Begin != nil {
			qc.Begin(qc)
		}
		qc.userMessage = qc.rows.Message()
		qc.rowNum = qc.rows.RowsAffected()
		if qc.End != nil {
			defer qc.End(qc)
		}
		return nil
	}

	if cols, err := qc.rows.Columns(); err != nil {
		return err
	} else {
		qc.columns = cols
	}
	if qc.Begin != nil {
		qc.Begin(qc)
	}
	if qc.End != nil {
		defer func() {
			if qc.err == nil {
				if qc.rowNum == 0 {
					qc.userMessage = "no rows fetched."
				} else if qc.rowNum == 1 {
					qc.userMessage = "a row fetched."
				} else {
					qc.userMessage = fmt.Sprintf("%d rows fetched.", qc.rowNum)
				}
			} else {
				qc.userMessage = fmt.Sprintf("QUERY %s", qc.err.Error())
			}
			qc.End(qc)
		}()
	}

	for qc.rows.Next() {
		qc.rowNum++
		if qc.Next != nil && !qc.Next(qc, qc.rowNum) {
			break
		}
	}

	return qc.err
}
