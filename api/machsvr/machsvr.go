package machsvr

import (
	"context"
	"database/sql"
	"expvar"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	mach "github.com/machbase/neo-engine/v8"
	"github.com/machbase/neo-engine/v8/native"
	"github.com/machbase/neo-server/v8/api"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/sony/sonyflake"
)

func LinkInfo() string {
	return mach.LinkInfo()
}

func LinkVersion() string {
	return native.Version
}

func LinkGitHash() string {
	return native.GitHash
}

type InitOption int

const (
	// machbase-engine takes all control of the signals
	OPT_SIGHANDLER_ON InitOption = 0x0
	// the caller takes all control, machbase-engine can not leave stack dump when the process crashed
	OPT_SIGHANDLER_OFF InitOption = 0x1
	// engine takes all control except SIGINT, so that the caller can take SIGINT control
	OPT_SIGHANDLER_SIGINT_OFF InitOption = 0x2
)

func Initialize(homeDir string, machPort int, opt InitOption) error {
	_env.Lock()
	defer _env.Unlock()
	homeDir = translateCodePage(homeDir)
	var handle unsafe.Pointer
	err := mach.EngInitialize(homeDir, machPort, int(opt), &handle)
	if err != nil {
		return err
	}
	_env.handle = handle
	return nil
}

func Finalize() {
	_env.Lock()
	defer _env.Unlock()
	if _env.handle != nil {
		mach.EngFinalize(_env.handle)
	}
}

func DestroyDatabase() error {
	_env.Lock()
	defer _env.Unlock()
	if _env.handle == nil {
		return api.ErrDatabaseNotInitialized
	}
	return mach.EngDestroyDatabase(_env.handle)
}

func CreateDatabase() error {
	_env.Lock()
	defer _env.Unlock()
	if _env.handle == nil {
		return api.ErrDatabaseNotInitialized
	}
	return mach.EngCreateDatabase(_env.handle)
}

func ExistsDatabase() bool {
	_env.Lock()
	defer _env.Unlock()
	if _env.handle == nil {
		return false
	}
	return mach.EngExistsDatabase(_env.handle)
}

func RestoreDatabase(path string) error {
	return mach.EngRestoreDatabase(_env.handle, path)
}

type DatabaseOption struct {
	// MaxOpenConn
	//
	//	< 0 : unlimited
	//	0 : default, maxOpenConns = CPU count * maxOpenConnsFactor
	//	> 0 : specified max open connections
	MaxOpenConn int

	// MaxOpenConnFactor
	//
	//	used to calculate the number of max open connections when maxOpenConns is 0
	//	default is 2.0
	MaxOpenConnFactor float64

	// MaxOpenQuery
	//
	//	< 0 : unlimited
	//	0 : default, maxQueryConns = CPU count * maxOpenConnsFactor
	//	> 0 : specified max query connections
	MaxOpenQuery int

	// MaxOpenQueryFactor
	//
	//	used to calculate the number of max query connections when maxQueryConns is 0
	//	default is 2.0
	MaxOpenQueryFactor float64

	MaxWorkerPoolSize int
}

type Env struct {
	sync.Mutex
	handle          unsafe.Pointer
	database        *Database
	onceNewDatabase sync.Once
}

var _env = Env{}

type Database struct {
	handle    unsafe.Pointer
	idGen     *sonyflake.Sonyflake
	conns     cmap.ConcurrentMap[string, *ConnWatcher]
	onceStart sync.Once
	onceStop  sync.Once

	maxConnMutex  sync.RWMutex
	maxConnChan   chan struct{}
	maxQueryMutex sync.RWMutex
	maxQueryChan  chan struct{}

	enableWorkerPool  bool
	pool              chan *Worker
	poolSize          int
	poolSizeHardLimit int
}

var _ api.Database = (*Database)(nil)

