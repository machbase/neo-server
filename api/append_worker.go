package api

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/machbase/neo-server/v8/mods/logging"
)

type AppendWorker struct {
	conn      Conn
	appender  Appender
	ctx       context.Context
	ctxCancel context.CancelFunc

	tableDesc *TableDescription
	lastTime  time.Time
	refCount  int32
	// append runner
	appendC    chan []interface{}
	appendStop chan struct{}
	appendWg   sync.WaitGroup
	log        logging.Log
}

var appenders map[string]*AppendWorker
var appendersLock sync.Mutex
var appendersFlusher chan struct{}
var appendersFlusherWg sync.WaitGroup

func StartAppendWorkers() {
	appenders = make(map[string]*AppendWorker)
	appendersFlusher = make(chan struct{})
	appendersFlusherWg.Add(1)
	go func() {
		defer appendersFlusherWg.Done()
		for {
			select {
			case <-time.After(15 * time.Second):
				appendersLock.Lock()
				var deleting []string
				for tableName, value := range appenders {
					if !value.lastTime.IsZero() && time.Since(value.lastTime) > 30*time.Second && atomic.LoadInt32(&value.refCount) == 0 {
						value.Stop()
						deleting = append(deleting, tableName)
					}
				}
				for _, tableName := range deleting {
					delete(appenders, tableName)
				}
				appendersLock.Unlock()
			case <-appendersFlusher:
				return
			}
		}
	}()
}

func StopAppendWorkers() {
	close(appendersFlusher)
	appendersFlusherWg.Wait()
	for _, value := range appenders {
		value.Stop()
	}
}

// FlushAppendWorkers flushes all append workers
// tables: table names to flush
// if tables is empty, flush all append workers
func FlushAppendWorkers(tables ...string) {
	appendersLock.Lock()
	defer appendersLock.Unlock()
	if len(tables) == 0 {
		for _, value := range appenders {
			value.Stop()
		}
		appenders = make(map[string]*AppendWorker)
	} else {
		var deleting []string
		for _, tableName := range tables {
			if value, exists := appenders[tableName]; exists {
				value.Stop()
				deleting = append(deleting, tableName)
			}
		}
		for _, tableName := range deleting {
			delete(appenders, tableName)
		}
	}
}

