package api

import (
	"context"
	"fmt"
	"time"
)

type Query struct {
	// Begin is called after when query is executed successfully
	// and before when the first row is fetched.
	Begin func(q *Query)

	// Next is called for each row fetched. If Next returns false,
	// the remains of rows are ignored and fetch loop is stopped.
	Next func(q *Query, rownum int64) bool

	// End is called after when the query is finished, Or non-query execution is finished.
	// If the query is cancelled, End is not called.
	End func(q *Query)

	isFetch     bool
	columns     Columns
	err         error
	userMessage string
	rowNum      int64
	rows        Rows
	startWait   chan struct{}
	cancelWait  chan struct{}
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

type QueryResult struct {
	Err error
}

func (qc *Query) Run(ctx context.Context, conn Conn, sqlText string, args ...any) <-chan QueryResult {
	qc.startWait = make(chan struct{})
	ch := make(chan QueryResult)
	go func() {
		defer close(ch)
		if err := qc.Execute(ctx, conn, sqlText, args...); err != nil {
			ch <- QueryResult{Err: err}
			return
		}
		ch <- QueryResult{}
	}()
	// If the HTTP context is closed before the go-routine starts,
	// the connection might already be closed by the time qc.Execute() is executed.
	// So, we need to wait for the go-routine to start before returning the channel.
	<-qc.startWait
	return ch
}

func (qc *Query) Cancel() {
	if qc.rows == nil {
		return
	}
	qc.cancelWait = make(chan struct{})
	select {
	case <-qc.cancelWait:
	case <-time.After(60 * time.Second):
	}
}

func (qc *Query) Execute(ctx context.Context, conn Conn, sqlText string, args ...any) error {
	if r, err := conn.Query(ctx, sqlText, args...); err != nil {
		if qc.startWait != nil {
			close(qc.startWait)
		}
		return err
	} else {
		qc.rows = r
		if qc.startWait != nil {
			close(qc.startWait)
		}
	}
	defer func() {
		qc.rows.Close()
		if qc.cancelWait != nil {
			close(qc.cancelWait)
		}
	}()

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
			if qc.cancelWait != nil {
				return
			}
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

	if pql, ok := qc.rows.(QueryLimiter); ok {
		timeCtx, timeCancel := context.WithTimeout(ctx, 60*time.Second)
		if !pql.QueryLimit(timeCtx) {
			timeCancel()
			qc.err = fmt.Errorf("query limit exceeded")
			return qc.err
		}
		timeCancel()
	}
	for qc.cancelWait == nil && qc.rows.Next() {
		qc.rowNum++
		if qc.Next != nil && !qc.Next(qc, qc.rowNum) {
			break
		}
	}

	return qc.err
}

type QueryLimiter interface {
	QueryLimit(context.Context) bool
}
