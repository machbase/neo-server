package machnet

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type nativeConn struct {
	mu sync.Mutex

	netConn net.Conn
	br      *bufio.Reader
	bw      *bufio.Writer

	host         string
	port         int
	user         string
	password     string
	queryTimeout time.Duration

	sessionID    uint64
	serverEndian uint32
	closed       bool

	stmtMu      sync.Mutex
	stmtCursor  uint32
	stmtUsed    [stmtIDLimit]bool
	stmtUsedCnt int
}

const stmtIDLimit = 1024

type stmtExecResult struct {
	stmtType   int
	message    string
	rowCount   int64
	columns    []columnMeta
	paramDescs []CliParamDesc
	rows       [][]any
	lastResult bool
}

func readUIntLE(data []byte) (uint64, bool) {
	switch {
	case len(data) >= 8:
		return binary.LittleEndian.Uint64(data[:8]), true
	case len(data) >= 4:
		return uint64(binary.LittleEndian.Uint32(data[:4])), true
	case len(data) >= 2:
		return uint64(binary.LittleEndian.Uint16(data[:2])), true
	case len(data) >= 1:
		return uint64(data[0]), true
	default:
		return 0, false
	}
}

func countSQLPlaceholders(sql string) int {
	if sql == "" {
		return 0
	}
	cnt := 0
	inSingle := false
	inDouble := false
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if inSingle {
			if ch == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		switch ch {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '?':
			cnt++
		}
	}
	return cnt
}

func dialNative(host string, port int, user string, password string, alts []net.TCPAddr) (*nativeConn, error) {
	endpoints := make([]string, 0, 1+len(alts))
	endpoints = append(endpoints, fmt.Sprintf("%s:%d", host, port))
	for _, alt := range alts {
		h := alt.IP.String()
		if h == "<nil>" || h == "" {
			continue
		}
		endpoints = append(endpoints, fmt.Sprintf("%s:%d", h, alt.Port))
	}
	var lastErr error
	for _, ep := range endpoints {
		c, err := net.DialTimeout("tcp", ep, defaultConnectTimeout)
		if err != nil {
			lastErr = err
			continue
		}
		nc := &nativeConn{
			netConn:      c,
			br:           bufio.NewReaderSize(c, defaultReadBufferSize),
			bw:           bufio.NewWriterSize(c, defaultWriteBufferSize),
			host:         host,
			port:         port,
			user:         user,
			password:     password,
			queryTimeout: defaultQueryTimeout,
		}
		if err := nc.handshake(); err != nil {
			_ = c.Close()
			lastErr = err
			continue
		}
		if err := nc.connectProtocol(); err != nil {
			_ = c.Close()
			lastErr = err
			continue
		}
		return nc, nil
	}
	if lastErr == nil {
		lastErr = errors.New("connect failed")
	}
	return nil, lastErr
}

func (c *nativeConn) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.netConn != nil {
		return c.netConn.Close()
	}
	return nil
}

func (c *nativeConn) nextStmtID() (uint32, error) {
	c.stmtMu.Lock()
	defer c.stmtMu.Unlock()

	if c.stmtUsedCnt >= stmtIDLimit {
		return 0, makeClientErr(fmt.Sprintf("Statement ID overflow (Limit = %d, Curr = %d).", stmtIDLimit, c.stmtUsedCnt))
	}
	start := c.stmtCursor % stmtIDLimit
	for i := uint32(0); i < stmtIDLimit; i++ {
		cand := (start + i) % stmtIDLimit
		if !c.stmtUsed[cand] {
			c.stmtUsed[cand] = true
			c.stmtUsedCnt++
			c.stmtCursor = (cand + 1) % stmtIDLimit
			return cand, nil
		}
	}
	return 0, makeClientErr(fmt.Sprintf("Statement ID overflow (Limit = %d, Curr = %d).", stmtIDLimit, c.stmtUsedCnt))
}

func (c *nativeConn) releaseStmtID(id uint32) {
	if id >= stmtIDLimit {
		return
	}
	c.stmtMu.Lock()
	defer c.stmtMu.Unlock()
	if c.stmtUsed[id] {
		c.stmtUsed[id] = false
		if c.stmtUsedCnt > 0 {
			c.stmtUsedCnt--
		}
	}
}