func NewDatabase(opt DatabaseOption) (*Database, error) {
	_env.Lock()
	defer _env.Unlock()

	_env.onceNewDatabase.Do(func() {
		_env.database = &Database{
			handle:            _env.handle,
			conns:             cmap.New[*ConnWatcher](),
			idGen:             sonyflake.NewSonyflake(sonyflake.Settings{}),
			poolSize:          runtime.NumCPU(),
			poolSizeHardLimit: runtime.NumCPU() * 100,
		}
	})

	if opt.MaxOpenConnFactor <= 0 {
		opt.MaxOpenConnFactor = 2.0
	}

	if opt.MaxOpenConn < 0 {
		opt.MaxOpenConn = -1
	} else if opt.MaxOpenConn == 0 {
		opt.MaxOpenConn = int(float64(runtime.NumCPU()) * opt.MaxOpenConnFactor)
	}

	_env.database.SetMaxOpenConn(opt.MaxOpenConn)

	if opt.MaxOpenQueryFactor <= 0 {
		opt.MaxOpenQueryFactor = 2.0
	}
	if opt.MaxOpenQuery < 0 {
		opt.MaxOpenQuery = -1
	} else if opt.MaxOpenQuery == 0 {
		opt.MaxOpenQuery = int(float64(runtime.NumCPU()) * opt.MaxOpenQueryFactor)
	}

	_env.database.SetMaxOpenQuery(opt.MaxOpenQuery)

	if opt.MaxWorkerPoolSize > 0 {
		_env.database.SetWorkerPoolSize(opt.MaxWorkerPoolSize)
	}
	return _env.database, nil
}

func (db *Database) Startup() (err error) {
	_env.Lock()
	defer _env.Unlock()

	db.onceStart.Do(func() {
		err = mach.EngStartup(db.handle)
		if err == nil {
			// start worker pool
			db.startWorkerPool()
		}
	})
	return
}

func (db *Database) Shutdown() (err error) {
	_env.Lock()
	defer _env.Unlock()

	db.onceStop.Do(func() {
		db.stopWorkerPool()
		err = mach.EngShutdown(db.handle)
		_env.database = nil
		_env.handle = nil
	})
	return
}

func (db *Database) Error() error {
	return mach.EngError(db.handle)
}

// MaxOpenConn returns the maximum number of open connections
// and the current remains capacity.
func (db *Database) MaxOpenConn() (int, int) {
	db.maxConnMutex.RLock()
	defer db.maxConnMutex.RUnlock()
	if db.maxConnChan == nil {
		// unlimited
		return -1, -1
	}
	limit := cap(db.maxConnChan)
	remains := len(db.maxConnChan)
	return limit, remains
}

func (db *Database) SetMaxOpenConn(desiredMaxOpenConn int) {
	if desiredMaxOpenConn < 0 {
		desiredMaxOpenConn = -1
	}
	if desiredMaxOpenConn == 0 {
		desiredMaxOpenConn = int(float64(runtime.NumCPU()) * 2.0)
	}

	currentCap := cap(db.maxConnChan)
	if currentCap == desiredMaxOpenConn {
		return
	}

	var newChan chan struct{}
	db.maxConnMutex.Lock()
	defer func() {
		db.maxConnChan = newChan
		db.maxConnMutex.Unlock()
	}()

	if desiredMaxOpenConn > 0 {
		newChan = make(chan struct{}, desiredMaxOpenConn)
		for i := 0; i < desiredMaxOpenConn; i++ {
			newChan <- struct{}{}
		}
	}
}

// MaxOpenQuery returns the maximum number of open query connections
// and the current remains capacity.
func (db *Database) MaxOpenQuery() (int, int) {
	db.maxQueryMutex.RLock()
	defer db.maxQueryMutex.RUnlock()
	if db.maxQueryChan == nil {
		// unlimited
		return -1, -1
	}
	limit := cap(db.maxQueryChan)
	remains := len(db.maxQueryChan)
	return limit, remains
}

func (db *Database) SetMaxOpenQuery(desiredMaxOpenQuery int) {
	if desiredMaxOpenQuery < 0 {
		desiredMaxOpenQuery = -1
	}
	if desiredMaxOpenQuery == 0 {
		desiredMaxOpenQuery = int(float64(runtime.NumCPU()) * 2.0)
	}

	currentCap := cap(db.maxQueryChan)
	if currentCap == desiredMaxOpenQuery {
		return
	}

	var newChan chan struct{}
	db.maxQueryMutex.Lock()
	defer func() {
		db.maxQueryChan = newChan
		db.maxQueryMutex.Unlock()
	}()

	if desiredMaxOpenQuery > 0 {
		newChan = make(chan struct{}, desiredMaxOpenQuery)
		for i := 0; i < desiredMaxOpenQuery; i++ {
			newChan <- struct{}{}
		}
	}
}

