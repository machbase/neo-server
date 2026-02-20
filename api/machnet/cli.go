package machnet

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
	"unsafe"
)

type envHandle struct {
	mu      sync.Mutex
	closed  bool
	lastErr cliErrorState
	conns   map[*connHandle]struct{}
}

type connHandle struct {
	mu      sync.Mutex
	env     *envHandle
	native  *nativeConn
	closed  bool
	lastErr cliErrorState
}

type appendState struct {
	table         string
	errCheckCnt   int
	columns       []columnMeta
	bindings      appendBindings
	bindingsReady bool
	addCount      int64
	sentCount     int64
	successCnt    int64
	failCnt       int64
	pendingRows   [][]byte
	pendingBytes  int
	firstQueuedAt time.Time
}

const (
	appendBatchMaxRows  = 512
	appendBatchMaxBytes = 512 * 1024
	appendBatchMaxDelay = 5 * time.Millisecond
)

type stmtHandle struct {
	mu        sync.Mutex
	conn      *connHandle
	id        uint32
	closed    bool
	prepared  bool
	sql       string
	stmtType  int
	rowCount  int64
	columns   []columnMeta
	paramDesc []CliParamDesc
	rows      [][]any
	rowPos    int
	current   []any
	bound     map[int]boundParam
	app       *appendState
	lastErr   cliErrorState
}

func setErr(st *cliErrorState, err error) {
	if st == nil {
		return
	}
	if err == nil {
		st.code = 0
		st.msg = ""
		return
	}
	var se *statusError
	if errors.As(err, &se) {
		st.code = se.code
		st.msg = se.msg
		if st.msg == "" {
			st.msg = err.Error()
		}
		return
	}
	st.code = 0
	st.msg = err.Error()
}

func CliInitialize(env *unsafe.Pointer) error {
	e := &envHandle{conns: map[*connHandle]struct{}{}}
	*env = unsafe.Pointer(e)
	return nil
}

func CliFinalize(env unsafe.Pointer) error {
	if env == nil {
		return nil
	}
	e := (*envHandle)(env)
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	conns := make([]*connHandle, 0, len(e.conns))
	for c := range e.conns {
		conns = append(conns, c)
	}
	e.mu.Unlock()
	for _, c := range conns {
		_ = CliDisconnect(unsafe.Pointer(c))
	}
	return nil
}

func CliConnect(env unsafe.Pointer, connStr string, conn *unsafe.Pointer) error {
	if env == nil {
		return makeClientErr("invalid environment")
	}
	e := (*envHandle)(env)
	host, port, user, pass, alts, err := parseConnString(connStr)
	if err != nil {
		setErr(&e.lastErr, err)
		return err
	}
	nc, err := dialNative(host, port, user, pass, alts)
	if err != nil {
		setErr(&e.lastErr, err)
		return err
	}
	ch := &connHandle{env: e, native: nc}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		_ = nc.close()
		err = makeClientErr("environment closed")
		setErr(&e.lastErr, err)
		return err
	}
	e.conns[ch] = struct{}{}
	e.mu.Unlock()
	*conn = unsafe.Pointer(ch)
	setErr(&e.lastErr, nil)
	return nil
}

func CliDisconnect(conn unsafe.Pointer) error {
	if conn == nil {
		return nil
	}
	ch := (*connHandle)(conn)
	ch.mu.Lock()
	if ch.closed {
		ch.mu.Unlock()
		return nil
	}
	ch.closed = true
	nc := ch.native
	ch.mu.Unlock()

	err := nc.close()
	setErr(&ch.lastErr, err)
	if ch.env != nil {
		ch.env.mu.Lock()
		delete(ch.env.conns, ch)
		ch.env.mu.Unlock()
	}
	return err
}