func (c *nativeConn) handshake() error {
	payload := []byte(cmiHandshakePayload)
	if len(payload) != cmiProtoCnt {
		return fmt.Errorf("invalid handshake payload size")
	}
	if err := writeAll(c.netConn, payload, defaultConnectTimeout); err != nil {
		return err
	}
	resp := make([]byte, cmiProtoCnt)
	if _, err := io.ReadFull(c.netConn, resp); err != nil {
		return err
	}
	if string(resp) != cmiHandshakeReady {
		return fmt.Errorf("handshake failed: %q", string(resp))
	}
	return nil
}

func (c *nativeConn) sendPackets(packets [][]byte, expected byte, timeout time.Duration) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("connection closed")
	}
	if timeout > 0 {
		_ = c.netConn.SetWriteDeadline(time.Now().Add(timeout))
		defer c.netConn.SetWriteDeadline(time.Time{})
	}
	for _, p := range packets {
		if err := writeAllNoDeadline(c.bw, p); err != nil {
			return nil, err
		}
	}
	if err := c.bw.Flush(); err != nil {
		return nil, err
	}
	return readProtocolFrom(c.br, c.netConn, expected, timeout)
}

func (c *nativeConn) sendPacketsNoResponse(packets [][]byte, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("connection closed")
	}
	if timeout > 0 {
		_ = c.netConn.SetWriteDeadline(time.Now().Add(timeout))
		defer c.netConn.SetWriteDeadline(time.Time{})
	}
	for _, p := range packets {
		if err := writeAllNoDeadline(c.bw, p); err != nil {
			return err
		}
	}
	if err := c.bw.Flush(); err != nil {
		return err
	}
	return nil
}

func (c *nativeConn) sendPacketsOptional(packets [][]byte, expected byte, timeout time.Duration) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, false, errors.New("connection closed")
	}
	if timeout > 0 {
		_ = c.netConn.SetWriteDeadline(time.Now().Add(timeout))
		defer c.netConn.SetWriteDeadline(time.Time{})
	}
	for _, p := range packets {
		if err := writeAllNoDeadline(c.bw, p); err != nil {
			return nil, false, err
		}
	}
	if err := c.bw.Flush(); err != nil {
		return nil, false, err
	}
	// Probe 1 byte first so optional timeout does not partially consume protocol data.
	if timeout > 0 {
		_ = c.netConn.SetReadDeadline(time.Now().Add(timeout))
	}
	_, peekErr := c.br.Peek(1)
	if timeout > 0 {
		_ = c.netConn.SetReadDeadline(time.Time{})
	}
	if peekErr != nil {
		var netErr net.Error
		if errors.As(peekErr, &netErr) && netErr.Timeout() {
			return nil, false, nil
		}
		return nil, false, peekErr
	}
	body, err := readProtocolFrom(c.br, c.netConn, expected, c.queryTimeout)
	if err != nil {
		return nil, false, err
	}
	return body, true, nil
}

func (c *nativeConn) connectProtocol() error {
	w := newMarshalWriter(cmiConnectProtocol, 0, 0)
	w.addUInt64(cmiCVersionID, protocolVersion())
	w.addString(cmiCClientID, "CLI")
	w.addString(cmiCDatabaseID, "data")
	w.addString(cmiCUserID, c.user)
	w.addString(cmiCPasswordID, c.password)
	w.addUInt64(cmiCTimeoutID, uint64(defaultQueryTimeout.Seconds()))
	w.addUInt32(cmiCSHCID, 0)
	if la, ok := c.netConn.LocalAddr().(*net.TCPAddr); ok && la.IP != nil {
		w.addString(cmiCIPID, la.IP.String())
	} else {
		w.addString(cmiCIPID, "127.0.0.1")
	}
	body, err := c.sendPackets(w.finalize(), cmiConnectProtocol, defaultConnectTimeout)
	if err != nil {
		return err
	}
	units, err := collectUnits(body)
	if err != nil {
		return err
	}
	result, ok := firstUnit(units, cmiRResultID)
	if !ok || len(result.data) < 8 {
		return errors.New("connect response missing result")
	}
	statusVal := binary.LittleEndian.Uint64(result.data)
	if statusCode(statusVal) != cmiOKResult {
		msg := ""
		if m, ok := firstUnit(units, cmiRMessageID); ok {
			msg = string(m.data)
		}
		return makeServerErr(statusErrNo(statusVal), msg)
	}
	if sid, ok := firstUnit(units, cmiCSIDID); ok && len(sid.data) >= 8 {
		c.sessionID = binary.LittleEndian.Uint64(sid.data)
	}
	if e, ok := firstUnit(units, cmiCEndianID); ok {
		switch {
		case len(e.data) >= 4:
			c.serverEndian = binary.LittleEndian.Uint32(e.data[:4])
		case len(e.data) >= 1:
			c.serverEndian = uint32(e.data[0])
		default:
			c.serverEndian = 0
		}
	}
	return nil
}