func (db *Database) UserAuth(ctx context.Context, username string, password string) (bool, string, error) {
	ok, err := mach.EngUserAuth(db.handle, username, password)
	if err != nil {
		return false, "", err
	}
	if ok {
		return true, "", nil
	} else {
		return false, "invalid username or password", nil
	}
}

func (db *Database) Ping(ctx context.Context) (time.Duration, error) {
	tick := time.Now()
	if ExistsDatabase() {
		return time.Since(tick), nil
	} else {
		return 0, api.ErrDatabaseNotInitialized
	}
}

func (db *Database) RegisterWatcher(key string, conn *Conn) {
	db.SetWatcher(key, &ConnWatcher{
		createdTime: time.Now(),
		conn:        conn,
	})
}

func (db *Database) SetWatcher(key string, cw *ConnWatcher) {
	db.conns.Set(key, cw)
}

func (db *Database) GetWatcher(key string) (*ConnState, bool) {
	w, ok := db.conns.Get(key)
	if ok {
		return w.ConnState(), true
	} else {
		return nil, false
	}
}

func (db *Database) RemoveWatcher(key string) {
	db.conns.Remove(key)
}

func (db *Database) ListWatcher(cb func(*ConnState) bool) {
	if cb == nil {
		return
	}
	var cont = true
	db.conns.IterCb(func(_ string, cw *ConnWatcher) {
		if !cont {
			return
		}
		v := cw.ConnState()
		cont = cb(v)
	})
}

func (db *Database) KillConnection(id string, force bool) error {
	if cw, ok := db.conns.Get(id); ok {
		if cw.conn == nil {
			return api.ErrDatabaseConnectionInvalid(id)
		}
		if force {
			return cw.conn.Close()
		} else {
			return cw.conn.Cancel()
		}
	} else {
		return api.ErrDatabaseConnectionNotFound(id)
	}
}

type ConnWatcher struct {
	createdTime time.Time
	conn        *Conn
}

type ConnState struct {
	Id          string
	CreatedTime time.Time
	LatestTime  time.Time
	LatestSql   string
}

func (cw *ConnWatcher) ConnState() *ConnState {
	ret := &ConnState{
		CreatedTime: cw.createdTime,
	}
	if cw.conn != nil {
		ret.Id = cw.conn.id
		ret.LatestTime = cw.conn.latestTime
		ret.LatestSql = cw.conn.latestSql
	}
	return ret
}

type Conn struct {
	ctx         context.Context
	username    string
	password    string
	isTrustUser bool
	handle      unsafe.Pointer
	closeOnce   sync.Once
	closed      bool
	db          *Database

	id            string
	connectTime   time.Time
	latestTime    time.Time
	latestSql     string
	closeCallback func()

	returnChan chan struct{}
}

var _ api.Conn = (*Conn)(nil)

func (conn *Conn) SetLatestSql(sql string) {
	conn.latestTime = time.Now()
	conn.latestSql = sql
}

func (db *Database) Connect(ctx context.Context, opts ...api.ConnectOption) (api.Conn, error) {
	if db.enableWorkerPool {
		return db.ConnectAsync(ctx, opts...)
	}
	return db.ConnectSync(ctx, opts...)
}

func (db *Database) ConnectAsync(ctx context.Context, opts ...api.ConnectOption) (api.Conn, error) {
	req := &ConnectWork{
		ctx:  ctx,
		opts: opts,
	}
	req = db.workPool(req).(*ConnectWork)
	return req.conn, req.err
}

