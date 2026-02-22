package machnet

import (
	"sync"
	"time"
)

type EnvHandle struct {
	mu      sync.Mutex
	closed  bool
	lastErr StatusError
	conns   map[*ConnHandle]struct{}
}

// Error() returns the last error code and message for the environment handle.
func (env *EnvHandle) Error() (int, string) {
	return env.lastErr.code, env.lastErr.msg
}

type ConnHandle struct {
	mu      sync.Mutex
	env     *EnvHandle
	native  *NativeConn
	closed  bool
	lastErr StatusError
}

// Error() returns the last error code and message for the connection handle.
func (conn *ConnHandle) Error() (int, string) {
	return conn.lastErr.code, conn.lastErr.msg
}

type AppendState struct {
	table         string
	errCheckCnt   int
	columns       []ColumnMeta
	bindings      AppendBindings
	bindingsReady bool
	addCount      int64
	sentCount     int64
	successCnt    int64
	failCnt       int64
	pendingRows   [][]byte
	pendingBytes  int
	firstQueuedAt time.Time

	appendBatchMaxRows  int
	appendBatchMaxBytes int
	appendBatchMaxDelay time.Duration
}

type StmtHandle struct {
	mu        sync.Mutex
	conn      *ConnHandle
	id        uint32
	closed    bool
	prepared  bool
	sql       string
	stmtType  StmtType
	rowCount  int64
	columns   []ColumnMeta
	paramDesc []ParamDesc
	rows      [][]any
	rowPos    int
	fetchLast bool
	fetchSize int64
	bound     map[int]BoundParam
	app       *AppendState
	lastErr   StatusError
}

// Error() returns the last error code and message for the statement handle.
func (stmt *StmtHandle) Error() (int, string) {
	return stmt.lastErr.code, stmt.lastErr.msg
}

func Initialize() (*EnvHandle, error) {
	return &EnvHandle{conns: map[*ConnHandle]struct{}{}}, nil
}

func (env *EnvHandle) Finalize() error {
	if env == nil {
		return nil
	}
	env.mu.Lock()
	if env.closed {
		env.mu.Unlock()
		return nil
	}
	env.closed = true
	conns := make([]*ConnHandle, 0, len(env.conns))
	for c := range env.conns {
		conns = append(conns, c)
	}
	env.mu.Unlock()
	for _, c := range conns {
		_ = c.Disconnect()
	}
	return nil
}

func (env *EnvHandle) Connect(connStr string) (*ConnHandle, error) {
	if env == nil {
		return nil, makeClientErr("invalid environment")
	}
	host, port, user, pass, alts, fetchRows, err := parseConnString(connStr)
	if err != nil {
		env.lastErr.setErr(err)
		return nil, err
	}
	nc, err := dialNative(host, port, user, pass, alts, fetchRows)
	if err != nil {
		env.lastErr.setErr(err)
		return nil, err
	}
	ch := &ConnHandle{env: env, native: nc}
	env.mu.Lock()
	if env.closed {
		env.mu.Unlock()
		_ = nc.close()
		err = makeClientErr("environment closed")
		env.lastErr.setErr(err)
		return nil, err
	}
	env.conns[ch] = struct{}{}
	env.mu.Unlock()
	env.lastErr.setErr(nil)
	return ch, nil
}

func (conn *ConnHandle) Disconnect() error {
	if conn == nil {
		return nil
	}
	conn.mu.Lock()
	if conn.closed {
		conn.mu.Unlock()
		return nil
	}
	conn.closed = true
	nc := conn.native
	conn.mu.Unlock()

	err := nc.close()
	conn.lastErr.setErr(err)
	if conn.env != nil {
		conn.env.mu.Lock()
		delete(conn.env.conns, conn)
		conn.env.mu.Unlock()
	}
	return err
}