func parseStmtResponse(body []byte, sql string, fallbackCols []columnMeta) (*stmtExecResult, error) {
	units, err := collectUnits(body)
	if err != nil {
		return nil, err
	}
	ret := &stmtExecResult{stmtType: inferStmtType(sql)}

	if m, ok := firstUnit(units, cmiRMessageID); ok {
		ret.message = string(m.data)
	}
	if rc, ok := firstUnit(units, cmiPRowsID); ok {
		if v, ok := readUIntLE(rc.data); ok {
			ret.rowCount = int64(v)
		}
	}
	if st, ok := firstUnit(units, cmimIDStmtType); ok {
		if len(st.data) >= 4 {
			ret.stmtType = int(int32(binary.LittleEndian.Uint32(st.data[:4])))
		}
	}

	paramTypeUnits := units[cmiPParamTypeID]
	switch {
	case len(paramTypeUnits) > 0:
		ret.paramDescs = buildParamDescs(units, len(paramTypeUnits))
	default:
		qCount := countSQLPlaceholders(sql)
		if qCount > 0 {
			ret.paramDescs = make([]CliParamDesc, qCount)
			for i := range ret.paramDescs {
				ret.paramDescs[i] = CliParamDesc{Type: MACHCLI_SQL_TYPE_STRING, Nullable: true}
			}
		}
	}

	ret.columns = buildColumns(units)
	if len(ret.columns) == 0 && len(fallbackCols) > 0 {
		ret.columns = append([]columnMeta(nil), fallbackCols...)
	}
	if v := units[cmiFValueID]; len(v) > 0 && len(ret.columns) > 0 {
		rows, deErr := decodeRowsFromUnits(v, ret.columns)
		if deErr != nil {
			return nil, deErr
		}
		ret.rows = append(ret.rows, rows...)
	}

	if results := units[cmiRResultID]; len(results) > 0 {
		for _, result := range results {
			if len(result.data) < 8 {
				continue
			}
			statusVal := binary.LittleEndian.Uint64(result.data)
			st := statusCode(statusVal)
			if st == cmiLastResult {
				ret.lastResult = true
			}
			if st != cmiOKResult && st != cmiLastResult {
				emsg := ""
				if em, ok := firstUnit(units, cmiREMessageID); ok {
					emsg = string(em.data)
				}
				msg := ret.message
				if emsg != "" {
					if msg == "" {
						msg = emsg
					} else {
						msg = msg + "; " + emsg
					}
				}
				return nil, makeServerErr(statusErrNo(statusVal), msg)
			}
		}
	}
	return ret, nil
}

func (c *nativeConn) fetchRows(stmtID uint32, columns []columnMeta) ([][]any, error) {
	ret := make([][]any, 0, 32)
	for {
		w := newMarshalWriter(cmiFetchProtocol, stmtID, 0)
		w.addUInt32(cmiFIDID, stmtID)
		w.addSInt64(cmiFRowsID, 1000)
		body, err := c.sendPackets(w.finalize(), cmiFetchProtocol, c.queryTimeout)
		if err != nil {
			return nil, err
		}
		units, err := collectUnits(body)
		if err != nil {
			return nil, err
		}
		last := false
		if results := units[cmiRResultID]; len(results) > 0 {
			for _, result := range results {
				if len(result.data) < 8 {
					continue
				}
				statusVal := binary.LittleEndian.Uint64(result.data)
				st := statusCode(statusVal)
				if st == cmiLastResult {
					last = true
				}
				if st != cmiOKResult && st != cmiLastResult {
					msg := ""
					if m, ok := firstUnit(units, cmiRMessageID); ok {
						msg = string(m.data)
					}
					return nil, makeServerErr(statusErrNo(statusVal), msg)
				}
			}
		}
		if vals := units[cmiFValueID]; len(vals) > 0 {
			rows, deErr := decodeRowsFromUnits(vals, columns)
			if deErr != nil {
				return nil, deErr
			}
			ret = append(ret, rows...)
		}
		if last {
			break
		}
		if r, ok := firstUnit(units, cmiFRowsID); ok {
			if v, ok := readUIntLE(r.data); ok && int64(v) == 0 && len(units[cmiFValueID]) == 0 {
				break
			}
		}
	}
	return ret, nil
}