func CliError(handle unsafe.Pointer, handleType HandleType, code *int, msg *string) error {
	if code == nil || msg == nil {
		return makeClientErr("invalid CliError output")
	}
	*code = 0
	*msg = ""
	if handle == nil {
		return nil
	}
	switch handleType {
	case MACHCLI_HANDLE_ENV:
		e := (*envHandle)(handle)
		*code = e.lastErr.code
		*msg = e.lastErr.msg
	case MACHCLI_HANDLE_DBC:
		c := (*connHandle)(handle)
		*code = c.lastErr.code
		*msg = c.lastErr.msg
	case MACHCLI_HANDLE_STMT:
		s := (*stmtHandle)(handle)
		*code = s.lastErr.code
		*msg = s.lastErr.msg
	default:
		return makeClientErr("unknown handle type")
	}
	return nil
}

func CliAllocStmt(conn unsafe.Pointer, stmt *unsafe.Pointer) error {
	if conn == nil {
		return makeClientErr("invalid connection")
	}
	ch := (*connHandle)(conn)
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if ch.closed || ch.native == nil {
		err := makeClientErr("connection closed")
		setErr(&ch.lastErr, err)
		return err
	}
	id, err := ch.native.nextStmtID()
	if err != nil {
		setErr(&ch.lastErr, err)
		return err
	}
	s := &stmtHandle{
		conn:  ch,
		id:    id,
		bound: map[int]boundParam{},
	}
	*stmt = unsafe.Pointer(s)
	setErr(&ch.lastErr, nil)
	return nil
}

func CliFreeStmt(stmt unsafe.Pointer) error {
	if stmt == nil {
		return nil
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	conn := s.conn
	id := s.id
	s.mu.Unlock()
	var err error
	if conn != nil && conn.native != nil {
		err = conn.native.free(id)
		if err == nil {
			conn.native.releaseStmtID(id)
		}
	}
	setErr(&s.lastErr, err)
	if conn != nil {
		setErr(&conn.lastErr, err)
	}
	return err
}

func CliPrepare(stmt unsafe.Pointer, query string) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.conn == nil || s.conn.native == nil {
		err := makeClientErr("statement closed")
		setErr(&s.lastErr, err)
		return err
	}
	res, err := s.conn.native.prepare(s.id, query)
	setErr(&s.lastErr, err)
	setErr(&s.conn.lastErr, err)
	if err != nil {
		return err
	}
	s.prepared = true
	s.sql = query
	s.columns = res.columns
	s.paramDesc = res.paramDescs
	s.stmtType = res.stmtType
	s.rowCount = res.rowCount
	s.rows = nil
	s.rowPos = 0
	s.current = nil
	return nil
}

func CliExecute(stmt unsafe.Pointer) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.conn == nil || s.conn.native == nil {
		err := makeClientErr("statement closed")
		setErr(&s.lastErr, err)
		return err
	}
	if !s.prepared {
		err := makeClientErr("statement is not prepared")
		setErr(&s.lastErr, err)
		return err
	}
	paramCnt := len(s.paramDesc)
	if paramCnt == 0 {
		for idx := range s.bound {
			if idx+1 > paramCnt {
				paramCnt = idx + 1
			}
		}
	}
	params := make([]boundParam, paramCnt)
	for i := 0; i < paramCnt; i++ {
		if b, ok := s.bound[i]; ok {
			params[i] = b
		} else {
			params[i] = boundParam{sqlType: MACHCLI_SQL_TYPE_STRING, isNull: true}
		}
	}
	res, err := s.conn.native.executePrepared(s.id, s.sql, params, s.columns)
	setErr(&s.lastErr, err)
	setErr(&s.conn.lastErr, err)
	if err != nil {
		return err
	}
	s.stmtType = res.stmtType
	s.rowCount = res.rowCount
	if len(res.columns) > 0 {
		s.columns = res.columns
	}
	s.rows = res.rows
	s.rowPos = 0
	s.current = nil
	return nil
}