func (db *Database) ConnectSync(ctx context.Context, opts ...api.ConnectOption) (api.Conn, error) {
	var connTimeout time.Duration
	ret := &Conn{
		ctx:        ctx,
		db:         db,
		returnChan: db.maxConnChan,
	}
	for _, o := range opts {
		switch v := o.(type) {
		case *api.ConnectOptionPassword:
			ret.username = v.User
			ret.password = v.Password
		case *api.ConnectOptionTrustUser:
			ret.username = v.User
			ret.isTrustUser = true
		case *api.ConnectOptionTimeout:
			connTimeout = v.Timeout
		default:
			return nil, fmt.Errorf("unknown option type-%T", o)
		}
	}

	waitTime := time.Now()
	// control max open connections
	if ret.returnChan != nil {
		if connTimeout > 0 {
			select {
			case <-ret.returnChan:
			case <-ctx.Done():
				return nil, api.NewError("connect canceled")
			case <-time.After(connTimeout):
				return nil, api.NewError("connect timeout")
			}
		} else {
			select {
			case <-ret.returnChan:
			case <-ctx.Done():
				return nil, api.NewError("connect canceled")
			}
		}
	}

	var handle unsafe.Pointer
	if ret.isTrustUser {
		if err := mach.EngConnectTrust(db.handle, ret.username, &handle); err != nil {
			return nil, err
		}
	} else {
		if err := mach.EngConnect(db.handle, ret.username, ret.password, &handle); err != nil {
			return nil, err
		}
	}
	ret.handle = handle
	ret.connectTime = time.Now()
	api.AllocConn(time.Since(waitTime))

	if id, err := mach.EngSessionID(ret.handle); err == nil {
		ret.id = fmt.Sprintf("%d", id)
	} else {
		id, err := db.idGen.NextID()
		if err != nil {
			return nil, api.ErrDatabaseConnectID(err.Error())
		}
		ret.id = fmt.Sprintf("%X", id)
	}

	ret.closeCallback = func() {
		ret.SetLatestSql("CLOSE") // 3. set latest sql time
		db.RemoveWatcher(ret.id)
	}
	db.RegisterWatcher(ret.id, ret) // 1. register watcher
	ret.SetLatestSql("CONNECT")     // 2. set latest sql time
	return ret, nil
}

// Close closes connection
func (conn *Conn) Close() (err error) {
	if conn.db.enableWorkerPool {
		return conn.CloseAsync()
	}
	return conn.CloseSync()
}

func (conn *Conn) CloseAsync() error {
	req := &ConnCloseWork{conn: conn}
	req = conn.db.workPool(req).(*ConnCloseWork)
	return req.err
}

func (conn *Conn) CloseSync() (err error) {
	conn.closeOnce.Do(func() {
		defer func() {
			if conn.returnChan != nil {
				conn.returnChan <- struct{}{}
			}
			if r := recover(); r != nil {
				fmt.Println("Recovered in Conn.Close", r)
			}
		}()
		conn.closed = true
		api.FreeConn(time.Since(conn.connectTime))
		err = mach.EngDisconnect(conn.handle)
		if conn.closeCallback != nil {
			conn.closeCallback()
		}
	})
	return
}

func (conn *Conn) Cancel() error {
	if err := mach.EngCancel(conn.handle); err != nil {
		return err
	}
	return conn.Close()
}

func (conn *Conn) Connected() bool {
	if conn.closed {
		return false
	}
	if len(conn.ctx.Done()) != 0 {
		<-conn.ctx.Done()
		conn.Close()
		return false
	}
	return true
}

// ExecContext executes SQL statements that does not return result
// like 'ALTER', 'CREATE TABLE', 'DROP TABLE', ...
func (conn *Conn) Exec(ctx context.Context, sqlText string, params ...any) api.Result {
	if conn.db.enableWorkerPool {
		return conn.ExecAsync(ctx, sqlText, params...)
	}
	return conn.ExecSync(ctx, sqlText, params...)
}

func (conn *Conn) ExecAsync(ctx context.Context, sqlText string, params ...any) api.Result {
	req := &ExecWork{
		ctx:     ctx,
		conn:    conn,
		sqlText: sqlText,
		params:  params,
	}
	req = conn.db.workPool(req).(*ExecWork)
	return req.result
}

func (conn *Conn) ExecSync(ctx context.Context, sqlText string, params ...any) api.Result {
	conn.SetLatestSql(sqlText)
	var result = &Result{}
	var stmt unsafe.Pointer
	if result.err = mach.EngAllocStmt(conn.handle, &stmt); result.err != nil {
		return result
	}
	api.AllocStmt()
	defer func() {
		mach.EngFreeStmt(stmt)
		api.FreeStmt()
	}()
	if len(params) == 0 {
		if result.err = mach.EngDirectExecute(stmt, sqlText); result.err != nil {
			return result
		}
	} else {
		if result.err = mach.EngPrepare(stmt, sqlText); result.err != nil {
			return result
		}
		for i, p := range params {
			if err := bind(stmt, i, p); err != nil {
				result.err = err
				return result
			}
		}
		if result.err = mach.EngExecute(stmt); result.err != nil {
			return result
		}
	}
	affectedRows, err := mach.EngEffectRows(stmt)
	if err != nil {
		result.err = err
		return result
	}
	stmtType, err := mach.EngStmtType(stmt)
	result.affectedRows = affectedRows
	result.stmtType = StmtType(stmtType)
	result.err = err
	return result
}

