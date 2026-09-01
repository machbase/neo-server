package spi

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-server/v8/mods/logging"
)

type AppendWorker struct {
	appender  *client.Appender
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
var appendersControl chan *AppendWorkerControl // send normalized worker key to stop the appender

type AppendWorkerControl struct {
	Key string
	ack chan struct{}
}

var AppendWorkerMaxIdleTimeout = 5 * time.Second
var AppendWorkerIdleCheckInterval = 3 * time.Second

func StartAppendWorkers() {
	appenders = make(map[string]*AppendWorker)
	appendersFlusher = make(chan struct{})
	appendersFlusherWg.Add(1)
	appendersControl = make(chan *AppendWorkerControl)
	go func() {
		defer appendersFlusherWg.Done()
		for {
			select {
			case <-time.After(AppendWorkerIdleCheckInterval):
				appendersLock.Lock()
				var deleting []string
				for tableName, value := range appenders {
					if !value.lastTime.IsZero() && time.Since(value.lastTime) > AppendWorkerMaxIdleTimeout && atomic.LoadInt32(&value.refCount) == 0 {
						value.Stop()
						deleting = append(deleting, tableName)
					}
				}
				for _, tableName := range deleting {
					delete(appenders, tableName)
				}
				appendersLock.Unlock()
			case control := <-appendersControl:
				appendersLock.Lock()
				if value, exists := appenders[control.Key]; exists {
					value.Stop()
					delete(appenders, control.Key)
				}
				appendersLock.Unlock()
				close(control.ack)
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

// StopAppendWorker stops the append worker for the given database, user and table
// (same resolution rules as GetAppendWorker) and returns a channel to wait for the
// stop to complete.
func StopAppendWorker(db, user, table string) chan struct{} {
	ack := make(chan struct{})
	appendersControl <- &AppendWorkerControl{
		Key: appendWorkerKey(db, user, table),
		ack: ack,
	}
	return ack
}

// FlushAppendWorkers flushes append workers for the given database, user and table
// names (same resolution rules as GetAppendWorker). If tables is empty, db and user
// are ignored and every append worker is flushed.
func FlushAppendWorkers(db, user string, tables ...string) {
	appendersLock.Lock()
	defer appendersLock.Unlock()
	if len(tables) == 0 {
		for _, value := range appenders {
			value.Stop()
		}
		appenders = make(map[string]*AppendWorker)
	} else {
		var deleting []string
		for _, table := range tables {
			key := appendWorkerKey(db, user, table)
			if value, exists := appenders[key]; exists {
				value.Stop()
				deleting = append(deleting, key)
			}
		}
		for _, key := range deleting {
			delete(appenders, key)
		}
	}
}

// splitAppendWorkerName parses a possibly qualified "db.user.table" / "user.table" /
// "table" string into its parts. When name carries no "." qualifier, db and user are
// both returned empty and table is returned unchanged.
func splitAppendWorkerName(name string) (db, user, table string) {
	parts := strings.SplitN(name, ".", 3)
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2]
	case 2:
		return "", parts[0], parts[1]
	default:
		return "", "", name
	}
}

// normalizeAppendWorkerParts fills in the default database (MACHBASEDB) and user
// (SYS) for empty inputs and upper-cases all three parts.
func normalizeAppendWorkerParts(db, user, table string) (normDB, normUser, normTable string) {
	normDB = strings.ToUpper(strings.TrimSpace(db))
	if normDB == "" {
		normDB = "MACHBASEDB"
	}
	normUser = strings.ToUpper(strings.TrimSpace(user))
	if normUser == "" {
		normUser = "SYS"
	}
	normTable = strings.ToUpper(strings.TrimSpace(table))
	return
}

// resolveAppendWorkerParts applies explicit db/user (when non-empty) over any
// "user.table"/"db.user.table" qualifier embedded in table, then normalizes empty
// parts to MACHBASEDB/SYS. GetAppendWorker, StopAppendWorker and FlushAppendWorkers
// all resolve through this so the same inputs always produce the same cache key.
func resolveAppendWorkerParts(db, user, table string) (normDB, normUser, normTable string) {
	if parsedDB, parsedUser, parsedTable := splitAppendWorkerName(table); parsedTable != table {
		if db == "" {
			db = parsedDB
		}
		if user == "" {
			user = parsedUser
		}
		table = parsedTable
	}
	return normalizeAppendWorkerParts(db, user, table)
}

// appendWorkerKey computes the normalized "db.user.table" cache key for the given
// database, user and table (used by StopAppendWorker/FlushAppendWorkers).
func appendWorkerKey(db, user, table string) string {
	normDB, normUser, normTable := resolveAppendWorkerParts(db, user, table)
	return strings.ToLower(normDB + "." + normUser + "." + normTable)
}

// GetAppendWorker returns a cached AppendWorker for the given database, user and
// table, creating one if it doesn't exist. The worker map key is normalized as
// "db.user.table" (empty db/user default to MACHBASEDB/SYS). For callers that
// haven't split a qualified name themselves, table may still carry a legacy
// "user.table" or "db.user.table" qualifier, but explicit db/user arguments always
// take precedence over it.
func GetAppendWorker(ctx context.Context, db, user, table string) (*AppendWorker, error) {
	appendersLock.Lock()
	defer appendersLock.Unlock()

	normDB, normUser, normTable := resolveAppendWorkerParts(db, user, table)
	key := strings.ToLower(normDB + "." + normUser + "." + normTable)

	if aw, exists := appenders[key]; exists {
		aw.lastTime = time.Now()
		atomic.AddInt32(&aw.refCount, 1)
		return aw, nil
	}

	qualifiedTableName := fmt.Sprintf("%s.%s.%s", normDB, normUser, normTable)

	trustConn, err := Connect(ctx, "sys")
	if err != nil {
		return nil, err
	}
	defer trustConn.Close()

	tableDesc, err := DescribeTable(ctx, trustConn, qualifiedTableName, false)
	if err != nil {
		return nil, err
	}

	workerCtx, ctxCancel := context.WithCancel(context.Background())

	appender := &client.Appender{}
	// The auth key belongs to "sys"; connecting directly as normUser would fail
	// authentication, so proxy via "sys as <normUser>" unless the target is sys itself.
	dsnUser := "sys"
	if !strings.EqualFold(normUser, "sys") {
		dsnUser = fmt.Sprintf("sys as %s", normUser)
	}
	// This connection lives exactly as long as the AppendWorker, so select the target
	// database directly in the DSN (Connector.Connect's own initial "USE <db>") instead
	// of switching it at runtime inside Appender.Connect, which would require holding
	// the connection's session lock reentrantly.
	dsn := DefaultDSN(map[string]string{"user": dsnUser, "db": normDB})
	if err := appender.Connect(workerCtx, dsn, normUser+"."+normTable); err != nil {
		ctxCancel()
		return nil, err
	}

	ret := &AppendWorker{
		ctx:       workerCtx,
		ctxCancel: ctxCancel,
		appender:  appender,
		tableDesc: tableDesc,
		lastTime:  time.Now(),
		refCount:  1,
		log:       logging.GetLog(fmt.Sprintf("appender-%s", key)),
	}
	appenders[key] = ret
	ret.Start()
	return ret, nil
}

func (aw *AppendWorker) Start() {
	aw.appendC = make(chan []interface{}, 1000)
	aw.appendStop = make(chan struct{})
	aw.appendWg.Add(1)
	go func(aw *AppendWorker) {
		defer aw.appendWg.Done()
		aw.log.Info("open")
	loop:
		for {
			select {
			case <-aw.ctx.Done():
				break loop
			case <-aw.appendStop:
				break loop
			case vals := <-aw.appendC:
				err := aw.appender.Append(vals...)
				if err != nil {
					aw.log.Error("error:", err)
				}
			}
		}
		for len(aw.appendC) > 0 {
			vals := <-aw.appendC
			err := aw.appender.Append(vals...)
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
		if fail > 0 {
			aw.log.Info("close, success:", success, "fail:", fail)
		} else {
			aw.log.Info("close, success:", success)
		}
	}
}

var _ Appender = (*AppendWorker)(nil)

func (aw *AppendWorker) Append(vals ...any) error {
	aw.appendC <- vals
	return nil
}

func (aw *AppendWorker) AppendLogTime(ts time.Time, vals ...any) error {
	if tableType := aw.appender.TableType(); tableType != client.TableTypeLog {
		return fmt.Errorf("%s is not a log table, use Append() instead", aw.appender.TableName())
	}
	aw.appendC <- append([]interface{}{ts}, vals...)
	return nil
}

func (aw *AppendWorker) Close() (success, fail int64, err error) {
	atomic.AddInt32(&aw.refCount, -1)
	return 0, 0, nil
}

func (aw *AppendWorker) Columns() client.Columns {
	return aw.appender.Columns()
}

func (aw *AppendWorker) TableType() client.TableType {
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
		columns := aw.appender.Columns()
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

func (aw *AppendWorker) WithBatchMaxRows(rows int) Appender {
	// noop, handled in Append
	return aw
}

func (aw *AppendWorker) WithBatchMaxBytes(bytes int) Appender {
	// noop, handled in Append
	return aw
}

func (aw *AppendWorker) WithBatchMaxDelay(duration time.Duration) Appender {
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
	columns := ap.Columns()
	if len(ap.inputColumns) == 0 {
		if len(columns) != len(vals) {
			return fmt.Errorf("value count %d, table '%s' requires %d columns to append", len(vals), ap.tableDesc.Name, len(columns))
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
	if tableType := aw.appender.TableType(); tableType != client.TableTypeLog {
		return fmt.Errorf("%s is not a log table, use Append() instead", aw.appender.TableName())
	}
	aw.Append(append([]interface{}{ts}, vals...))
	return nil
}
