package machgo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machnet"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	defaultQueryStmtPoolCap         = 128
	defaultQueryStmtPoolPerQueryCap = 8
)

type queryStmtReuseMode int

const (
	queryStmtReuseOff queryStmtReuseMode = iota
	queryStmtReuseOn
	queryStmtReuseAuto
)

func errorWithCause(obj any, cause error) error {
	var handle unsafe.Pointer
	var handleType machnet.HandleType
	switch x := obj.(type) {
	case *Database:
		handle = x.handle
		handleType = machnet.MACHCLI_HANDLE_ENV
	case *Conn:
		handle = x.handle
		handleType = machnet.MACHCLI_HANDLE_DBC
	case *Stmt:
		handle = x.handle
		handleType = machnet.MACHCLI_HANDLE_STMT
	default:
		return cause
	}
	var code int
	var msg string
	if reErr := machnet.CliError(handle, handleType, &code, &msg); reErr != nil {
		if cause == nil {
			return fmt.Errorf("MACHCLI Fail to get error, %s", reErr.Error())
		} else {
			return fmt.Errorf("MACHCLI Fail to get error code of %s, %s", cause.Error(), reErr.Error())
		}
	} else if code == 0 && msg == "" && cause == nil {
		// no error
		return nil
	} else if code == 0 && msg != "" {
		// code == 0 means client-side error
		if cause == nil {
			return fmt.Errorf("MACHCLI %s", msg)
		} else {
			return fmt.Errorf("MACHCLI %s, %s", msg, cause.Error())
		}
	} else {
		// code > 0 means server-side error
		if cause == nil {
			return fmt.Errorf("MACHCLI-ERR-%d, %s", code, msg)
		} else {
			return fmt.Errorf("MACHCLI-ERR-%d, %s", code, msg)
		}
	}
}

func parseQueryStmtReuseMode() queryStmtReuseMode {
	legacy := strings.ToLower(strings.TrimSpace(os.Getenv("MACHCLI2_QUERY_STMT_REUSE")))
	switch legacy {
	case "1", "true", "on", "yes":
		return queryStmtReuseOn
	case "0", "false", "off", "no":
		return queryStmtReuseOff
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MACHCLI2_QUERY_STMT_REUSE_MODE"))) {
	case "on":
		return queryStmtReuseOn
	case "off":
		return queryStmtReuseOff
	case "auto":
		return queryStmtReuseAuto
	default:
		return queryStmtReuseAuto
	}
}

type Config struct {
	Host string
	Port int

	AlternativeHost string
	AlternativePort int

	// MaxOpenConns
	//
	//	< 0 : unlimited
	//	0 : default, maxOpenConns = CPU count * maxOpenConnsFactor
	//	> 0 : specified max open connections
	MaxOpenConn int

	// MaxOpenConnsFactor
	//
	//	used to calculate the number of max open connections when maxOpenConns is 0
	//	default is 1.5
	MaxOpenConnFactor float64

	MaxOpenQuery       int
	MaxOpenQueryFactor float64

	TrustUsers map[string]string

	// unused
	ConType int
}

type Database struct {
	handle unsafe.Pointer
	host   string
	port   int

	alternativeHost string
	alternativePort int

	trustUsersMutex sync.RWMutex
	trustUsers      map[string]string

	maxConnsMutex sync.RWMutex
	maxConnsChan  chan struct{}
}

var _ api.Database = (*Database)(nil)

func NewDatabase(conf *Config) (*Database, error) {
	handle := new(unsafe.Pointer)
	if err := machnet.CliInitialize(handle); err != nil {
		return nil, err
	}
	ret := &Database{
		host:            conf.Host,
		port:            conf.Port,
		alternativeHost: conf.AlternativeHost,
		alternativePort: conf.AlternativePort,
		handle:          *handle,
		trustUsers:      map[string]string{},
	}
	for u, p := range conf.TrustUsers {
		ret.trustUsers[strings.ToUpper(u)] = p
	}

	if conf.MaxOpenConnFactor <= 0 {
		conf.MaxOpenConnFactor = 1.5
	}

	if conf.MaxOpenConn < 0 {
		conf.MaxOpenConn = -1
	} else if conf.MaxOpenConn == 0 {
		conf.MaxOpenConn = int(float64(runtime.NumCPU()) * conf.MaxOpenConnFactor)
	}

	ret.SetMaxOpenConns(conf.MaxOpenConn)
	return ret, nil
}

func (db *Database) Close() error {
	if err := machnet.CliFinalize(db.handle); err == nil {
		return nil
	} else {
		return errorWithCause(db, err)
	}
}

// MaxOpenConns returns the maximum number of open connections
// and the current remains capacity.
func (db *Database) MaxOpenConns() (int, int) {
	db.maxConnsMutex.RLock()
	defer db.maxConnsMutex.RUnlock()
	if db.maxConnsChan == nil {
		// unlimited
		return -1, -1
	}
	limit := cap(db.maxConnsChan)
	remains := len(db.maxConnsChan)
	return limit, remains
}

func (db *Database) SetMaxOpenConns(desiredMaxOpenConns int) {
	if desiredMaxOpenConns < 0 {
		desiredMaxOpenConns = -1
	}
	if desiredMaxOpenConns == 0 {
		desiredMaxOpenConns = int(float64(runtime.NumCPU()) * 1.5)
	}

	currentCap := cap(db.maxConnsChan)
	if currentCap == desiredMaxOpenConns {
		return
	}

	var newChan chan struct{}
	db.maxConnsMutex.Lock()
	defer func() {
		db.maxConnsChan = newChan
		db.maxConnsMutex.Unlock()
	}()

	if desiredMaxOpenConns > 0 {
		newChan = make(chan struct{}, desiredMaxOpenConns)
		for i := 0; i < desiredMaxOpenConns; i++ {
			newChan <- struct{}{}
		}
	}
}

func (db *Database) Ping(ctx context.Context) (time.Duration, error) {
	tick := time.Now()
	db.trustUsersMutex.RLock()
	var user, pass string
	for u, p := range db.trustUsers {
		user = u
		pass = p
		break
	}
	db.trustUsersMutex.RUnlock()
	if user == "" {
		return 0, errors.New("ping requires at least one trust user")
	}
	conn, err := db.Connect(ctx, api.WithPassword(user, pass))
	if err != nil {
		return time.Since(tick), err
	}
	if err := conn.Close(); err != nil {
		return time.Since(tick), err
	}
	return time.Since(tick), nil
}