// Query executes SQL statements that are expected multiple rows as result.
// Commonly used to execute 'SELECT * FROM <TABLE>'
//
// Rows returned by Query() must be closed to prevent server-side-resource leaks.
//
//	ctx, cancelFunc := context.WithTimeout(5*time.Second)
//	defer cancelFunc()
//
//	rows, err := conn.Query(ctx, "select * from my_table where name = ?", my_name)
//	if err != nil {
//		panic(err)
//	}
//	defer rows.Close()
func (conn *Conn) Query(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
	if conn.db.enableWorkerPool {
		return conn.QueryAsync(ctx, sqlText, params...)
	}
	return conn.QuerySync(ctx, sqlText, params...)
}

func (conn *Conn) QueryAsync(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
	req := &QueryWork{
		ctx:     ctx,
		conn:    conn,
		sqlText: sqlText,
		params:  params,
	}
	req = conn.db.workPool(req).(*QueryWork)
	return req.rows, req.err
}

func (conn *Conn) QuerySync(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
	conn.SetLatestSql(sqlText)
	rows := &Rows{
		sqlText:             sqlText,
		candidateReturnChan: conn.db.maxQueryChan,
	}
	if err := mach.EngAllocStmt(conn.handle, &rows.stmt); err != nil {
		return nil, err
	}
	if err := mach.EngPrepare(rows.stmt, sqlText); err != nil {
		mach.EngFreeStmt(rows.stmt)
		return nil, err
	}
	for i, p := range params {
		if err := bind(rows.stmt, i, p); err != nil {
			mach.EngFreeStmt(rows.stmt)
			return nil, err
		}
	}
	if err := mach.EngExecute(rows.stmt); err != nil {
		mach.EngFreeStmt(rows.stmt)
		return nil, err
	}
	if stmtType, err := mach.EngStmtType(rows.stmt); err != nil {
		mach.EngFreeStmt(rows.stmt)
		return nil, err
	} else {
		rows.stmtType = StmtType(stmtType)
	}

	if cols, err := stmtColumns(rows.stmt); err != nil {
		mach.EngFreeStmt(rows.stmt)
		return nil, err
	} else {
		rows.columns = cols
	}
	api.AllocStmt()
	return rows, nil
}

func stmtColumns(stmt unsafe.Pointer) (api.Columns, error) {
	columnCount, err := mach.EngColumnCount(stmt)
	if err != nil {
		return nil, err
	}
	ret := make(api.Columns, columnCount)
	for i := 0; i < columnCount; i++ {
		var columnName string
		var columnRawType, columnSize, columnLength int
		err = mach.EngColumnInfo(stmt, i, &columnName, &columnRawType, &columnSize, &columnLength)
		if err != nil {
			return nil, err
		}
		dataType, err := columnRawTypeToDataType(columnRawType)
		if err != nil {
			return nil, mach.ErrDatabaseWrap("Invalid column type", err)
		}
		ret[i] = &api.Column{
			Name:     strings.ToUpper(columnName),
			DataType: dataType,
			Length:   columnSize,
		}
	}
	return ret, nil
}

const (
	ColumnRawTypeInt16    int = iota + 0
	ColumnRawTypeInt32        = 1
	ColumnRawTypeInt64        = 2
	ColumnRawTypeDatetime     = 3
	ColumnRawTypeFloat32      = 4
	ColumnRawTypeFloat64      = 5
	ColumnRawTypeIPv4         = 6
	ColumnRawTypeIPv6         = 7
	ColumnRawTypeString       = 8
	ColumnRawTypeBinary       = 9
)