func (c *nativeConn) execDirect(stmtID uint32, sql string) (*stmtExecResult, error) {
	w := newMarshalWriter(cmiExecDirectProtocol, stmtID, 0)
	w.addString(cmiDStatementID, sql)
	w.addUInt64(cmiPIDID, uint64(stmtID))
	body, err := c.sendPackets(w.finalize(), cmiExecDirectProtocol, c.queryTimeout)
	if err != nil {
		return nil, err
	}
	ret, err := parseStmtResponse(body, sql, nil)
	if err != nil {
		return nil, err
	}
	if len(ret.columns) > 0 {
		if !ret.lastResult {
			rows, fErr := c.fetchRows(stmtID, ret.columns)
			if fErr != nil {
				return nil, fErr
			}
			ret.rows = append(ret.rows, rows...)
		}
		ret.rowCount = int64(len(ret.rows))
	}
	if ret.stmtType == 0 {
		ret.stmtType = inferStmtType(sql)
	}
	return ret, nil
}

func (c *nativeConn) prepare(stmtID uint32, sql string) (*stmtExecResult, error) {
	w := newMarshalWriter(cmiPrepareProtocol, stmtID, 0)
	w.addUInt64(cmiPIDID, uint64(stmtID))
	w.addString(cmiPStatementID, sql)
	body, err := c.sendPackets(w.finalize(), cmiPrepareProtocol, c.queryTimeout)
	if err != nil {
		return nil, err
	}
	ret, err := parseStmtResponse(body, sql, nil)
	if err != nil {
		return nil, err
	}
	if ret.stmtType == 0 {
		ret.stmtType = inferStmtType(sql)
	}
	return ret, nil
}

func (c *nativeConn) executePrepared(stmtID uint32, sql string, params []boundParam, preparedCols []columnMeta) (*stmtExecResult, error) {
	w := newMarshalWriter(cmiExecuteProtocol, stmtID, 0)
	w.addUInt64(cmiPIDID, uint64(stmtID))
	w.addSInt64(cmiFRowsID, 1000)
	if len(params) > 0 {
		p, err := encodeParams(params)
		if err != nil {
			return nil, err
		}
		if len(p) > 0 {
			w.addBinary(cmiEParamID, p)
		}
	}
	body, err := c.sendPackets(w.finalize(), cmiExecuteProtocol, c.queryTimeout)
	if err != nil {
		return nil, err
	}
	ret, err := parseStmtResponse(body, sql, preparedCols)
	if err != nil {
		return nil, err
	}
	if len(ret.columns) > 0 {
		if !ret.lastResult {
			rows, fErr := c.fetchRows(stmtID, ret.columns)
			if fErr != nil {
				return nil, fErr
			}
			ret.rows = append(ret.rows, rows...)
		}
		ret.rowCount = int64(len(ret.rows))
	}
	if ret.stmtType == 0 {
		ret.stmtType = inferStmtType(sql)
	}
	return ret, nil
}

func (c *nativeConn) free(stmtID uint32) error {
	w := newMarshalWriter(cmiFreeProtocol, stmtID, 0)
	w.addUInt64(cmiXIDID, uint64(stmtID))
	body, err := c.sendPackets(w.finalize(), cmiFreeProtocol, c.queryTimeout)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected protocol") {
			return nil
		}
		return err
	}
	units, err := collectUnits(body)
	if err != nil {
		return err
	}
	if result, ok := firstUnit(units, cmiRResultID); ok && len(result.data) >= 8 {
		statusVal := binary.LittleEndian.Uint64(result.data)
		st := statusCode(statusVal)
		if st != cmiOKResult && st != cmiLastResult {
			msg := ""
			if m, ok := firstUnit(units, cmiRMessageID); ok {
				msg = string(m.data)
			}
			return makeServerErr(statusErrNo(statusVal), msg)
		}
	}
	return nil
}