func CliExecDirect(stmt unsafe.Pointer, query string) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.conn == nil || s.conn.native == nil {
		err := makeClientErr("statement closed")
		setErr(&s.lastErr, err)
		return err
	}
	res, err := s.conn.native.execDirect(s.id, query)
	setErr(&s.lastErr, err)
	setErr(&s.conn.lastErr, err)
	if err != nil {
		return err
	}
	s.prepared = false
	s.sql = query
	s.stmtType = res.stmtType
	s.rowCount = res.rowCount
	s.columns = res.columns
	s.paramDesc = res.paramDescs
	s.rows = res.rows
	s.rowPos = 0
	s.current = nil
	return nil
}

func CliExecuteClean(stmt unsafe.Pointer) error {
	if stmt == nil {
		return nil
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = nil
	s.rowPos = 0
	s.current = nil
	return nil
}

func CliNumParam(stmt unsafe.Pointer) (int, error) {
	if stmt == nil {
		return 0, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.paramDesc), nil
}

func CliDescribeParam(stmt unsafe.Pointer, paramNo int) (CliParamDesc, error) {
	if stmt == nil {
		return CliParamDesc{}, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if paramNo < 0 || paramNo >= len(s.paramDesc) {
		return CliParamDesc{Type: MACHCLI_SQL_TYPE_STRING, Nullable: true}, makeClientErr("invalid parameter index")
	}
	return s.paramDesc[paramNo], nil
}

func CliBindParam(stmt unsafe.Pointer, paramNo int, cType CType, sqlType SqlType, value unsafe.Pointer, valueLen int) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if paramNo < 0 {
		err := makeClientErr("invalid parameter index")
		setErr(&s.lastErr, err)
		return err
	}
	bp := boundParam{sqlType: sqlType}
	if value == nil {
		bp.isNull = true
		s.bound[paramNo] = bp
		setErr(&s.lastErr, nil)
		return nil
	}
	switch cType {
	case MACHCLI_C_TYPE_INT16:
		bp.value = *(*int16)(value)
	case MACHCLI_C_TYPE_INT32:
		bp.value = *(*int32)(value)
	case MACHCLI_C_TYPE_INT64:
		bp.value = *(*int64)(value)
	case MACHCLI_C_TYPE_FLOAT:
		bp.value = *(*float32)(value)
	case MACHCLI_C_TYPE_DOUBLE:
		bp.value = *(*float64)(value)
	case MACHCLI_C_TYPE_CHAR:
		data := make([]byte, valueLen)
		copy(data, unsafe.Slice((*byte)(value), valueLen))
		switch sqlType {
		case MACHCLI_SQL_TYPE_BINARY:
			bp.value = data
		case MACHCLI_SQL_TYPE_IPV4, MACHCLI_SQL_TYPE_IPV6:
			bp.value = string(data)
		default:
			bp.value = string(data)
		}
	default:
		err := makeClientErr("unsupported c type")
		setErr(&s.lastErr, err)
		return err
	}
	s.bound[paramNo] = bp
	setErr(&s.lastErr, nil)
	return nil
}

func CliRowCount(stmt unsafe.Pointer) (int64, error) {
	if stmt == nil {
		return 0, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rowCount, nil
}

func CliGetStmtType(stmt unsafe.Pointer) (int, error) {
	if stmt == nil {
		return 0, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stmtType == 0 {
		return inferStmtType(s.sql), nil
	}
	return s.stmtType, nil
}

func CliNumResultCol(stmt unsafe.Pointer) (int, error) {
	if stmt == nil {
		return 0, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.columns), nil
}

func CliDescribeCol(stmt unsafe.Pointer, columnNo int, pName *string, pType *SqlType, pSize *int, pScale *int, pNullable *bool) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if columnNo < 0 || columnNo >= len(s.columns) {
		err := makeClientErr("invalid column index")
		setErr(&s.lastErr, err)
		return err
	}
	col := s.columns[columnNo]
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

func CliFetch(stmt unsafe.Pointer) (bool, error) {
	if stmt == nil {
		return true, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rowPos >= len(s.rows) {
		s.current = nil
		return true, nil
	}
	s.current = s.rows[s.rowPos]
	s.rowPos++
	return false, nil
}

func CliFetchCurrent(stmt unsafe.Pointer) ([]any, bool, error) {
	if stmt == nil {
		return nil, true, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rowPos >= len(s.rows) {
		s.current = nil
		return nil, true, nil
	}
	row := s.rows[s.rowPos]
	s.rowPos++
	s.current = row
	return row, false, nil
}

func CliCurrentRow(stmt unsafe.Pointer) ([]any, error) {
	if stmt == nil {
		return nil, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return nil, nil
	}
	return s.current, nil
}

func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case int:
		return int64(x), true
	case uint:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		return int64(x), true
	case float32:
		return int64(x), true
	case float64:
		return int64(x), true
	default:
		return 0, false
	}
}

func asFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float32:
		return float64(x), true
	case float64:
		return x, true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	default:
		return 0, false
	}
}

func CliGetData(stmt unsafe.Pointer, columnNo int, cType CType, buf unsafe.Pointer, bufLen int) (int64, error) {
	if stmt == nil {
		return 0, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return -1, nil
	}
	if columnNo < 0 || columnNo >= len(s.current) {
		err := makeClientErr("invalid column index")
		setErr(&s.lastErr, err)
		return 0, err
	}
	v := s.current[columnNo]
	if v == nil {
		return -1, nil
	}
	if buf == nil {
		return 0, makeClientErr("invalid output buffer")
	}

	switch cType {
	case MACHCLI_C_TYPE_INT16:
		n, ok := asInt64(v)
		if !ok {
			return 0, makeClientErr("cannot convert value to int16")
		}
		*(*int16)(buf) = int16(n)
		return 2, nil
	case MACHCLI_C_TYPE_INT32:
		n, ok := asInt64(v)
		if !ok {
			return 0, makeClientErr("cannot convert value to int32")
		}
		*(*int32)(buf) = int32(n)
		return 4, nil
	case MACHCLI_C_TYPE_INT64:
		switch x := v.(type) {
		case time.Time:
			*(*int64)(buf) = x.UnixNano()
			return 8, nil
		default:
			n, ok := asInt64(v)
			if !ok {
				return 0, makeClientErr("cannot convert value to int64")
			}
			*(*int64)(buf) = n
			return 8, nil
		}
	case MACHCLI_C_TYPE_FLOAT:
		f, ok := asFloat64(v)
		if !ok {
			return 0, makeClientErr("cannot convert value to float")
		}
		*(*float32)(buf) = float32(f)
		return 4, nil
	case MACHCLI_C_TYPE_DOUBLE:
		f, ok := asFloat64(v)
		if !ok {
			return 0, makeClientErr("cannot convert value to double")
		}
		*(*float64)(buf) = f
		return 8, nil
	case MACHCLI_C_TYPE_CHAR:
		var data []byte
		switch x := v.(type) {
		case string:
			data = []byte(x)
		case []byte:
			data = x
		case net.IP:
			if ip4 := x.To4(); ip4 != nil {
				data = ip4
			} else if ip16 := x.To16(); ip16 != nil {
				data = ip16
			}
		default:
			data = []byte(fmt.Sprint(x))
		}
		if len(data) == 0 {
			return -1, nil
		}
		if bufLen > 0 {
			copy(unsafe.Slice((*byte)(buf), bufLen), data)
		}
		return int64(len(data)), nil
	default:
		return 0, makeClientErr("unsupported c type")
	}
}

func CliAppendOpen(stmt unsafe.Pointer, tableName string, errCheckCount int) error {
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.conn == nil || s.conn.native == nil {
		err := makeClientErr("statement closed")
		setErr(&s.lastErr, err)
		return err
	}
	res, err := s.conn.native.appendOpen(s.id, tableName, errCheckCount)
	setErr(&s.lastErr, err)
	setErr(&s.conn.lastErr, err)
	if err != nil {
		return err
	}
	s.app = &appendState{
		table:       tableName,
		errCheckCnt: errCheckCount,
		columns:     append([]columnMeta(nil), res.columns...),
		bindings: appendBindings{
			arrivalArg: -1,
		},
		pendingRows: make([][]byte, 0, appendBatchMaxRows),
	}
	return nil
}

func flushAppendBufferedLocked(s *stmtHandle, checkResponse bool) error {
	if s == nil || s.app == nil || len(s.app.pendingRows) == 0 {
		return nil
	}
	pending := s.app.pendingRows
	pendingCount := len(pending)
	s.app.pendingRows = s.app.pendingRows[:0]
	s.app.pendingBytes = 0
	s.app.firstQueuedAt = time.Time{}

	err := s.conn.native.appendData(s.id, pending, checkResponse)
	setErr(&s.lastErr, err)
	setErr(&s.conn.lastErr, err)
	if err != nil {
		s.app.failCnt += int64(pendingCount)
		return err
	}
	s.app.sentCount += int64(pendingCount)
	return nil
}

func CliAppendData(stmt unsafe.Pointer, types []SqlType, names []string, args []any, formats []string) error {
	_ = types
	_ = formats
	if stmt == nil {
		return makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.conn == nil || s.conn.native == nil {
		err := makeClientErr("statement closed")
		setErr(&s.lastErr, err)
		return err
	}
	if s.app == nil {
		err := makeClientErr("append not opened")
		setErr(&s.lastErr, err)
		return err
	}
	if len(names) != len(args) {
		err := makeClientErr("append argument mismatch")
		setErr(&s.lastErr, err)
		return err
	}
	if !s.app.bindingsReady {
		bindings, err := buildAppendBindings(s.app.columns, names)
		if err != nil {
			setErr(&s.lastErr, err)
			setErr(&s.conn.lastErr, err)
			s.app.failCnt++
			return err
		}
		s.app.bindings = bindings
		s.app.bindingsReady = true
	}
	rowPayload, err := encodeAppendRow(s.app.columns, s.app.bindings, args, s.conn.native.serverEndian)
	if err != nil {
		setErr(&s.lastErr, err)
		setErr(&s.conn.lastErr, err)
		s.app.failCnt++
		return err
	}
	s.app.addCount++
	if len(s.app.pendingRows) == 0 {
		s.app.firstQueuedAt = time.Now()
	}
	s.app.pendingRows = append(s.app.pendingRows, rowPayload)
	s.app.pendingBytes += len(rowPayload)

	checkResponse := s.app.errCheckCnt > 0 && (s.app.addCount%int64(s.app.errCheckCnt) == 0)
	flushNow := checkResponse ||
		len(s.app.pendingRows) >= appendBatchMaxRows ||
		s.app.pendingBytes >= appendBatchMaxBytes
	if !flushNow && !s.app.firstQueuedAt.IsZero() && time.Since(s.app.firstQueuedAt) >= appendBatchMaxDelay {
		flushNow = true
	}
	if flushNow {
		if err := flushAppendBufferedLocked(s, checkResponse); err != nil {
			return err
		}
	}
	return nil
}

func CliAppendClose(stmt unsafe.Pointer) (int64, int64, error) {
	if stmt == nil {
		return 0, 0, makeClientErr("invalid statement")
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil {
		return 0, 0, nil
	}
	if err := flushAppendBufferedLocked(s, false); err != nil {
		localSucc := s.app.sentCount - s.app.failCnt
		if localSucc < 0 {
			localSucc = 0
		}
		return localSucc, s.app.failCnt, err
	}

	succ, fail, err := s.conn.native.appendClose(s.id)
	setErr(&s.lastErr, err)
	setErr(&s.conn.lastErr, err)
	if err != nil {
		localSucc := s.app.sentCount - s.app.failCnt
		if localSucc < 0 {
			localSucc = 0
		}
		return localSucc, s.app.failCnt, err
	}
	s.app.successCnt = succ
	s.app.failCnt = fail
	s.app = nil
	return succ, fail, nil
}

func CliAppendFlush(stmt unsafe.Pointer) error {
	if stmt == nil {
		return nil
	}
	s := (*stmtHandle)(stmt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil || s.conn == nil || s.conn.native == nil {
		return nil
	}
	return flushAppendBufferedLocked(s, false)
}
