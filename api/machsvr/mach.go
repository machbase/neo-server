package machsvr

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-engine/native"
	"github.com/machbase/neo-server/api/types"
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
		return types.ErrDatabaseNotInitialized
	}
	return mach.EngDestroyDatabase(_env.handle)
}

func CreateDatabase() error {
	_env.Lock()
	defer _env.Unlock()
	if _env.handle == nil {
		return types.ErrDatabaseNotInitialized
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

type Env struct {
	sync.Mutex
	handle   unsafe.Pointer
	database *Database
}

var _env = Env{}

type Database struct {
	handle unsafe.Pointer
	idGen  *sonyflake.Sonyflake
	conns  cmap.ConcurrentMap[string, *ConnWatcher]
}

func NewDatabase() (*Database, error) {
	_env.Lock()
	defer _env.Unlock()
	if _env.database == nil {
		_env.database = &Database{
			handle: _env.handle,
			conns:  cmap.New[*ConnWatcher](),
			idGen:  sonyflake.NewSonyflake(sonyflake.Settings{}),
		}
	}
	return _env.database, nil
}

func (db *Database) Startup() error {
	// machbase change the current dir to $HOME during startup process.
	// Call chdir() for keeping the working dir of the application.
	cwd, _ := os.Getwd()
	defer func() {
		os.Chdir(cwd)
	}()

	err := mach.EngStartup(db.handle)
	return err
}

func (db *Database) Shutdown() error {
	return mach.EngShutdown(db.handle)
}

func (db *Database) Error() error {
	return mach.EngError(db.handle)
}

func (db *Database) UserAuth(username, password string) (bool, error) {
	return mach.EngUserAuth(db.handle, username, password)
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
			return types.ErrDatabaseConnectionInvalid(id)
		}
		if force {
			return cw.conn.Close()
		} else {
			return cw.conn.Cancel()
		}
	} else {
		return types.ErrDatabaseConnectionNotFound(id)
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
	latestTime    time.Time
	latestSql     string
	closeCallback func()
}

func (conn *Conn) SetLatestSql(sql string) {
	conn.latestTime = time.Now()
	conn.latestSql = sql
}

type ConnectOption func(*Conn)

func WithPassword(username string, password string) ConnectOption {
	return func(c *Conn) {
		c.username = username
		c.password = password
	}
}

func WithTrustUser(username string) ConnectOption {
	return func(c *Conn) {
		c.username = username
		c.isTrustUser = true
	}
}

func (db *Database) Connect(ctx context.Context, opts ...ConnectOption) (*Conn, error) {
	ret := &Conn{
		ctx: ctx,
		db:  db,
	}
	for _, o := range opts {
		o(ret)
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

	if id, err := mach.EngSessionID(ret.handle); err == nil {
		ret.id = fmt.Sprintf("%d", id)
	} else {
		id, err := db.idGen.NextID()
		if err != nil {
			return nil, types.ErrDatabaseConnectID(err.Error())
		}
		ret.id = fmt.Sprintf("%X", id)
	}

	statz.AllocConn()
	if statz.Debug {
		_, file, no, ok := runtime.Caller(1)
		if ok {
			fmt.Printf("Conn.Connect() from %s#%d\n", file, no)
		}
	}
	ret.closeCallback = func() {
		ret.SetLatestSql("CLOSE") // 3. set latest sql time
		db.RemoveWatcher(ret.id)
	}
	db.RegisterWatcher(ret.id, ret) // 1. set creTime
	ret.SetLatestSql("CONNECT")     // 2. set latest sql time
	return ret, nil
}

// Close closes connection
func (conn *Conn) Close() (err error) {
	if statz.Debug {
		_, file, no, ok := runtime.Caller(1)
		if ok {
			fmt.Printf("Conn.Close() from %s#%d\n", file, no)
		}
	}
	conn.closeOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered in Conn.Close", r)
			}
		}()
		conn.closed = true
		statz.FreeConn()
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

func (conn *Conn) Ping() (time.Duration, error) {
	return 0, nil
}

// ExecContext executes SQL statements that does not return result
// like 'ALTER', 'CREATE TABLE', 'DROP TABLE', ...
func (conn *Conn) Exec(ctx context.Context, sqlText string, params ...any) *Result {
	conn.SetLatestSql(sqlText)
	var result = &Result{}
	var stmt unsafe.Pointer
	if result.err = mach.EngAllocStmt(conn.handle, &stmt); result.err != nil {
		return result
	}
	statz.AllocStmt()
	defer func() {
		mach.EngFreeStmt(stmt)
		statz.FreeStmt()
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
func (conn *Conn) Query(ctx context.Context, sqlText string, params ...any) (*Rows, error) {
	conn.SetLatestSql(sqlText)
	rows := &Rows{
		sqlText: sqlText,
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
	statz.AllocStmt()
	return rows, nil
}

func stmtColumns(stmt unsafe.Pointer) (types.Columns, error) {
	columnCount, err := mach.EngColumnCount(stmt)
	if err != nil {
		return nil, err
	}
	ret := make(types.Columns, columnCount)
	for i := 0; i < columnCount; i++ {
		var columnName string
		var columnRawType, columnSize, columnLength int
		err = mach.EngColumnInfo(stmt, i, &columnName, &columnRawType, &columnSize, &columnLength)
		if err != nil {
			return nil, err
		}
		typ, err := columnRawTypeToDataType(columnRawType)
		if err != nil {
			return nil, mach.ErrDatabaseWrap("Invalid column type", err)
		}
		col := &types.Column{
			Name:     strings.ToUpper(columnName),
			DataType: typ,
			Length:   columnLength,
		}
		ret[i] = col
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

func columnRawTypeToDataType(rawType int) (types.DataType, error) {
	switch rawType {
	case ColumnRawTypeInt16:
		return types.DataTypeInt16, nil
	case ColumnRawTypeInt32:
		return types.DataTypeInt32, nil
	case ColumnRawTypeInt64:
		return types.DataTypeInt64, nil
	case ColumnRawTypeDatetime:
		return types.DataTypeDatetime, nil
	case ColumnRawTypeFloat32:
		return types.DataTypeFloat32, nil
	case ColumnRawTypeFloat64:
		return types.DataTypeFloat64, nil
	case ColumnRawTypeIPv4:
		return types.DataTypeIPv4, nil
	case ColumnRawTypeIPv6:
		return types.DataTypeIPv6, nil
	case ColumnRawTypeString:
		return types.DataTypeString, nil
	case ColumnRawTypeBinary:
		return types.DataTypeBinary, nil
	default:
		return "", types.ErrDatabaseUnsupportedType("ColumnType", rawType)
	}
}

func columnDataTypeToRawType(typ types.DataType) (int, error) {
	switch typ {
	case types.DataTypeInt16:
		return ColumnRawTypeInt16, nil
	case types.DataTypeInt32:
		return ColumnRawTypeInt32, nil
	case types.DataTypeInt64:
		return ColumnRawTypeInt64, nil
	case types.DataTypeDatetime:
		return ColumnRawTypeDatetime, nil
	case types.DataTypeFloat32:
		return ColumnRawTypeFloat32, nil
	case types.DataTypeFloat64:
		return ColumnRawTypeFloat64, nil
	case types.DataTypeIPv4:
		return ColumnRawTypeIPv4, nil
	case types.DataTypeIPv6:
		return ColumnRawTypeIPv6, nil
	case types.DataTypeString:
		return ColumnRawTypeString, nil
	case types.DataTypeBinary:
		return ColumnRawTypeBinary, nil
	default:
		return 0, types.ErrDatabaseUnsupportedTypeName("DataType", string(typ))
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
func (conn *Conn) QueryRow(ctx context.Context, sqlText string, params ...any) *Row {
	conn.SetLatestSql(sqlText)
	var row = &Row{}
	var stmt unsafe.Pointer
	statz.AllocStmt()
	if row.err = mach.EngAllocStmt(conn.handle, &stmt); row.err != nil {
		return row
	}
	defer func() {
		statz.FreeStmt()
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
			row.err = readColumnData(stmt, rawType, i, row.values[i])
			if row.err != nil {
				return row
			}
		}
	}
	if row.err == nil {
		row.ok = true
	}
	return row
}

func (conn *Conn) Explain(ctx context.Context, sqlText string, full bool) (string, error) {
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

type Statz struct {
	Conns          int64
	Stmts          int64
	Appenders      int64
	ConnsInUse     int32
	StmtsInUse     int32
	AppendersInUse int32
	RawConns       int32
	Debug          bool
}

var statz Statz

func (s *Statz) AllocConn() {
	atomic.AddInt32(&s.ConnsInUse, 1)
	atomic.AddInt64(&s.Conns, 1)
}

func (s *Statz) FreeConn() {
	atomic.AddInt32(&s.ConnsInUse, -1)
}

func (s *Statz) AllocStmt() {
	atomic.AddInt32(&s.StmtsInUse, 1)
	atomic.AddInt64(&s.Stmts, 1)
}

func (s *Statz) FreeStmt() {
	atomic.AddInt32(&s.StmtsInUse, -1)
}

func (s *Statz) AllocAppender() {
	atomic.AddInt32(&s.AppendersInUse, 1)
	atomic.AddInt64(&s.Appenders, 1)
}

func (s *Statz) FreeAppender() {
	atomic.AddInt32(&s.AppendersInUse, -1)
}

func StatzDebug(flag bool) {
	statz.Debug = flag
}

func StatzSnapshot() *Statz {
	ret := statz
	if _env.handle != nil {
		ret.RawConns = int32(mach.EngConnectionCount(_env.handle))
	}
	return &ret
}