func (c *nativeConn) appendOpen(stmtID uint32, table string, errCheckCount int) (*stmtExecResult, error) {
	_ = errCheckCount
	w := newMarshalWriter(cmiAppendOpenProtocol, stmtID, 0)
	w.addUInt64(cmiPIDID, uint64(stmtID))
	w.addString(cmiPTableID, table)
	w.addUInt64(cmiEEndianID, 0)
	body, err := c.sendPackets(w.finalize(), cmiAppendOpenProtocol, c.queryTimeout)
	if err != nil {
		return nil, err
	}
	ret, err := parseStmtResponse(body, "APPEND "+table, nil)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func parseAppendDataResponse(body []byte) error {
	units, err := collectUnits(body)
	if err != nil {
		return err
	}
	if results := units[cmiRResultID]; len(results) > 0 {
		for _, result := range results {
			if len(result.data) < 8 {
				continue
			}
			statusVal := binary.LittleEndian.Uint64(result.data)
			st := statusCode(statusVal)
			if st != cmiOKResult && st != cmiLastResult {
				msg := ""
				if m, ok := firstUnit(units, cmiRMessageID); ok {
					msg = string(m.data)
				}
				if em, ok := firstUnit(units, cmiREMessageID); ok {
					if msg == "" {
						msg = string(em.data)
					} else {
						msg += "; " + string(em.data)
					}
				}
				return makeServerErr(statusErrNo(statusVal), msg)
			}
		}
	}
	if fail, ok := firstUnit(units, cmiXAppendFailureID); ok && len(fail.data) >= 8 {
		failCnt := binary.LittleEndian.Uint64(fail.data[:8])
		if failCnt > 0 {
			msg := ""
			if m, ok := firstUnit(units, cmiRMessageID); ok {
				msg = string(m.data)
			}
			if msg == "" {
				msg = fmt.Sprintf("append data failed rows=%d", failCnt)
			}
			return makeServerErr(0, msg)
		}
	}
	return nil
}

func (c *nativeConn) appendData(stmtID uint32, rows [][]byte, checkResponse bool) error {
	if len(rows) == 0 {
		return nil
	}
	w := newMarshalWriter(cmiAppendDataProtocol, stmtID, uint16(stmtID&0xffff))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		w.addBinary(cmiPRowsID, row)
	}
	packets := w.finalize()
	if !checkResponse {
		fmt.Println("sendPacketsNoResponse() : Before 11")
		return c.sendPacketsNoResponse(packets, c.queryTimeout)
	}
	timeout := 5 * time.Millisecond
	if c.queryTimeout > 0 && timeout > c.queryTimeout {
		timeout = c.queryTimeout
	}
        fmt.Println("sendPacketsNoResponse() : Before 22")
	body, ok, err := c.sendPacketsOptional(packets, cmiAppendDataProtocol, timeout)
	if err != nil {
		return err
	}
	if !ok || len(body) == 0 {
		return nil
	}
	return parseAppendDataResponse(body)
}

func parseAppendCloseResponse(body []byte) (int64, int64, error) {
	units, err := collectUnits(body)
	if err != nil {
		return 0, 0, err
	}
	if result, ok := firstUnit(units, cmiRResultID); ok && len(result.data) >= 8 {
		statusVal := binary.LittleEndian.Uint64(result.data)
		st := statusCode(statusVal)
		if st != cmiOKResult && st != cmiLastResult {
			msg := ""
			if m, ok := firstUnit(units, cmiRMessageID); ok {
				msg = string(m.data)
			}
			return 0, 0, makeServerErr(statusErrNo(statusVal), msg)
		}
	}
	var succ int64
	var fail int64
	if v, ok := firstUnit(units, cmiXAppendSuccessID); ok && len(v.data) >= 8 {
		succ = int64(binary.LittleEndian.Uint64(v.data))
	}
	if v, ok := firstUnit(units, cmiXAppendFailureID); ok && len(v.data) >= 8 {
		fail = int64(binary.LittleEndian.Uint64(v.data))
	}
	return succ, fail, nil
}

func (c *nativeConn) appendClose(stmtID uint32) (int64, int64, error) {
	w := newMarshalWriter(cmiAppendCloseProtocol, stmtID, 0)
	w.addUInt64(cmiPIDID, uint64(stmtID))
	packets := w.finalize()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, 0, errors.New("connection closed")
	}

	if c.queryTimeout > 0 {
		_ = c.netConn.SetWriteDeadline(time.Now().Add(c.queryTimeout))
		defer c.netConn.SetWriteDeadline(time.Time{})
	}
	for _, p := range packets {
		if err := writeAllNoDeadline(c.bw, p); err != nil {
			return 0, 0, err
		}
	}
	if err := c.bw.Flush(); err != nil {
		return 0, 0, err
	}

	for {
		fmt.Println("readNextProtocolFrom() : Before")
		protocol, body, err := readNextProtocolFrom(c.br, c.netConn, c.queryTimeout)
		if err != nil {
			return 0, 0, err
		}
		switch protocol {
		case cmiAppendDataProtocol:
			if err := parseAppendDataResponse(body); err != nil {
				return 0, 0, err
			}
		case cmiAppendCloseProtocol:
			return parseAppendCloseResponse(body)
		default:
			return 0, 0, fmt.Errorf("unexpected protocol %d expected %d", protocol, cmiAppendCloseProtocol)
		}
	}
}