func (conn *ConnHandle) AllocStmt() (*StmtHandle, error) {
	if conn == nil {
		return nil, makeClientErr("invalid connection")
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.closed || conn.native == nil {
		err := makeClientErr("connection closed")
		conn.lastErr.setErr(err)
		return nil, err
	}
	id, err := conn.native.nextStmtID()
	if err != nil {
		conn.lastErr.setErr(err)
		return nil, err
	}
	stmt := &StmtHandle{
		conn:      conn,
		id:        id,
		fetchLast: true,
		fetchSize: conn.native.fetchRows,
		bound:     map[int]BoundParam{},
	}
	if stmt.fetchSize <= 0 {
		stmt.fetchSize = defaultFetchRows
	}
	conn.lastErr.setErr(nil)
	return stmt, nil
}

func (stmt *StmtHandle) Free() error {
	if stmt == nil {
		return nil
	}
	stmt.mu.Lock()
	if stmt.closed {
		stmt.mu.Unlock()
		return nil
	}
	stmt.closed = true
	conn := stmt.conn
	id := stmt.id
	stmt.mu.Unlock()
	var err error
	if conn != nil && conn.native != nil {
		err = conn.native.free(id)
		if err == nil {
			conn.native.releaseStmtID(id)
		}
	}
	stmt.lastErr.setErr(err)
	if conn != nil {
		conn.lastErr.setErr(err)
	}
	return err
}

func (stmt *StmtHandle) Prepare(query string) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if stmt.closed || stmt.conn == nil || stmt.conn.native == nil {
		err := makeClientErr("statement closed")
		stmt.lastErr.setErr(err)
		return err
	}
	res, err := stmt.conn.native.prepare(stmt.id, query)
	stmt.lastErr.setErr(err)
	stmt.conn.lastErr.setErr(err)
	if err != nil {
		return err
	}
	stmt.prepared = true
	stmt.sql = query
	stmt.columns = res.columns
	stmt.paramDesc = res.paramDesc
	stmt.stmtType = res.stmtType
	stmt.rowCount = res.rowCount
	stmt.rows = nil
	stmt.rowPos = 0
	stmt.fetchLast = true
	return nil
}

func (stmt *StmtHandle) Execute() error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if stmt.closed || stmt.conn == nil || stmt.conn.native == nil {
		err := makeClientErr("statement closed")
		stmt.lastErr.setErr(err)
		return err
	}
	if !stmt.prepared {
		err := makeClientErr("statement is not prepared")
		stmt.lastErr.setErr(err)
		return err
	}
	paramCnt := len(stmt.paramDesc)
	if paramCnt == 0 {
		for idx := range stmt.bound {
			if idx+1 > paramCnt {
				paramCnt = idx + 1
			}
		}
	}
	params := make([]BoundParam, paramCnt)
	for i := 0; i < paramCnt; i++ {
		if b, ok := stmt.bound[i]; ok {
			params[i] = b
		} else {
			params[i] = BoundParam{sqlType: MACHCLI_SQL_TYPE_STRING, isNull: true}
		}
	}
	res, err := stmt.conn.native.executePrepared(stmt.id, stmt.sql, params, stmt.columns)
	stmt.lastErr.setErr(err)
	stmt.conn.lastErr.setErr(err)
	if err != nil {
		return err
	}
	stmt.stmtType = res.stmtType
	stmt.rowCount = res.rowCount
	if len(res.columns) > 0 {
		stmt.columns = res.columns
	}
	stmt.rows = res.rows
	stmt.rowPos = 0
	stmt.fetchLast = res.lastResult || len(stmt.columns) == 0
	if stmt.fetchSize <= 0 {
		stmt.fetchSize = defaultFetchRows
	}
	return nil
}

func (stmt *StmtHandle) ExecDirect(query string) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if stmt.closed || stmt.conn == nil || stmt.conn.native == nil {
		err := makeClientErr("statement closed")
		stmt.lastErr.setErr(err)
		return err
	}
	res, err := stmt.conn.native.execDirect(stmt.id, query)
	stmt.lastErr.setErr(err)
	stmt.conn.lastErr.setErr(err)
	if err != nil {
		return err
	}
	stmt.prepared = false
	stmt.sql = query
	stmt.stmtType = res.stmtType
	stmt.rowCount = res.rowCount
	stmt.columns = res.columns
	stmt.paramDesc = res.paramDesc
	stmt.rows = res.rows
	stmt.rowPos = 0
	stmt.fetchLast = res.lastResult || len(stmt.columns) == 0
	if stmt.fetchSize <= 0 {
		stmt.fetchSize = defaultFetchRows
	}
	return nil
}