func (db *Database) SetTrustUser(user, password string) error {
	db.trustUsersMutex.Lock()
	defer db.trustUsersMutex.Unlock()
	db.trustUsers[strings.ToUpper(user)] = password
	return nil
}

func (db *Database) getTrustUser(user string) (string, bool) {
	db.trustUsersMutex.RLock()
	defer db.trustUsersMutex.RUnlock()
	pass, ok := db.trustUsers[strings.ToUpper(user)]
	return pass, ok
}

func (db *Database) UserAuth(ctx context.Context, user, password string) (bool, string, error) {
	conn, err := db.Connect(ctx, api.WithPassword(user, password))
	if err != nil {
		return false, "invalid username or password", nil
	}
	err = conn.Close()
	return true, "", err
}

func (db *Database) connectionString(user string, password string) string {
	entries := []string{
		fmt.Sprintf("SERVER=%s", db.host),
		fmt.Sprintf("PORT_NO=%d", db.port),
		fmt.Sprintf("UID=%s", strings.ToUpper(user)),
		fmt.Sprintf("PWD=%s", strings.ToUpper(password)),
		"CONNTYPE=1",
	}
	if db.alternativeHost != "" && db.alternativePort != 0 {
		entries = append(entries,
			fmt.Sprintf("ALTERNATIVE_SERVERS=%s:%d",
				db.alternativeHost, db.alternativePort))
	}
	return strings.Join(entries, ";")
}

func (db *Database) Connect(ctx context.Context, opts ...api.ConnectOption) (api.Conn, error) {
	var user, password string
	for _, opt := range opts {
		switch o := opt.(type) {
		case *api.ConnectOptionPassword:
			user = o.User
			password = o.Password
		case *api.ConnectOptionTrustUser:
			if pass, ok := db.getTrustUser(o.User); ok {
				user = strings.ToUpper(o.User)
				password = pass
			}
			if user == "" {
				return nil, errors.New("trust user not found")
			}
		default:
			return nil, fmt.Errorf("unknown option type-%T", o)
		}
	}

	returnChan := db.maxConnsChan

	if returnChan != nil {
		select {
		case <-returnChan:
		case <-ctx.Done():
			return nil, api.NewError("connect canceled")
		}
	}

	handle := new(unsafe.Pointer)
	if err := machnet.CliConnect(db.handle, db.connectionString(user, password), handle); err != nil {
		return nil, errorWithCause(db, err)
	}

	db.SetTrustUser(user, password)

	ret := &Conn{
		db:                     db,
		handle:                 *handle,
		user:                   strings.ToUpper(user),
		usedAt:                 time.Now(),
		returnChan:             returnChan,
		queryStmtReuseMode:     parseQueryStmtReuseMode(),
		queryStmtPool:          map[string][]*Stmt{},
		queryStmtPoolCap:       defaultQueryStmtPoolCap,
		queryStmtPoolPerKeyCap: defaultQueryStmtPoolPerQueryCap,
	}
	return ret, nil
}

type Conn struct {
	handle unsafe.Pointer
	db     *Database

	user       string
	usedAt     time.Time
	usedCount  int64
	closeOnce  sync.Once
	returnChan chan struct{}

	queryStmtReuseMode     queryStmtReuseMode
	queryStmtPoolMu        sync.Mutex
	queryStmtFastKey       string
	queryStmtFast          *Stmt
	queryStmtPool          map[string][]*Stmt
	queryStmtPoolCount     int
	queryStmtPoolCap       int
	queryStmtPoolPerKeyCap int
}

var _ api.Conn = (*Conn)(nil)

func (c *Conn) Close() (ret error) {
	c.closeOnce.Do(func() {
		defer func() {
			c.usedAt = time.Now()
			c.usedCount++
			if c.returnChan != nil {
				c.returnChan <- struct{}{}
			}
		}()
		if err := c.closeQueryStmtPool(); err != nil && ret == nil {
			ret = err
		}
		if err := machnet.CliDisconnect(c.handle); err != nil {
			if ret == nil {
				ret = errorWithCause(c, err)
			}
		}
	})
	return ret
}

func (c *Conn) Error() error {
	return errorWithCause(c, nil)
}

func queryHead(query string) string {
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return ""
	}
	return strings.ToUpper(parts[0])
}

func (c *Conn) shouldReuseStmtForQuery(query string) bool {
	switch c.queryStmtReuseMode {
	case queryStmtReuseOn:
		return true
	case queryStmtReuseOff:
		return false
	default:
		switch queryHead(query) {
		case "SELECT", "WITH", "INSERT", "UPDATE", "DELETE", "MERGE", "UPSERT", "REPLACE":
			return true
		default:
			return false
		}
	}
}

func (c *Conn) acquireQueryStmt(query string) (*Stmt, error) {
	if !c.shouldReuseStmtForQuery(query) {
		stmt, err := c.NewStmt()
		if err != nil {
			return nil, err
		}
		stmt.sqlHead = queryHead(query)
		if err := stmt.prepare(query); err != nil {
			_ = stmt.Close()
			return nil, err
		}
		return stmt, nil
	}
	c.queryStmtPoolMu.Lock()
	if c.queryStmtFast != nil && c.queryStmtFastKey == query {
		stmt := c.queryStmtFast
		c.queryStmtFast = nil
		c.queryStmtFastKey = ""
		c.queryStmtPoolMu.Unlock()

		stmt.sqlHead = queryHead(query)
		stmt.reachEOF = false
		stmt.boundBuffers = nil
		return stmt, nil
	}
	if pool := c.queryStmtPool[query]; len(pool) > 0 {
		idx := len(pool) - 1
		stmt := pool[idx]
		if idx == 0 {
			delete(c.queryStmtPool, query)
		} else {
			c.queryStmtPool[query] = pool[:idx]
		}
		if c.queryStmtPoolCount > 0 {
			c.queryStmtPoolCount--
		}
		c.queryStmtPoolMu.Unlock()

		stmt.sqlHead = queryHead(query)
		stmt.reachEOF = false
		stmt.boundBuffers = nil
		return stmt, nil
	}
	c.queryStmtPoolMu.Unlock()

	stmt, err := c.NewStmt()
	if err != nil {
		return nil, err
	}
	stmt.sqlHead = queryHead(query)
	if err := stmt.prepare(query); err != nil {
		_ = stmt.Close()
		return nil, err
	}
	return stmt, nil
}

