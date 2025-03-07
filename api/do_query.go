package api

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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

	// Stat is called after when the query iteration is finished.
	Stat func(m *QueryMeter)

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
	meter := NewQueryMeter()
	if r, err := conn.Query(ctx, sqlText, args...); err != nil {
		return err
	} else {
		qc.rows = r
	}
	defer func() {
		qc.rows.Close()
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

	meter.MarkExecute(sqlText, args)

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
	defer func() {
		meter.MarkFetch()

		if qc.Stat != nil {
			qc.Stat(meter)
		}
	}()

	if pql, ok := qc.rows.(QueryLimiter); ok {
		timeCtx, timeCancel := context.WithTimeout(ctx, 60*time.Second)
		if !pql.QueryLimit(timeCtx) {
			timeCancel()
			qc.err = fmt.Errorf("query limit exceeded")
			return qc.err
		}
		timeCancel()
		meter.MarkLimitWait()
	}
	for qc.rows.Next() {
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

type QueryMeter struct {
	ts        time.Time
	SqlText   string
	SqlArgs   []any
	Execute   time.Duration
	LimitWait time.Duration
	Fetch     time.Duration
}

func NewQueryMeter() *QueryMeter {
	metricQueryCount.Add(1)
	return &QueryMeter{ts: time.Now()}
}

func (m *QueryMeter) MarkExecute(sqlText string, args []any) {
	m.SqlText = sqlText
	m.SqlArgs = args
	m.Execute = time.Since(m.ts)
	metricQueryExecuteElapse.Add(m.Execute)
}

func (m *QueryMeter) MarkLimitWait() {
	m.LimitWait = time.Since(m.ts) - m.Execute
	metricQueryLimitWait.Add(m.LimitWait)
}

func (m *QueryMeter) MarkFetch() {
	m.Fetch = time.Since(m.ts) - m.Execute - m.LimitWait
	metricQueryFetchElapse.Add(m.Fetch)

	elapse := int64(m.Execute + m.LimitWait + m.Fetch)
	// update high water mark of query elapse
	if queryElapseHwm.Load() < elapse {
		queryElapseHwm.Store(elapse)

		queryElapseHwmLock.Lock()
		defer queryElapseHwmLock.Unlock()

		metricQueryHwmSqlText.Set(m.SqlText)
		metricQueryHwmSqlArgs.Set(fmt.Sprintf("%v", m.SqlArgs))
		metricQueryHwmElapse.Set(int64(m.Execute + m.LimitWait + m.Fetch))
		metricQueryHwmLimitWait.Set(int64(m.LimitWait))
		metricQueryHwmFetchElapse.Set(int64(m.Fetch))
		metricQueryHwmExecuteElapse.Set(int64(m.Execute))
	}
}

var queryElapseHwmLock sync.Mutex
var queryElapseHwm atomic.Int64