func columnRawTypeToDataType(rawType int) (api.DataType, error) {
	switch rawType {
	case ColumnRawTypeInt16:
		return api.DataTypeInt16, nil
	case ColumnRawTypeInt32:
		return api.DataTypeInt32, nil
	case ColumnRawTypeInt64:
		return api.DataTypeInt64, nil
	case ColumnRawTypeDatetime:
		return api.DataTypeDatetime, nil
	case ColumnRawTypeFloat32:
		return api.DataTypeFloat32, nil
	case ColumnRawTypeFloat64:
		return api.DataTypeFloat64, nil
	case ColumnRawTypeIPv4:
		return api.DataTypeIPv4, nil
	case ColumnRawTypeIPv6:
		return api.DataTypeIPv6, nil
	case ColumnRawTypeString:
		return api.DataTypeString, nil
	case ColumnRawTypeBinary:
		return api.DataTypeBinary, nil
	default:
		return "", api.ErrDatabaseUnsupportedType("ColumnType", rawType)
	}
}

func columnDataTypeToRawType(typ api.DataType) (int, error) {
	switch typ {
	case api.DataTypeInt16:
		return ColumnRawTypeInt16, nil
	case api.DataTypeInt32:
		return ColumnRawTypeInt32, nil
	case api.DataTypeInt64:
		return ColumnRawTypeInt64, nil
	case api.DataTypeDatetime:
		return ColumnRawTypeDatetime, nil
	case api.DataTypeFloat32:
		return ColumnRawTypeFloat32, nil
	case api.DataTypeFloat64:
		return ColumnRawTypeFloat64, nil
	case api.DataTypeIPv4:
		return ColumnRawTypeIPv4, nil
	case api.DataTypeIPv6:
		return ColumnRawTypeIPv6, nil
	case api.DataTypeString:
		return ColumnRawTypeString, nil
	case api.DataTypeBinary:
		return ColumnRawTypeBinary, nil
	default:
		return 0, api.ErrDatabaseUnsupportedTypeName("DataType", string(typ))
	}
}

// QueryRow executes a SQL statement that expects a single row result.
//
//	ctx, cancelFunc := context.WithTimeout(5*time.Second)
//	defer cancelFunc()
//
//	var cnt int
//	row := conn.QueryRow(ctx, "select count(*) from my_table where name = ?", "my_name")
//	row.Scan(&cnt)
func (conn *Conn) QueryRow(ctx context.Context, sqlText string, params ...any) api.Row {
	if conn.db.enableWorkerPool {
		return conn.QueryRowAsync(ctx, sqlText, params...)
	}
	return conn.QueryRowSync(ctx, sqlText, params...)
}

func (conn *Conn) QueryRowAsync(ctx context.Context, sqlText string, params ...any) api.Row {
	req := &QueryRowWork{
		ctx:     ctx,
		conn:    conn,
		sqlText: sqlText,
		params:  params,
	}
	req = conn.db.workPool(req).(*QueryRowWork)
	return req.row
}

func (conn *Conn) QueryRowSync(ctx context.Context, sqlText string, params ...any) api.Row {
	conn.SetLatestSql(sqlText)
	var row = &Row{}
	var stmt unsafe.Pointer
	if row.err = mach.EngAllocStmt(conn.handle, &stmt); row.err != nil {
		return row
	}
	api.AllocStmt()
	defer func() {
		api.FreeStmt()
		err := mach.EngFreeStmt(stmt)
		if err != nil && row.err == nil {
			row.err = err
		}
	}()

	if row.err = mach.EngPrepare(stmt, sqlText); row.err != nil {
		return row
	}
	for i, p := range params {
		if row.err = bind(stmt, i, p); row.err != nil {
			return row
		}
	}
	if row.err = mach.EngExecute(stmt); row.err != nil {
		return row
	}

	if typ, err := mach.EngStmtType(stmt); err != nil {
		row.err = err
		return row
	} else {
		row.stmtType = StmtType(typ)
	}

	// Do not proceed if the statement is not a SELECT
	if !row.stmtType.IsSelect() {
		affectedRows, err := mach.EngEffectRows(stmt)
		if err != nil {
			row.err = err
			return row
		}
		row.affectedRows = affectedRows
		row.ok = true
		return row
	}

	var fetched bool
	if fetched, row.err = mach.EngFetch(stmt); row.err != nil {
		// fetch error
		return row
	}

	// nothing fetched
	if !fetched {
		row.err = sql.ErrNoRows
		return row
	}

	if cols, err := stmtColumns(stmt); err != nil {
		row.err = err
		return row
	} else {
		row.columns = cols
		row.values, row.err = cols.MakeBuffer()
		if row.err != nil {
			return row
		}
		for i, col := range cols {
			rawType, err := columnDataTypeToRawType(col.DataType)
			if err != nil {
				row.err = err
				return row
			}
			isNull := false
			row.err = readColumnData(stmt, rawType, i, row.values[i], &isNull)
			if row.err != nil {
				return row
			}
			if isNull {
				row.values[i] = nil
			}
		}
	}
	if row.err == nil {
		row.affectedRows = 1
		row.ok = true
	}
	return row
}