func (stmt *StmtHandle) ExecuteClean() error {
	if stmt == nil {
		return nil
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	stmt.rows = nil
	stmt.rowPos = 0
	stmt.fetchLast = true
	return nil
}

func (stmt *StmtHandle) NumParam() (int, error) {
	if stmt == nil {
		return 0, makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	return len(stmt.paramDesc), nil
}

func (stmt *StmtHandle) DescribeParam(paramNo int) (ParamDesc, error) {
	if stmt == nil {
		return ParamDesc{}, makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if paramNo < 0 || paramNo >= len(stmt.paramDesc) {
		return ParamDesc{Type: MACHCLI_SQL_TYPE_STRING, Nullable: true}, makeClientErr("invalid parameter index")
	}
	return stmt.paramDesc[paramNo], nil
}

func (stmt *StmtHandle) BindParam(paramNo int, sqlType SqlType, value any) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if paramNo < 0 {
		err := makeClientErr("invalid parameter index")
		stmt.lastErr.setErr(err)
		return err
	}
	bp := BoundParam{sqlType: sqlType, value: value, isNull: value == nil}
	stmt.bound[paramNo] = bp
	stmt.lastErr.setErr(nil)
	return nil
}

func (stmt *StmtHandle) RowCount() (int64, error) {
	if stmt == nil {
		return 0, makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	return stmt.rowCount, nil
}

func (stmt *StmtHandle) GetStmtType() (StmtType, error) {
	if stmt == nil {
		return 0, makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if stmt.stmtType == 0 {
		return inferStmtType(stmt.sql), nil
	}
	return stmt.stmtType, nil
}

func (stmt *StmtHandle) NumResultCol() (int, error) {
	if stmt == nil {
		return 0, makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	return len(stmt.columns), nil
}

func (stmt *StmtHandle) DescribeCol(columnNo int, pName *string, pType *SqlType, pSize *int, pScale *int, pNullable *bool) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if columnNo < 0 || columnNo >= len(stmt.columns) {
		err := makeClientErr("invalid column index")
		stmt.lastErr.setErr(err)
		return err
	}
	col := stmt.columns[columnNo]
	if pName != nil {
		*pName = col.name
	}
	if pType != nil {
		*pType = col.sqlType
	}
	if pSize != nil {
		sz := col.length
		if col.isVariable {
			if sz <= 0 {
				sz = col.precision
			}
			if sz > 65536 {
				sz = 65536
			}
		}
		*pSize = sz
	}
	if pScale != nil {
		*pScale = col.scale
	}
	if pNullable != nil {
		*pNullable = col.nullable
	}
	return nil
}

func (stmt *StmtHandle) Fetch() ([]any, error) {
	if stmt == nil {
		return nil, makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	for {
		if stmt.rowPos < len(stmt.rows) {
			ret := stmt.rows[stmt.rowPos]
			stmt.rowPos++
			if stmt.rowPos >= len(stmt.rows) {
				stmt.rows = nil
				stmt.rowPos = 0
			}
			return ret, nil
		}

		if stmt.fetchLast || len(stmt.columns) == 0 {
			return nil, nil
		}
		if stmt.conn == nil || stmt.conn.native == nil {
			return nil, nil
		}

		rows, last, err := stmt.conn.native.fetchRowsChunk(stmt.id, stmt.columns, stmt.fetchSize)
		stmt.fetchLast = last
		stmt.lastErr.setErr(err)
		stmt.conn.lastErr.setErr(err)
		if err != nil {
			return nil, err
		}
		stmt.rows = rows
		stmt.rowPos = 0
		if len(stmt.rows) == 0 && stmt.fetchLast {
			return nil, nil
		}
	}
}

func (stmt *StmtHandle) AppendOpen(tableName string, errCheckCount int) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if stmt.closed || stmt.conn == nil || stmt.conn.native == nil {
		err := makeClientErr("statement closed")
		stmt.lastErr.setErr(err)
		return err
	}
	res, err := stmt.conn.native.appendOpen(stmt.id, tableName, errCheckCount)
	stmt.lastErr.setErr(err)
	stmt.conn.lastErr.setErr(err)
	if err != nil {
		return err
	}

	stmt.app = &AppendState{
		table:       tableName,
		errCheckCnt: errCheckCount,
		columns:     append([]ColumnMeta(nil), res.columns...),
		bindings: AppendBindings{
			arrivalArg: -1,
		},
		appendBatchMaxRows:  512,
		appendBatchMaxBytes: 512 * 1024,
		appendBatchMaxDelay: 5 * time.Millisecond,
	}
	stmt.app.pendingRows = make([][]byte, 0, stmt.app.appendBatchMaxRows)
	return nil
}

func (stmt *StmtHandle) flushAppendBufferedLocked(checkResponse bool) error {
	if stmt == nil || stmt.app == nil || len(stmt.app.pendingRows) == 0 {
		return nil
	}
	pending := stmt.app.pendingRows
	pendingCount := len(pending)
	stmt.app.pendingRows = stmt.app.pendingRows[:0]
	stmt.app.pendingBytes = 0
	stmt.app.firstQueuedAt = time.Time{}

	err := stmt.conn.native.appendData(stmt.id, pending, checkResponse)
	stmt.lastErr.setErr(err)
	stmt.conn.lastErr.setErr(err)
	if err != nil {
		stmt.app.failCnt += int64(pendingCount)
		return err
	}
	stmt.app.sentCount += int64(pendingCount)
	return nil
}

func (stmt *StmtHandle) AppendData(types []SqlType, names []string, args []any, formats []string) error {
	_ = types
	_ = formats
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if stmt.closed || stmt.conn == nil || stmt.conn.native == nil {
		err := makeClientErr("statement closed")
		stmt.lastErr.setErr(err)
		return err
	}
	if stmt.app == nil {
		err := makeClientErr("append not opened")
		stmt.lastErr.setErr(err)
		return err
	}
	if len(names) != len(args) {
		err := makeClientErr("append argument mismatch")
		stmt.lastErr.setErr(err)
		return err
	}
	if !stmt.app.bindingsReady {
		bindings, err := buildAppendBindings(stmt.app.columns, names)
		if err != nil {
			stmt.lastErr.setErr(err)
			stmt.conn.lastErr.setErr(err)
			stmt.app.failCnt++
			return err
		}
		stmt.app.bindings = bindings
		stmt.app.bindingsReady = true
	}
	rowPayload, err := encodeAppendRow(stmt.app.columns, stmt.app.bindings, args, stmt.conn.native.serverEndian)
	if err != nil {
		stmt.lastErr.setErr(err)
		stmt.conn.lastErr.setErr(err)
		stmt.app.failCnt++
		return err
	}
	stmt.app.addCount++
	if len(stmt.app.pendingRows) == 0 {
		stmt.app.firstQueuedAt = time.Now()
	}
	stmt.app.pendingRows = append(stmt.app.pendingRows, rowPayload)
	stmt.app.pendingBytes += len(rowPayload)

	checkResponse := stmt.app.errCheckCnt > 0 && (stmt.app.addCount%int64(stmt.app.errCheckCnt) == 0)
	flushNow := checkResponse ||
		len(stmt.app.pendingRows) >= stmt.app.appendBatchMaxRows ||
		stmt.app.pendingBytes >= stmt.app.appendBatchMaxBytes
	if !flushNow && !stmt.app.firstQueuedAt.IsZero() && time.Since(stmt.app.firstQueuedAt) >= stmt.app.appendBatchMaxDelay {
		flushNow = true
	}
	if flushNow {
		if err := stmt.flushAppendBufferedLocked(checkResponse); err != nil {
			return err
		}
	}
	return nil
}

func (stmt *StmtHandle) AppendClose() (int64, int64, error) {
	if stmt == nil {
		return 0, 0, makeClientErr("invalid statement")
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if stmt.app == nil {
		return 0, 0, nil
	}
	if err := stmt.flushAppendBufferedLocked(false); err != nil {
		localSuccess := stmt.app.sentCount - stmt.app.failCnt
		if localSuccess < 0 {
			localSuccess = 0
		}
		return localSuccess, stmt.app.failCnt, err
	}

	success, fail, err := stmt.conn.native.appendClose(stmt.id)
	stmt.lastErr.setErr(err)
	stmt.conn.lastErr.setErr(err)
	if err != nil {
		localSuccess := stmt.app.sentCount - stmt.app.failCnt
		if localSuccess < 0 {
			localSuccess = 0
		}
		return localSuccess, stmt.app.failCnt, err
	}
	stmt.app.successCnt = success
	stmt.app.failCnt = fail
	stmt.app = nil
	return success, fail, nil
}

func (stmt *StmtHandle) AppendFlush() error {
	if stmt == nil {
		return nil
	}
	stmt.mu.Lock()
	defer stmt.mu.Unlock()
	if stmt.app == nil || stmt.conn == nil || stmt.conn.native == nil {
		return nil
	}
	return stmt.flushAppendBufferedLocked(false)
}
