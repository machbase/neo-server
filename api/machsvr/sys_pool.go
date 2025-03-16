package machsvr

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/api"
)

var enableWorkerPool bool
var pool chan *Worker
var poolSize int
var poolSizeHardLimit = runtime.NumCPU() * 10
var poolSizeSoftThreshold = 2
var poolDB *Database

func useWorkerPool() bool {
	return enableWorkerPool && len(pool) > poolSizeSoftThreshold
}

func SetWorkerPoolSize(size int) {
	if size <= 0 {
		size = runtime.NumCPU()
	} else if size > poolSizeHardLimit {
		size = poolSizeHardLimit
	}

	prevSize := poolSize
	poolSize = size
	if pool == nil || (poolSize > 0 && poolSize == prevSize) {
		return
	}
	if poolSize > prevSize {
		for i := poolSize - prevSize; i > 0; i-- {
			w := makeWorker()
			w.Start()
			pool <- w
		}
	} else if poolSize < prevSize {
		for i := prevSize - poolSize; i > 0; i-- {
			w := <-pool
			w.Stop()
		}
	}
	if poolSize < 4 {
		poolSizeSoftThreshold = 0
	} else {
		poolSizeSoftThreshold = 2
	}
}

func WorkerPoolSize() int {
	return poolSize
}

func makeWorker() *Worker {
	return &Worker{
		db:         poolDB,
		requestCh:  make(chan any),
		responseCh: make(chan any),
		closeCh:    make(chan struct{}),
		createdAt:  time.Now(),
	}
}

func StartWorkerPool(db *Database) {
	if poolSize == 0 {
		poolSize = runtime.NumCPU()
	}
	poolDB = db
	pool = make(chan *Worker, poolSizeHardLimit)
	for i := 0; i < poolSize; i++ {
		w := makeWorker()
		w.Start()
		pool <- w
	}
	enableWorkerPool = true
}

func StopWorkerPool() {
	enableWorkerPool = false
	for i := 0; i < poolSize; i++ {
		w := <-pool
		w.Stop()
	}
}

func workPool(req any) any {
	w := <-pool
	// send req
	w.requestCh <- req
	// recv rsp in the same struct
	req = <-w.responseCh
	// task has been done, put back to the pool
	pool <- w
	return req
}

type Worker struct {
	db         *Database
	requestCh  chan any
	responseCh chan any
	usedCount  uint64
	closeCh    chan struct{}
	closeWg    sync.WaitGroup
	createdAt  time.Time
}

func (w *Worker) Start() {
	w.closeWg.Add(1)
	go func() {
		defer w.closeWg.Done()
		runtime.LockOSThread()
		// intentionally ignore calling runtime.UnlockOSThread()
		// to terminate the native thread
	loop:
		for {
			select {
			case req := <-w.requestCh:
				w.handle(req)
			case <-w.closeCh:
				break loop
			}
		}
		for len(w.requestCh) > 0 {
			req := <-w.requestCh
			w.handle(req)
		}
	}()
}

func (w *Worker) Stop() {
	close(w.closeCh)
	w.closeWg.Wait()
}

func (w *Worker) handle(req any) {
	w.usedCount++
	switch r := req.(type) {
	case *ConnectWork:
		r.conn, r.err = w.db.ConnectSync(r.ctx, r.opts...)
		w.responseCh <- r
	case *ConnCloseWork:
		r.err = r.conn.CloseSync()
		w.responseCh <- r
	case *ExecWork:
		r.result = r.conn.ExecSync(r.ctx, r.sqlText, r.params...)
		w.responseCh <- r
	case *QueryWork:
		r.rows, r.err = r.conn.QuerySync(r.ctx, r.sqlText, r.params...)
		w.responseCh <- r
	case *QueryRowWork:
		r.row = r.conn.QueryRowSync(r.ctx, r.sqlText, r.params...)
		w.responseCh <- r
	case *RowsFetchWork:
		r.values, r.next, r.err = r.rows.FetchSync()
		w.responseCh <- r
	case *RowsNextWork:
		r.next = r.rows.NextSync()
		w.responseCh <- r
	case *RowsScanWork:
		r.err = r.rows.ScanSync(r.values...)
		w.responseCh <- r
	case *RowsAffectedWork:
		r.affected = r.rows.RowsAffectedSync()
		w.responseCh <- r
	case *RowsCloseWork:
		r.err = r.rows.CloseSync()
		w.responseCh <- r
	case *ExplainWork:
		r.explain, r.err = r.conn.ExplainSync(r.ctx, r.sqlText, r.full)
		w.responseCh <- r
	case *AppenderOpenWork:
		r.appender, r.err = r.conn.AppenderSync(r.ctx, r.table, r.opts...)
		w.responseCh <- r
	case *AppenderCloseWork:
		r.success, r.failure, r.err = r.appender.CloseSync()
		w.responseCh <- r
	case *AppendWork:
		r.err = r.appender.AppendSync(r.values...)
		w.responseCh <- r
	case *AppendLogTimeWork:
		r.err = r.appender.AppendLogTimeSync(r.ts, r.values...)
		w.responseCh <- r
	default:
		w.responseCh <- fmt.Errorf("unknown worker pool type %T", r)
	}
}

type ConnectWork struct {
	// request
	ctx  context.Context
	opts []api.ConnectOption

	// result
	conn api.Conn
	err  error
}

type ConnCloseWork struct {
	// request
	conn *Conn
	// result
	err error
}

type ExecWork struct {
	// request
	ctx     context.Context
	conn    *Conn
	sqlText string
	params  []any
	// result
	result api.Result
}

type QueryWork struct {
	// request
	ctx     context.Context
	conn    *Conn
	sqlText string
	params  []any
	// result
	rows api.Rows
	err  error
}

type QueryRowWork struct {
	// request
	ctx     context.Context
	conn    *Conn
	sqlText string
	params  []any
	// result
	row api.Row
}

type RowsFetchWork struct {
	// request
	rows *Rows
	// result
	values []any
	next   bool
	err    error
}

type RowsNextWork struct {
	// request
	rows *Rows
	// result
	next bool
}

type RowsScanWork struct {
	// request
	rows   *Rows
	values []any
	// result
	err error
}

type RowsAffectedWork struct {
	// request
	rows *Rows
	// result
	affected int64
}

type RowsCloseWork struct {
	// request
	rows *Rows
	// result
	err error
}

type ExplainWork struct {
	// request
	ctx     context.Context
	conn    *Conn
	sqlText string
	full    bool
	// result
	explain string
	err     error
}

type AppenderOpenWork struct {
	// request
	ctx   context.Context
	conn  *Conn
	table string
	opts  []api.AppenderOption
	// result
	appender api.Appender
	err      error
}

type AppenderCloseWork struct {
	// request
	appender *Appender
	// result
	success int64
	failure int64
	err     error
}

type AppendWork struct {
	// request
	appender *Appender
	values   []any
	// result
	err error
}

type AppendLogTimeWork struct {
	// request
	appender *Appender
	ts       time.Time
	values   []any
	// result
	err error
}