func GetAppendWorker(ctx context.Context, db Database, tableName string) (*AppendWorker, error) {
	appendersLock.Lock()
	defer appendersLock.Unlock()

	if aw, exists := appenders[tableName]; exists {
		aw.lastTime = time.Now()
		atomic.AddInt32(&aw.refCount, 1)
		return aw, nil
	}

	trustConn, err := db.Connect(ctx, WithTrustUser("sys"))
	if err != nil {
		return nil, err
	}
	defer trustConn.Close()

	tableDesc, err := DescribeTable(ctx, trustConn, tableName, false)
	if err != nil {
		return nil, err
	}

	appendConn, err := db.Connect(ctx, WithTrustUser("sys"))
	if err != nil {
		return nil, err
	}

	appender, err := appendConn.Appender(ctx, tableName)
	if err != nil {
		return nil, err
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	ret := &AppendWorker{
		ctx:       ctx,
		ctxCancel: ctxCancel,
		conn:      appendConn,
		appender:  appender,
		tableDesc: tableDesc,
		lastTime:  time.Now(),
		refCount:  1,
		log:       logging.GetLog(fmt.Sprintf("appender-%s", strings.ToLower(tableName))),
	}
	appenders[tableName] = ret
	ret.Start()
	return ret, nil
}

func (aw *AppendWorker) Start() {
	aw.appendC = make(chan []interface{}, 1000)
	aw.appendStop = make(chan struct{})
	aw.appendWg.Add(1)
	go func(aw *AppendWorker) {
		defer aw.appendWg.Done()
		runtime.LockOSThread()
		// !!IMPORTANT!!
		// machsvr.Appender implements "AppendSync()" which is at least x50 faster than "Append()"
		// ONLY when it is called by a dedicated native thread,
		// which is the case here by locking a native thread with "runtime.LockOSThread()".
		// And intentionally ignore calling "runtime.UnlockOSThread()"
		// to terminate the native thread when the goroutine is done.
		var appendFunc func(...any) error
		if appendSync, ok := aw.appender.(interface{ AppendSync(...any) error }); ok {
			appendFunc = appendSync.AppendSync
		} else {
			appendFunc = aw.appender.Append
		}
		aw.log.Info("open")
	loop:
		for {
			select {
			case <-aw.ctx.Done():
				break loop
			case <-aw.appendStop:
				break loop
			case vals := <-aw.appendC:
				err := appendFunc(vals...)
				if err != nil {
					aw.log.Error("error:", err)
				}
			}
		}
		for len(aw.appendC) > 0 {
			vals := <-aw.appendC
			err := appendFunc(vals...)
			if err != nil {
				aw.log.Error("error:", err)
			}
		}
	}(aw)
}

func (aw *AppendWorker) Stop() {
	if aw.appendC != nil {
		close(aw.appendStop)
		aw.appendWg.Wait()
		close(aw.appendC)
		aw.appendC = nil
	}
	aw.ctxCancel()
	if success, fail, err := aw.appender.Close(); err != nil {
		aw.log.Error("close error:", err)
	} else {
		aw.log.Info("close, success:", success, "fail:", fail)
	}
	aw.conn.Close()
}

var _ Appender = (*AppendWorker)(nil)

func (aw *AppendWorker) Append(vals ...any) error {
	aw.appendC <- vals
	return nil
}

func (aw *AppendWorker) AppendLogTime(ts time.Time, vals ...any) error {
	if aw.appender.TableType() != TableTypeLog {
		return fmt.Errorf("%s is not a log table, use Append() instead", aw.appender.TableName())
	}
	aw.appendC <- append([]interface{}{ts}, vals...)
	return nil
}

func (aw *AppendWorker) Close() (success, fail int64, err error) {
	atomic.AddInt32(&aw.refCount, -1)
	return 0, 0, nil
}

func (aw *AppendWorker) Columns() (Columns, error) {
	return aw.appender.Columns()
}

func (aw *AppendWorker) TableType() TableType {
	return aw.appender.TableType()
}

func (aw *AppendWorker) TableName() string {
	return aw.appender.TableName()
}

func (aw *AppendWorker) WithInputColumns(columns ...string) Appender {
	ret := &AppenderWithWorker{
		AppendWorker: aw,
		inputColumns: make([]AppenderInputColumn, len(columns)),
	}

	ret.inputColumns = nil
	for _, col := range columns {
		ret.inputColumns = append(ret.inputColumns, AppenderInputColumn{Name: strings.ToUpper(col), Idx: -1})
	}
	if len(ret.inputColumns) > 0 {
		columns, _ := aw.appender.Columns()
		for idx, col := range columns {
			for inIdx, inputCol := range ret.inputColumns {
				if col.Name == inputCol.Name {
					ret.inputColumns[inIdx].Idx = idx
				}
			}
		}
	}
	return ret
}

func (aw *AppendWorker) WithInputFormats(formats ...string) Appender {
	// noop, handled in Append
	return aw
}

type AppenderWithWorker struct {
	*AppendWorker
	inputColumns []AppenderInputColumn
}

var _ Appender = (*AppenderWithWorker)(nil)

type AppenderInputColumn struct {
	Name string
	Idx  int
}

func (ap *AppenderWithWorker) Append(vals ...any) error {
	columns, _ := ap.Columns()
	if len(ap.inputColumns) == 0 {
		if len(columns) != len(vals) {
			return ErrDatabaseLengthOfColumns(ap.tableDesc.Name, len(columns), len(vals))
		}
		return ap.AppendWorker.Append(vals...)
	}
	newVals := make([]any, len(columns))
	for i, inputCol := range ap.inputColumns {
		newVals[inputCol.Idx] = vals[i]
	}
	return ap.AppendWorker.Append(newVals...)
}

func (aw *AppenderWithWorker) AppendLogTime(ts time.Time, vals ...any) error {
	if aw.appender.TableType() != TableTypeLog {
		return fmt.Errorf("%s is not a log table, use Append() instead", aw.appender.TableName())
	}
	aw.Append(append([]interface{}{ts}, vals...))
	return nil
}