func (c *Conn) releaseQueryStmt(query string, stmt *Stmt, reusable bool) error {
	if stmt == nil {
		return nil
	}
	if !c.shouldReuseStmtForQuery(query) {
		return stmt.Close()
	}
	if !reusable {
		return stmt.Close()
	}
	stmt.reachEOF = false
	stmt.boundBuffers = nil
	if err := machnet.CliExecuteClean(stmt.handle); err != nil {
		_ = stmt.Close()
		return errorWithCause(stmt, err)
	}

	keep := false
	c.queryStmtPoolMu.Lock()
	if c.queryStmtFast == nil {
		c.queryStmtFast = stmt
		c.queryStmtFastKey = query
		keep = true
	} else if c.queryStmtPool != nil &&
		c.queryStmtPoolCount < c.queryStmtPoolCap &&
		len(c.queryStmtPool[query]) < c.queryStmtPoolPerKeyCap {
		c.queryStmtPool[query] = append(c.queryStmtPool[query], stmt)
		c.queryStmtPoolCount++
		keep = true
	}
	c.queryStmtPoolMu.Unlock()

	if keep {
		return nil
	}
	return stmt.Close()
}

func (c *Conn) closeQueryStmtPool() error {
	if c.queryStmtReuseMode == queryStmtReuseOff {
		return nil
	}
	c.queryStmtPoolMu.Lock()
	if len(c.queryStmtPool) == 0 && c.queryStmtFast == nil {
		c.queryStmtPoolMu.Unlock()
		return nil
	}
	capHint := c.queryStmtPoolCount
	if c.queryStmtFast != nil {
		capHint++
	}
	stmts := make([]*Stmt, 0, capHint)
	if c.queryStmtFast != nil {
		stmts = append(stmts, c.queryStmtFast)
		c.queryStmtFast = nil
		c.queryStmtFastKey = ""
	}
	for key, pool := range c.queryStmtPool {
		stmts = append(stmts, pool...)
		delete(c.queryStmtPool, key)
	}
	c.queryStmtPoolCount = 0
	c.queryStmtPoolMu.Unlock()

	var firstErr error
	for _, stmt := range stmts {
		if stmt == nil {
			continue
		}
		if err := stmt.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Conn) Explain(ctx context.Context, query string, full bool) (string, error) {
	if full {
		query = "explain full " + query
	} else {
		query = "explain " + query
	}

	var stmt unsafe.Pointer
	if err := machnet.CliAllocStmt(c.handle, &stmt); err != nil {
		return "", errorWithCause(c, err)
	}
	defer machnet.CliFreeStmt(stmt)

	if err := machnet.CliExecDirect(stmt, query); err != nil {
		return "", errorWithCause(c, err)
	}

	ret := make([]string, 0, 20)
	buf := make([]byte, 256)
	for {
		if eof, err := machnet.CliFetch(stmt); err != nil {
			return "", errorWithCause(c, err)
		} else if eof {
			break
		}
		if n, err := machnet.CliGetData(stmt, 0, machnet.MACHCLI_C_TYPE_CHAR, (unsafe.Pointer)(&buf[0]), len(buf)); err != nil {
			return "", errorWithCause(c, err)
		} else {
			ret = append(ret, string(buf[:n]))
		}
	}
	return strings.Join(ret, "\n"), nil
}

func (c *Conn) Exec(ctx context.Context, query string, args ...any) api.Result {
	ret := &Result{}
	if len(args) == 0 {
		stmt, err := c.NewStmt()
		if err != nil {
			ret.err = err
			return ret
		}
		defer stmt.Close()

		stmt.sqlHead = queryHead(query)
		if err := machnet.CliExecDirect(stmt.handle, query); err != nil {
			ret.err = errorWithCause(c, err)
		}
		ret.rowCount, ret.err = machnet.CliRowCount(stmt.handle)
		if typ, err := machnet.CliGetStmtType(stmt.handle); err != nil {
			ret.err = err
			return ret
		} else {
			ret.stmtType = machnet.StmtType(typ)
		}
		return ret
	}

	stmt, err := c.acquireQueryStmt(query)
	if err != nil {
		ret.err = err
		return ret
	}
	defer func() {
		keep := ret.err == nil
		if relErr := c.releaseQueryStmt(query, stmt, keep); relErr != nil && ret.err == nil {
			ret.err = relErr
		}
	}()

	if ret.err = stmt.bindParams(args...); ret.err != nil {
		return ret
	}
	ret.err = stmt.execute()
	ret.rowCount = stmt.rowCount
	if typ, err := machnet.CliGetStmtType(stmt.handle); err != nil {
		ret.err = err
		return ret
	} else {
		ret.stmtType = machnet.StmtType(typ)
	}
	return ret
}

func (c *Conn) Prepare(ctx context.Context, query string) (api.Stmt, error) {
	stmt, err := c.NewStmt()
	if err != nil {
		return nil, err
	}

	stmt.sqlHead = queryHead(query)
	if err := stmt.prepare(query); err != nil {
		return nil, err
	}
	return &PreparedStmt{stmt: stmt}, nil
}

func (c *Conn) QueryRow(ctx context.Context, query string, args ...any) api.Row {
	ret := &Row{}
	stmt, err := c.acquireQueryStmt(query)
	if err != nil {
		ret.err = err
		return ret
	}
	defer func() {
		keep := ret.err == nil || errors.Is(ret.err, sql.ErrNoRows)
		if relErr := c.releaseQueryStmt(query, stmt, keep); relErr != nil && ret.err == nil {
			ret.err = relErr
		}
	}()
	if ret.err = stmt.bindParams(args...); ret.err != nil {
		return ret
	}
	if ret.err = stmt.execute(); ret.err != nil {
		return ret
	}
	if typ, err := machnet.CliGetStmtType(stmt.handle); err != nil {
		ret.err = err
		return ret
	} else {
		ret.stmtType = machnet.StmtType(typ)
	}
	ret.rowCount = stmt.rowCount
	if values, err := stmt.fetch(); err != nil {
		if err == io.EOF {
			// it means no row fetched
			ret.err = sql.ErrNoRows
		} else {
			ret.err = err
		}
		return ret
	} else {
		ret.values = values
	}
	ret.columns = make(api.Columns, len(stmt.columnDescs))
	for i, desc := range stmt.columnDescs {
		ret.columns[i] = &api.Column{
			Name:     desc.Name,
			Length:   desc.Size,
			Type:     desc.Type.ColumnType(),
			DataType: desc.Type.DataType(),
		}
	}
	return ret
}

func (c *Conn) Query(ctx context.Context, query string, args ...any) (api.Rows, error) {
	stmt, err := c.acquireQueryStmt(query)
	if err != nil {
		return nil, err
	}
	if err := stmt.bindParams(args...); err != nil {
		if relErr := c.releaseQueryStmt(query, stmt, false); relErr != nil {
			return nil, relErr
		}
		return nil, err
	}
	if err := stmt.execute(); err != nil {
		if relErr := c.releaseQueryStmt(query, stmt, false); relErr != nil {
			return nil, relErr
		}
		return nil, err
	}
	ret := &Rows{
		stmt:            stmt,
		queryStmtKey:    query,
		queryStmtPooled: true,
	}
	if typ, err := machnet.CliGetStmtType(stmt.handle); err != nil {
		if relErr := c.releaseQueryStmt(query, stmt, false); relErr != nil {
			return nil, relErr
		}
		return nil, err
	} else {
		ret.stmtType = machnet.StmtType(typ)
	}
	ret.rowsCount = stmt.rowCount
	return ret, nil
}

type PreparedStmt struct {
	stmt *Stmt
}

func (pStmt *PreparedStmt) Close() error {
	if pStmt.stmt == nil {
		return nil
	}
	err := pStmt.stmt.Close()
	pStmt.stmt = nil
	return err
}

func (pStmt *PreparedStmt) Exec(ctx context.Context, params ...any) api.Result {
	defer machnet.CliExecuteClean(pStmt.stmt.handle)
	ret := &Result{}
	if err := pStmt.stmt.bindParams(params...); err != nil {
		ret.err = err
		return ret
	}
	if err := pStmt.stmt.execute(); err != nil {
		ret.err = err
		return ret
	}
	ret.rowCount = pStmt.stmt.rowCount
	if typ, err := machnet.CliGetStmtType(pStmt.stmt.handle); err != nil {
		ret.err = err
		return ret
	} else {
		ret.stmtType = machnet.StmtType(typ)
	}
	return ret
}

func (pStmt *PreparedStmt) Query(ctx context.Context, params ...any) (api.Rows, error) {
	if err := pStmt.stmt.bindParams(params...); err != nil {
		return nil, err
	}
	if err := pStmt.stmt.execute(); err != nil {
		return nil, err
	}
	ret := &Rows{
		stmt:       pStmt.stmt,
		isPrepared: true,
	}
	ret.rowsCount = pStmt.stmt.rowCount
	return ret, nil
}

func (pStmt *PreparedStmt) QueryRow(ctx context.Context, params ...any) api.Row {
	ret := &Row{}
	if err := pStmt.stmt.bindParams(params...); err != nil {
		ret.err = err
		return ret
	}
	if err := pStmt.stmt.execute(); err != nil {
		ret.err = err
		return ret
	}
	ret.rowCount = pStmt.stmt.rowCount
	if values, err := pStmt.stmt.fetch(); err != nil {
		if err == io.EOF {
			// it means no row fetched
			ret.err = sql.ErrNoRows
		} else {
			ret.err = err
		}
		return ret
	} else {
		ret.values = values
	}
	ret.columns = make(api.Columns, len(pStmt.stmt.columnDescs))
	for i, desc := range pStmt.stmt.columnDescs {
		ret.columns[i] = &api.Column{
			Name:     desc.Name,
			Length:   desc.Size,
			Type:     desc.Type.ColumnType(),
			DataType: desc.Type.DataType(),
		}
	}
	return ret
}

func (stmt *Stmt) prepare(query string) error {
	if err := machnet.CliPrepare(stmt.handle, query); err != nil {
		return errorWithCause(stmt, err)
	}
	return nil
}

func (stmt *Stmt) bindParams(args ...any) error {
	numParam, err := machnet.CliNumParam(stmt.handle)
	if err != nil {
		return errorWithCause(stmt, err)
	}
	if len(args) != numParam {
		return api.ErrParamCount(numParam, len(args))
	}

	// Clear previous buffers
	stmt.boundBuffers = make([][]byte, 0, numParam)

	for idx, arg := range args {
		var value unsafe.Pointer
		var valueLen int
		var cType machnet.CType
		var sqlType machnet.SqlType
		switch val := arg.(type) {
		default:
			pd, _ := machnet.CliDescribeParam(stmt.handle, idx)
			if val == nil {
				cType = machnet.MACHCLI_C_TYPE_CHAR
				sqlType = pd.Type
				value = nil
				valueLen = 0
			} else {
				return api.ErrDatabaseBindUnknownType(idx, fmt.Sprintf("%T, expect: %d", val, pd.Type))
			}
		case int16:
			cType = machnet.MACHCLI_C_TYPE_INT16
			sqlType = machnet.MACHCLI_SQL_TYPE_INT16
			value = unsafe.Pointer(&val)
			valueLen = 2
		case *int16:
			cType = machnet.MACHCLI_C_TYPE_INT16
			sqlType = machnet.MACHCLI_SQL_TYPE_INT16
			value = unsafe.Pointer(val)
			valueLen = 2
		case int32:
			cType = machnet.MACHCLI_C_TYPE_INT32
			sqlType = machnet.MACHCLI_SQL_TYPE_INT32
			value = unsafe.Pointer(&val)
			valueLen = 4
		case *int32:
			cType = machnet.MACHCLI_C_TYPE_INT32
			sqlType = machnet.MACHCLI_SQL_TYPE_INT32
			value = unsafe.Pointer(val)
			valueLen = 4
		case int:
			cType = machnet.MACHCLI_C_TYPE_INT32
			sqlType = machnet.MACHCLI_SQL_TYPE_INT32
			value = unsafe.Pointer(&val)
			valueLen = 4
		case *int:
			cType = machnet.MACHCLI_C_TYPE_INT32
			sqlType = machnet.MACHCLI_SQL_TYPE_INT32
			value = unsafe.Pointer(val)
			valueLen = 4
		case int64:
			cType = machnet.MACHCLI_C_TYPE_INT64
			sqlType = machnet.MACHCLI_SQL_TYPE_INT64
			value = unsafe.Pointer(&val)
			valueLen = 8
		case *int64:
			cType = machnet.MACHCLI_C_TYPE_INT64
			sqlType = machnet.MACHCLI_SQL_TYPE_INT64
			value = unsafe.Pointer(val)
			valueLen = 8
		case time.Time:
			cType = machnet.MACHCLI_C_TYPE_INT64
			sqlType = machnet.MACHCLI_SQL_TYPE_DATETIME
			n := new(int64)
			*n = val.UnixNano()
			value = unsafe.Pointer(n)
			valueLen = 8
		case *time.Time:
			cType = machnet.MACHCLI_C_TYPE_INT64
			sqlType = machnet.MACHCLI_SQL_TYPE_DATETIME
			n := new(int64)
			*n = val.UnixNano()
			value = unsafe.Pointer(n)
			valueLen = 8
		case float32:
			cType = machnet.MACHCLI_C_TYPE_FLOAT
			sqlType = machnet.MACHCLI_SQL_TYPE_FLOAT
			value = unsafe.Pointer(&val)
			valueLen = 4
		case *float32:
			cType = machnet.MACHCLI_C_TYPE_FLOAT
			sqlType = machnet.MACHCLI_SQL_TYPE_FLOAT
			value = unsafe.Pointer(val)
			valueLen = 4
		case float64:
			cType = machnet.MACHCLI_C_TYPE_DOUBLE
			sqlType = machnet.MACHCLI_SQL_TYPE_DOUBLE
			value = unsafe.Pointer(&val)
			valueLen = 8
		case *float64:
			cType = machnet.MACHCLI_C_TYPE_DOUBLE
			sqlType = machnet.MACHCLI_SQL_TYPE_DOUBLE
			value = unsafe.Pointer(val)
			valueLen = 8
		case net.IP:
			if ipv4 := val.To4(); ipv4 != nil {
				cType = machnet.MACHCLI_C_TYPE_CHAR
				sqlType = machnet.MACHCLI_SQL_TYPE_IPV4
				bStr := []byte(ipv4.String())
				if len(bStr) > 0 {
					value = unsafe.Pointer(&bStr[0])
				}
				valueLen = len(bStr)
				stmt.boundBuffers = append(stmt.boundBuffers, bStr)
			} else {
				cType = machnet.MACHCLI_C_TYPE_CHAR
				sqlType = machnet.MACHCLI_SQL_TYPE_IPV6
				bStr := []byte(val.To16().String())
				if len(bStr) > 0 {
					value = unsafe.Pointer(&bStr[0])
				}
				valueLen = len(bStr)
				stmt.boundBuffers = append(stmt.boundBuffers, bStr)
			}
		case string:
			cType = machnet.MACHCLI_C_TYPE_CHAR
			sqlType = machnet.MACHCLI_SQL_TYPE_STRING
			bStr := []byte(val)
			if len(bStr) > 0 {
				value = unsafe.Pointer(&bStr[0])
			}
			valueLen = len(bStr)
			stmt.boundBuffers = append(stmt.boundBuffers, bStr)
		case *string:
			cType = machnet.MACHCLI_C_TYPE_CHAR
			sqlType = machnet.MACHCLI_SQL_TYPE_STRING
			bStr := []byte(*val)
			if len(bStr) > 0 {
				value = unsafe.Pointer(&bStr[0])
			}
			valueLen = len(bStr)
			stmt.boundBuffers = append(stmt.boundBuffers, bStr)
		case []byte:
			cType = machnet.MACHCLI_C_TYPE_CHAR
			sqlType = machnet.MACHCLI_SQL_TYPE_BINARY
			if len(val) > 0 {
				value = unsafe.Pointer(&val[0])
			}
			valueLen = len(val)
			stmt.boundBuffers = append(stmt.boundBuffers, val)
		}
		if err := machnet.CliBindParam(stmt.handle, idx, cType, sqlType, value, valueLen); err != nil {
			return errorWithCause(stmt, err)
		}
	}
	// Keep bound buffers alive until CliExecute completes
	runtime.KeepAlive(stmt.boundBuffers)
	return nil
}

type Result struct {
	message  string
	err      error
	rowCount int64
	stmtType machnet.StmtType
}

var _ api.Result = (*Result)(nil)

func (rs *Result) Message() string {
	return rs.message
}

func (rs *Result) Err() error {
	return rs.err
}

func (rs *Result) LastInsertId() (int64, error) {
	return 0, api.ErrNotImplemented("LastInsertId")
}

func (rs *Result) RowsAffected() int64 {
	return rs.rowCount
}

func (c *Conn) NewStmt() (*Stmt, error) {
	var handle unsafe.Pointer
	if err := machnet.CliAllocStmt(c.handle, &handle); err != nil {
		return nil, errorWithCause(c, err)
	}
	ret := &Stmt{conn: c, handle: handle}
	return ret, nil
}

type SqlType int

const (
	MACHCLI_SQL_TYPE_INT16    SqlType = 0
	MACHCLI_SQL_TYPE_INT32    SqlType = 1
	MACHCLI_SQL_TYPE_INT64    SqlType = 2
	MACHCLI_SQL_TYPE_DATETIME SqlType = 3
	MACHCLI_SQL_TYPE_FLOAT    SqlType = 4
	MACHCLI_SQL_TYPE_DOUBLE   SqlType = 5
	MACHCLI_SQL_TYPE_IPV4     SqlType = 6
	MACHCLI_SQL_TYPE_IPV6     SqlType = 7
	MACHCLI_SQL_TYPE_STRING   SqlType = 8
	MACHCLI_SQL_TYPE_BINARY   SqlType = 9
)

func (st SqlType) String() string {
	switch st {
	case MACHCLI_SQL_TYPE_INT16:
		return "INT16"
	case MACHCLI_SQL_TYPE_INT32:
		return "INT32"
	case MACHCLI_SQL_TYPE_INT64:
		return "INT64"
	case MACHCLI_SQL_TYPE_DATETIME:
		return "DATETIME"
	case MACHCLI_SQL_TYPE_FLOAT:
		return "FLOAT"
	case MACHCLI_SQL_TYPE_DOUBLE:
		return "DOUBLE"
	case MACHCLI_SQL_TYPE_IPV4:
		return "IPV4"
	case MACHCLI_SQL_TYPE_IPV6:
		return "IPV6"
	case MACHCLI_SQL_TYPE_STRING:
		return "STRING"
	case MACHCLI_SQL_TYPE_BINARY:
		return "BINARY"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", st)
	}
}

func (st SqlType) ColumnType() api.ColumnType {
	switch st {
	default:
		return api.ColumnTypeUnknown
	case MACHCLI_SQL_TYPE_INT16:
		return api.ColumnTypeShort
	case MACHCLI_SQL_TYPE_INT32:
		return api.ColumnTypeInteger
	case MACHCLI_SQL_TYPE_INT64:
		return api.ColumnTypeLong
	case MACHCLI_SQL_TYPE_DATETIME:
		return api.ColumnTypeDatetime
	case MACHCLI_SQL_TYPE_FLOAT:
		return api.ColumnTypeFloat
	case MACHCLI_SQL_TYPE_DOUBLE:
		return api.ColumnTypeDouble
	case MACHCLI_SQL_TYPE_IPV4:
		return api.ColumnTypeIPv4
	case MACHCLI_SQL_TYPE_IPV6:
		return api.ColumnTypeIPv6
	case MACHCLI_SQL_TYPE_STRING:
		return api.ColumnTypeVarchar
	case MACHCLI_SQL_TYPE_BINARY:
		return api.ColumnTypeBinary
	}
}

func (st SqlType) DataType() api.DataType {
	switch st {
	default:
		return api.DataTypeAny
	case MACHCLI_SQL_TYPE_INT16:
		return api.DataTypeInt16
	case MACHCLI_SQL_TYPE_INT32:
		return api.DataTypeInt32
	case MACHCLI_SQL_TYPE_INT64:
		return api.DataTypeInt64
	case MACHCLI_SQL_TYPE_DATETIME:
		return api.DataTypeDatetime
	case MACHCLI_SQL_TYPE_FLOAT:
		return api.DataTypeFloat32
	case MACHCLI_SQL_TYPE_DOUBLE:
		return api.DataTypeFloat64
	case MACHCLI_SQL_TYPE_IPV4:
		return api.DataTypeIPv4
	case MACHCLI_SQL_TYPE_IPV6:
		return api.DataTypeIPv6
	case MACHCLI_SQL_TYPE_STRING:
		return api.DataTypeString
	case MACHCLI_SQL_TYPE_BINARY:
		return api.DataTypeBinary
	}
}

type ColumnDesc struct {
	Name     string
	Type     SqlType
	Size     int
	Scale    int
	Nullable bool
}

type Stmt struct {
	handle       unsafe.Pointer
	conn         *Conn
	columnDescs  []ColumnDesc
	reachEOF     bool
	sqlHead      string
	rowCount     int64
	execCount    int64
	boundBuffers [][]byte // Keeps byte slices alive until execute completes
}

func (stmt *Stmt) Close() error {
	if err := machnet.CliFreeStmt(stmt.handle); err == nil {
		return nil
	} else {
		return errorWithCause(stmt, err)
	}
}

func (stmt *Stmt) Error() error {
	return errorWithCause(stmt, nil)
}

func (stmt *Stmt) execute() error {
	stmt.reachEOF = false
	if err := machnet.CliExecute(stmt.handle); err != nil {
		return errorWithCause(stmt, err)
	}
	defer func() {
		stmt.execCount++
		// Clear bound buffers after execute completes
		stmt.boundBuffers = nil
	}()
	if rowCount, err := machnet.CliRowCount(stmt.handle); err != nil {
		return errorWithCause(stmt, err)
	} else {
		stmt.rowCount = rowCount
	}
	if stmt.execCount > 0 {
		return nil
	}
	if stmt.sqlHead != "SELECT" {
		return nil
	}
	num, err := machnet.CliNumResultCol(stmt.handle)
	if err != nil {
		return errorWithCause(stmt, err)
	}
	stmt.columnDescs = make([]ColumnDesc, num)
	for i := 0; i < num; i++ {
		d := ColumnDesc{}
		if err := machnet.CliDescribeCol(stmt.handle, i, &d.Name, (*machnet.SqlType)(&d.Type), &d.Size, &d.Scale, &d.Nullable); err != nil {
			return errorWithCause(stmt, err)
		}
		stmt.columnDescs[i] = d
	}
	return nil
}

// fetch fetches the next row from the result set.
// It returns true if it reaches end of the result, otherwise false.
func (stmt *Stmt) fetch() ([]any, error) {
	if stmt.reachEOF {
		return nil, api.ErrDatabaseFetch(fmt.Errorf("reached end of the result set"))
	}
	row, end, err := machnet.CliFetchCurrent(stmt.handle)
	if err != nil {
		return nil, err
	}
	stmt.reachEOF = end
	if stmt.reachEOF {
		return nil, io.EOF
	}
	if row == nil {
		return nil, io.EOF
	}
	return row, nil
}

type Row struct {
	err      error
	values   []any
	columns  api.Columns
	rowCount int64
	stmtType machnet.StmtType
}

var _ api.Row = (*Row)(nil)

func (r *Row) Success() bool {
	return r.err == nil
}

func (r *Row) Err() error {
	return r.err
}

func (r *Row) Columns() (api.Columns, error) {
	return r.columns, nil
}

func (r *Row) Scan(dest ...any) error {
	if len(dest) > len(r.values) {
		return api.ErrParamCount(len(r.values), len(dest))
	}
	for i, d := range dest {
		if r.values[i] == nil {
			dest[i] = nil
			continue
		}
		if err := api.Scan(r.values[i], d); err != nil {
			return err
		}
	}
	return nil
}

func (r *Row) Values() []any {
	return r.values
}

func (r *Row) RowsAffected() int64 {
	return r.rowCount
}

func (r *Row) Message() string {
	if r.err != nil {
		return r.err.Error()
	}
	rows := "no rows"
	if r.rowCount == 1 {
		rows = "a row"
	} else if r.rowCount > 1 {
		p := message.NewPrinter(language.English)
		rows = p.Sprintf("%d rows", r.rowCount)
	}
	if r.stmtType.IsSelect() {
		return rows + " selected."
	} else if r.stmtType.IsInsert() {
		return rows + " inserted."
	} else if r.stmtType.IsUpdate() {
		return rows + " updated."
	} else if r.stmtType.IsDelete() {
		return rows + " deleted."
	} else if r.stmtType.IsInsertSelect() {
		return rows + " inserted from select."
	} else if r.stmtType.IsAlterSystem() {
		return "system altered."
	} else if r.stmtType.IsDDL() {
		return "ok."
	}
	return fmt.Sprintf("ok.(%d)", r.stmtType)
}

type Rows struct {
	stmt            *Stmt
	err             error
	row             []any
	rowsCount       int64
	stmtType        machnet.StmtType
	isPrepared      bool
	queryStmtPooled bool
	queryStmtKey    string
}

var _ api.Rows = (*Rows)(nil)

func (r *Rows) Err() error {
	return r.err
}

func (r *Rows) Close() error {
	if r.stmt == nil {
		return nil
	}
	stmt := r.stmt
	r.stmt = nil
	if r.isPrepared {
		stmt.reachEOF = false
		return machnet.CliExecuteClean(stmt.handle)
	}
	if r.queryStmtPooled && stmt.conn != nil {
		return stmt.conn.releaseQueryStmt(r.queryStmtKey, stmt, true)
	}
	return stmt.Close()
}

func (r *Rows) IsFetchable() bool {
	return r.stmt != nil && r.stmt.sqlHead == "SELECT"
}

func (r *Rows) Columns() (api.Columns, error) {
	if r.stmt == nil {
		return nil, nil
	}
	ret := make(api.Columns, len(r.stmt.columnDescs))
	for i, desc := range r.stmt.columnDescs {
		ret[i] = &api.Column{
			Name:     desc.Name,
			Length:   desc.Size,
			Type:     desc.Type.ColumnType(),
			DataType: desc.Type.DataType(),
		}
	}
	return ret, nil
}

func (r *Rows) definedMessage() (string, bool) {
	if r.stmt == nil {
		return "", false
	}
	switch r.stmt.sqlHead {
	case "CREATE":
		return "Created successfully.", true
	case "DROP":
		return "Dropped successfully.", true
	case "TRUNCATE":
		return "Truncated successfully.", true
	case "ALTER":
		return "Altered successfully.", true
	case "CONNECT":
		return "Connected successfully.", true
	}
	return "", false
}

func (r *Rows) Message() string {
	var verb = ""

	if r.stmtType >= 1 && r.stmtType <= 255 {
		if msg, ok := r.definedMessage(); ok {
			return msg
		}
		return "executed."
	} else if r.stmtType >= 256 && r.stmtType <= 511 {
		if msg, ok := r.definedMessage(); ok {
			return msg
		}
		return "system altered."
	} else if r.stmtType.IsSelect() {
		verb = "selected."
	} else if r.stmtType.IsInsert() {
		verb = "inserted."
	} else if r.stmtType.IsDelete() {
		verb = "deleted."
	} else if r.stmtType.IsInsertSelect() {
		verb = "inserted from select."
	} else if r.stmtType.IsUpdate() {
		verb = "updated."
	} else if r.stmtType.IsExecRollup() {
		return "rollup executed."
	} else {
		return fmt.Sprintf("executed (%d).", r.stmtType)
	}
	switch r.rowsCount {
	case 0:
		return "no rows " + verb
	case 1:
		return "a row " + verb
	default:
		p := message.NewPrinter(language.English)
		return p.Sprintf("%d rows %s", r.rowsCount, verb)
	}
}

func (r *Rows) RowsAffected() int64 {
	return r.rowsCount
}

func (r *Rows) Next() bool {
	if r.stmt == nil {
		return false
	}
	if r.stmt.reachEOF {
		return false
	}
	row, err := r.stmt.fetch()
	if err != nil {
		if err != io.EOF {
			r.err = err
		}
		return false
	}
	r.row = row
	return true
}

func (r *Rows) Row() []any {
	return r.row
}

func (r *Rows) ColumnDescriptions() []ColumnDesc {
	if r.stmt == nil {
		return nil
	}
	return r.stmt.columnDescs
}

func (r *Rows) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > len(r.row) {
		return api.ErrParamCount(len(r.row), len(dest))
	}
	for i, d := range dest {
		if d == nil {
			continue
		}
		if r.row[i] == nil {
			if !api.ScanNull(dest[i]) {
				dest[i] = nil
			}
			continue
		}
		if err := api.Scan(r.row[i], d); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) Appender(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
	db, user, table := api.TableName(tableName).SplitOr("MACHBASEDB", c.user)
	dbId := -1
	tableId := int64(-1)
	var tableType api.TableType = api.TableType(-1)
	var tableFlag api.TableFlag
	var tableColCount int

	if db != "" && db != "MACHBASEDB" {
		row := c.QueryRow(ctx, "select BACKUP_TBSID from V$STORAGE_MOUNT_DATABASES where MOUNTDB = ?", db)
		if row.Err() != nil {
			return nil, row.Err()
		}
		if err := row.Scan(&dbId); err != nil {
			return nil, err
		}
	}

	if user == "" {
		user = c.user
	}

	describeSqlText := `SELECT
			j.ID as TABLE_ID,
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG,
			j.COLCOUNT as TABLE_COLCOUNT
		from
			M$SYS_USERS u,
			M$SYS_TABLES j
		where
			u.NAME = ?
		and j.USER_ID = u.USER_ID
		and j.DATABASE_ID = ?
		and j.NAME = ?`

	r := c.QueryRow(ctx, describeSqlText, user, dbId, table)
	if r.Err() != nil {
		if r.Err() == sql.ErrNoRows {
			return nil, fmt.Errorf("table '%s' does not exist", tableName)
		}
		return nil, r.Err()
	}
	if err := r.Scan(&tableId, &tableType, &tableFlag, &tableColCount); err != nil {
		return nil, err
	}
	if tableType != api.TableTypeLog && tableType != api.TableTypeTag {
		return nil, fmt.Errorf("%s '%s' doesn't support append", tableType, tableName)
	}
	rows, err := c.Query(ctx, "select name, type, length, id, flag from M$SYS_COLUMNS where table_id = ? and database_id = ? order by id", tableId, dbId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ret := &Appender{tableName: strings.ToUpper(tableName), tableType: tableType}
	for _, opt := range opts {
		switch o := opt.(type) {
		case *api.AppenderOptionBuffer:
			ret.errCheckCount = o.Threshold
		default:
			return nil, fmt.Errorf("unknown option type-%T", o)
		}
	}
	for rows.Next() {
		col := &api.Column{}
		err = rows.Scan(&col.Name, &col.Type, &col.Length, &col.Id, &col.Flag)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(col.Name, "_") {
			if tableType != api.TableTypeLog || col.Name != "_ARRIVAL_TIME" {
				continue
			}
		}
		col.DataType = col.Type.DataType()
		ret.columns = append(ret.columns, col)
		ret.columnNames = append(ret.columnNames, col.Name)
		ret.columnTypes = append(ret.columnTypes, columnTypeToSqlType(col.Type))
	}

	stmt, err := c.NewStmt()
	if err != nil {
		return nil, err
	}
	ret.stmt = stmt

	if err := machnet.CliAppendOpen(stmt.handle, table, ret.errCheckCount); err != nil {
		err = errorWithCause(stmt, err)
		stmt.Close()
		return nil, err
	}
	return ret, nil
}

func columnTypeToSqlType(ct api.ColumnType) machnet.SqlType {
	switch ct {
	case api.ColumnTypeShort:
		return machnet.MACHCLI_SQL_TYPE_INT16
	case api.ColumnTypeUShort:
		// Bind as INT32 to avoid INT16 overflow for unsigned short values.
		return machnet.MACHCLI_SQL_TYPE_INT32
	case api.ColumnTypeInteger:
		return machnet.MACHCLI_SQL_TYPE_INT32
	case api.ColumnTypeUInteger:
		// Bind as INT64 to preserve unsigned integer range.
		return machnet.MACHCLI_SQL_TYPE_INT64
	case api.ColumnTypeLong:
		return machnet.MACHCLI_SQL_TYPE_INT64
	case api.ColumnTypeULong:
		return machnet.MACHCLI_SQL_TYPE_INT64
	case api.ColumnTypeDatetime:
		return machnet.MACHCLI_SQL_TYPE_DATETIME
	case api.ColumnTypeFloat:
		return machnet.MACHCLI_SQL_TYPE_FLOAT
	case api.ColumnTypeDouble:
		return machnet.MACHCLI_SQL_TYPE_DOUBLE
	case api.ColumnTypeIPv4:
		return machnet.MACHCLI_SQL_TYPE_IPV4
	case api.ColumnTypeIPv6:
		return machnet.MACHCLI_SQL_TYPE_IPV6
	case api.ColumnTypeVarchar:
		return machnet.MACHCLI_SQL_TYPE_STRING
	case api.ColumnTypeBinary:
		return machnet.MACHCLI_SQL_TYPE_BINARY
	default:
		return machnet.MACHCLI_SQL_TYPE_STRING
	}
}

type Appender struct {
	stmt          *Stmt
	tableName     string
	tableType     api.TableType
	errCheckCount int
	columns       api.Columns
	columnNames   []string
	columnTypes   []machnet.SqlType
	inputColumns  []AppenderInputColumn
	inputFormats  []string
	closed        bool
	successCount  int64
	failCount     int64
}

var _ api.Appender = (*Appender)(nil)
var _ api.Flusher = (*Appender)(nil)

type AppenderInputColumn struct {
	Name string
	Idx  int
}

// Close returns the number of success and fail rows.
func (a *Appender) Close() (int64, int64, error) {
	if a.closed {
		return a.successCount, a.failCount, nil
	}
	a.closed = true
	var err error

	//// even if error occurred, we should close the statement
	a.successCount, a.failCount, err = machnet.CliAppendClose(a.stmt.handle)

	if errClose := a.stmt.Close(); errClose != nil {
		return a.successCount, a.failCount, errorWithCause(a.stmt.conn, errClose)
	}
	return a.successCount, a.failCount, err
}

func (a *Appender) WithInputColumns(columns ...string) api.Appender {
	a.inputColumns = nil
	for _, col := range columns {
		a.inputColumns = append(a.inputColumns, AppenderInputColumn{Name: strings.ToUpper(col), Idx: -1})
	}
	if len(a.inputColumns) > 0 {
		for idx, col := range a.columns {
			for inIdx, inputCol := range a.inputColumns {
				if col.Name == inputCol.Name {
					a.inputColumns[inIdx].Idx = idx
				}
			}
		}
	}
	return a
}

func (a *Appender) WithInputFormats(formats ...string) api.Appender {
	a.inputFormats = formats
	return a
}

func (a *Appender) TableName() string {
	return a.tableName
}

func (a *Appender) TableType() api.TableType {
	return a.tableType
}

func (a *Appender) Columns() (api.Columns, error) {
	return a.columns, nil
}

func (a *Appender) Flush() error {
	if err := machnet.CliAppendFlush(a.stmt.handle); err == nil {
		return nil
	} else {
		return errorWithCause(a.stmt, err)
	}
}

func (a *Appender) Append(values ...any) error {
	switch a.tableType {
	case api.TableTypeTag:
		return a.append(values...)
	case api.TableTypeLog:
		var valuesWithTime []any
		if len(values) == len(a.columns) {
			valuesWithTime = values
		} else {
			valuesWithTime = append([]any{time.Time{}}, values...)
		}
		return a.append(valuesWithTime...)
	default:
		return fmt.Errorf("%s can not be appended", a.tableName)
	}
}

func (a *Appender) AppendLogTime(ts time.Time, values ...any) error {
	if a.tableType != api.TableTypeLog {
		return fmt.Errorf("%s is not a log table, use Append() instead", a.tableName)
	}
	values = append([]any{ts}, values...)
	return a.append(values...)
}

func (a *Appender) append(values ...any) error {
	if len(a.columns) == 0 {
		return api.ErrDatabaseNoColumns(a.tableName)
	}
	if len(a.inputColumns) > 0 {
		if len(a.inputColumns) != len(values) {
			return api.ErrDatabaseLengthOfColumns(a.tableName, len(a.columns), len(values))
		}
		newValues := make([]any, len(a.columns))
		for i, inputCol := range a.inputColumns {
			newValues[inputCol.Idx] = values[i]
		}
		values = newValues
	} else {
		if len(a.columns) != len(values) {
			return api.ErrDatabaseLengthOfColumns(a.tableName, len(a.columns), len(values))
		}
	}
	if a.closed {
		return api.ErrDatabaseClosedAppender
	}
	if a.stmt == nil || a.stmt.conn == nil {
		return api.ErrDatabaseNoConnection
	}

	if err := machnet.CliAppendData(a.stmt.handle, a.columnTypes, a.columnNames, values, a.inputFormats); err != nil {
		return errorWithCause(a.stmt, err)
	}
	return nil
}