func (conn *Conn) Explain(ctx context.Context, sqlText string, full bool) (string, error) {
	if conn.db.enableWorkerPool {
		return conn.ExplainAsync(ctx, sqlText, full)
	}
	return conn.ExplainSync(ctx, sqlText, full)
}

func (conn *Conn) ExplainAsync(ctx context.Context, sqlText string, full bool) (string, error) {
	req := &ExplainWork{
		ctx:     ctx,
		conn:    conn,
		sqlText: sqlText,
		full:    full,
	}
	req = conn.db.workPool(req).(*ExplainWork)
	return req.explain, req.err
}

func (conn *Conn) ExplainSync(ctx context.Context, sqlText string, full bool) (string, error) {
	conn.SetLatestSql("EXPLAIN " + sqlText)
	var stmt unsafe.Pointer
	if err := mach.EngAllocStmt(conn.handle, &stmt); err != nil {
		return "", err
	}
	defer mach.EngFreeStmt(stmt)

	if full {
		if err := mach.EngDirectExecute(stmt, sqlText); err != nil {
			return "", err
		}
	} else {
		if err := mach.EngPrepare(stmt, sqlText); err != nil {
			return "", err
		}
	}
	return mach.EngExplain(stmt, full)
}

func bind(stmt unsafe.Pointer, idx int, c any) error {
	if c == nil {
		if err := mach.EngBindNull(stmt, idx); err != nil {
			return api.ErrDatabaseBindNull(idx, err)
		}
		return nil
	}
	switch cv := c.(type) {
	case int:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *int:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case uint:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *uint:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case int16:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *int16:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case uint16:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *uint16:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case int32:
		if err := mach.EngBindInt32(stmt, idx, cv); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *int32:
		if err := mach.EngBindInt32(stmt, idx, *cv); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case uint32:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *uint32:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case int64:
		if err := mach.EngBindInt64(stmt, idx, cv); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *int64:
		if err := mach.EngBindInt64(stmt, idx, *cv); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case uint64:
		if err := mach.EngBindInt64(stmt, idx, int64(cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *uint64:
		if err := mach.EngBindInt64(stmt, idx, int64(*cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case float32:
		if err := mach.EngBindFloat64(stmt, idx, float64(cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *float32:
		if err := mach.EngBindFloat64(stmt, idx, float64(*cv)); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case float64:
		if err := mach.EngBindFloat64(stmt, idx, cv); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *float64:
		if err := mach.EngBindFloat64(stmt, idx, *cv); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case string:
		if err := mach.EngBindString(stmt, idx, cv); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *string:
		if err := mach.EngBindString(stmt, idx, *cv); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case []byte:
		if err := mach.EngBindBinary(stmt, idx, cv); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case net.IP:
		if err := mach.EngBindString(stmt, idx, cv.String()); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case time.Time:
		if err := mach.EngBindInt64(stmt, idx, cv.UnixNano()); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	case *time.Time:
		if err := mach.EngBindInt64(stmt, idx, cv.UnixNano()); err != nil {
			return api.ErrDatabaseBind(idx, c, err)
		}
	default:
		return api.ErrDatabaseBindType(idx, c)
	}
	return nil
}

func init() {
	expvar.Publish("machbase:session:raw_conns", expvar.Func(func() any { return rawConns() }))
	api.RawConns = rawConns
}

func rawConns() int {
	if _env.handle != nil {
		return mach.EngConnectionCount(_env.handle)
	}
	return 0
}
