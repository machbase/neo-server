package bridge

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type rowsWrap struct {
	id      string
	conn    *sql.Conn
	rows    *sql.Rows
	ctx     context.Context
	release func()

	bridge     SqlBridge
	enlistInfo string
	enlistTime time.Time
}

var contextIdSerial int64

type ExecRequest struct {
	Name    string      `json:"name"`
	Command ExecCommand `json:"command"`
}

type ExecCommand struct {
	SqlExec  *SqlRequest    `json:"sqlExec,omitempty"`
	SqlQuery *SqlRequest    `json:"sqlQuery,omitempty"`
	Invoke   *InvokeRequest `json:"invoke,omitempty"`
}

type SqlRequest struct {
	SqlText string `json:"sqlText"`
	Params  []any  `json:"params,omitempty"`
}

type InvokeRequest struct {
	Args  []string `json:"args"`
	Stdin []byte   `json:"stdin,omitempty"`
}

type ExecResponse struct {
	Success bool       `json:"success"`
	Reason  string     `json:"reason"`
	Elapse  string     `json:"elapse"`
	Result  ExecResult `json:"result"`
}

type ExecResult struct {
	SqlExecResult  *SqlExecResult  `json:"sqlExecResult,omitempty"`
	SqlQueryResult *SqlQueryResult `json:"sqlQueryResult,omitempty"`
	InvokeResult   *InvokeResult   `json:"invokeResult,omitempty"`
}

type SqlExecResult struct {
	LastInsertedId int64 `json:"lastInsertedId"`
	RowsAffected   int64 `json:"rowsAffected"`
}

type SqlQueryResult struct {
	Handle string                 `json:"handle,omitempty"`
	Fields []*SqlQueryResultField `json:"fields,omitempty"`
}

type SqlQueryResultField struct {
	Name   string `json:"name,omitempty"`
	Type   string `json:"type,omitempty"`
	Size   int32  `json:"size,omitempty"`
	Length int32  `json:"length,omitempty"`
}

type InvokeResult struct {
	ExitCode int32  `json:"exitCode"`
	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
}

// ////////////////////////////
// runtime service
func (s *Service) Exec(ctx context.Context, req *ExecRequest) (*ExecResponse, error) {
	rsp := &ExecResponse{}
	tick := time.Now()
	conn, err := GetBridge(req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		return rsp, nil
	}
	switch br := conn.(type) {
	case SqlBridge:
		switch {
		case req.Command.SqlExec != nil:
			return s.execSqlBridge(br, ctx, req)
		case req.Command.SqlQuery != nil:
			return s.querySqlBridge(br, req)
		default:
			rsp.Reason = fmt.Sprintf("%s does not support %T", conn.String(), req.Command)
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
	case PythonBridge:
		switch {
		case req.Command.Invoke != nil:
			return s.execPythonBridge(br, ctx, req.Command.Invoke)
		default:
			rsp.Reason = fmt.Sprintf("%s does not support %T", conn.String(), req.Command)
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
	case Bridge:
		rsp.Reason = fmt.Sprintf("%s does not support exec", conn.String())
		rsp.Elapse = time.Since(tick).String()
		return rsp, nil
	default:
		rsp.Reason = fmt.Sprintf("%s is unknown", conn.String())
		rsp.Elapse = time.Since(tick).String()
		return rsp, nil
	}
}

func (s *Service) execSqlBridge(br SqlBridge, ctx context.Context, req *ExecRequest) (*ExecResponse, error) {
	cmd := req.Command.SqlExec
	rsp := &ExecResponse{}
	tick := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.Error("panic recover", err)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	conn, err := br.Connect(ctx)
	if err != nil {
		rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
		return rsp, nil
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, cmd.SqlText, cmd.Params...)
	if err != nil {
		rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
		return rsp, nil
	}
	ret := &SqlExecResult{}
	if br.SupportLastInsertId() {
		ret.LastInsertedId, err = result.LastInsertId()
		if err != nil {
			rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
			return rsp, nil
		}
	} else {
		ret.LastInsertedId = -1
	}
	ret.RowsAffected, err = result.RowsAffected()
	if err != nil {
		rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
		return rsp, nil
	}
	rsp.Success, rsp.Reason = true, "success"
	rsp.Result.SqlExecResult = ret
	return rsp, nil
}

func (s *Service) querySqlBridge(br SqlBridge, req *ExecRequest) (*ExecResponse, error) {
	cmd := req.Command.SqlQuery
	rsp := &ExecResponse{}
	tick := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.Error("panic recover", err)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	// context should be managed independently,
	// to keep the state of sql conn and rows
	ctx := context.Background()
	conn, err := br.Connect(ctx)
	if err != nil {
		rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
		return rsp, nil
	}
	rows, err := conn.QueryContext(ctx, cmd.SqlText, cmd.Params...)
	if err != nil {
		rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
		conn.Close()
		return rsp, nil
	}
	cols, err := rows.ColumnTypes()
	if err != nil {
		rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
		rows.Close()
		conn.Close()
		return rsp, nil
	}

	ret := &SqlQueryResult{}
	for _, c := range cols {
		length, _ := c.Length()
		ret.Fields = append(ret.Fields, &SqlQueryResultField{
			Name:   c.Name(),
			Type:   c.DatabaseTypeName(),
			Size:   int32(length),
			Length: int32(length),
		})
	}
	rsp.Result.SqlQueryResult = ret

	if len(cols) > 0 { // Fetchable
		handle := strconv.FormatInt(atomic.AddInt64(&contextIdSerial, 1), 10)
		ret.Handle = handle
		// TODO leak detector
		s.ctxMap.Set(handle, &rowsWrap{
			id:         handle,
			conn:       conn,
			rows:       rows,
			ctx:        ctx,
			bridge:     br,
			enlistInfo: fmt.Sprintf("%s: %s", req.Name, cmd.SqlText),
			enlistTime: time.Now(),
			release: func() {
				s.ctxMap.RemoveCb(handle, func(key string, v *rowsWrap, exists bool) bool {
					rows.Close()
					conn.Close()
					return true
				})
			},
		})
	} else {
		rows.Close()
		conn.Close()
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

type SqlQueryResultFetchResponse struct {
	Success   bool   `json:"success,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Elapse    string `json:"elapse,omitempty"`
	HasNoRows bool   `json:"hasNoRows,omitempty"`
	Values    []any  `json:"values,omitempty"`
}

func (s *Service) SqlQueryResultFetch(ctx context.Context, cr *SqlQueryResult) (*SqlQueryResultFetchResponse, error) {
	rsp := &SqlQueryResultFetchResponse{}
	tick := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.Error("ResultFetch panic recover", err)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	rowsWrap, exists := s.ctxMap.Get(cr.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("SqlBridge: handle '%s' not found", cr.Handle)
		return rsp, nil
	}
	if rowsWrap.rows == nil || rowsWrap.conn == nil {
		rsp.Reason = fmt.Sprintf("SqlBridge: handle '%s' is invalid", cr.Handle)
		return rsp, nil
	}

	if !rowsWrap.rows.Next() {
		if err := rowsWrap.rows.Err(); err != nil {
			rsp.Success = false
			rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
		} else {
			rsp.Success = true
			rsp.Reason = "success"
		}
		rsp.HasNoRows = true
		return rsp, nil
	}

	columns, err := rowsWrap.rows.ColumnTypes()
	if err != nil {
		rsp.Success = false
		rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
		return rsp, nil
	}

	rsp.Values = make([]any, len(columns))
	for i, c := range columns {
		rsp.Values[i] = rowsWrap.bridge.NewScanType(c.ScanType().String(), strings.ToUpper(c.DatabaseTypeName()))
	}
	err = rowsWrap.rows.Scan(rsp.Values...)
	if err != nil {
		rsp.Success = false
		rsp.Reason = fmt.Sprintf("SqlBridge: %s", err.Error())
		return rsp, nil
	}
	for i, v := range rsp.Values {
		rsp.Values[i] = UnboxValueToNative(v)
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func UnboxValueToNative(v any) any {
	return UnboxValueToNativeTZ(v, time.UTC)
}

func UnboxValueToNativeTZ(v any, runtimeTZ *time.Location) any {
	switch val := v.(type) {
	case *float32:
		if val == nil {
			return nil
		}
		return *val
	case *float64:
		if val == nil {
			return nil
		}
		return *val
	case *int:
		if val == nil {
			return nil
		}
		return *val
	case *int8:
		if val == nil {
			return nil
		}
		return *val
	case *int16:
		if val == nil {
			return nil
		}
		return *val
	case *int32:
		if val == nil {
			return nil
		}
		return *val
	case *int64:
		if val == nil {
			return nil
		}
		return *val
	case *uint:
		if val == nil {
			return nil
		}
		return *val
	case *uint8:
		if val == nil {
			return nil
		}
		return *val
	case *uint16:
		if val == nil {
			return nil
		}
		return *val
	case *uint32:
		if val == nil {
			return nil
		}
		return *val
	case *uint64:
		if val == nil {
			return nil
		}
		return *val
	case *bool:
		if val == nil {
			return nil
		}
		return *val
	case *string:
		if val == nil {
			return nil
		}
		return *val
	case *time.Time:
		if val == nil {
			return nil
		}
		return *val
	case *sql.NullString:
		if val.Valid {
			return val.String
		}
		return nil
	case *sql.NullInt16:
		if val.Valid {
			return val.Int16
		}
		return nil
	case *sql.NullInt32:
		if val.Valid {
			return val.Int32
		}
		return nil
	case *sql.NullInt64:
		if val.Valid {
			return val.Int64
		}
		return nil
	case *sql.NullFloat64:
		if val.Valid {
			return val.Float64
		}
		return nil
	case *sql.NullBool:
		if val.Valid {
			return val.Bool
		}
		return nil
	case *sql.NullTime:
		if val.Valid {
			return val.Time.In(runtimeTZ)
		}
		return nil
	case *sql.NullByte:
		if val.Valid {
			return val.Byte
		}
		return nil
	case *sql.RawBytes:
		if val == nil {
			return nil
		}
		dst := make([]byte, len(*val))
		copy(dst, *val)
		return dst
	case *[]uint8:
		if val == nil {
			return nil
		}
		dst := make([]byte, len(*val))
		copy(dst, *val)
		return dst
	default:
		return v
	}
}

type SqlQueryResultCloseResponse struct {
	Success bool   `json:"success,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Elapse  string `json:"elapse,omitempty"`
}

func (s *Service) SqlQueryResultClose(ctx context.Context, cr *SqlQueryResult) (*SqlQueryResultCloseResponse, error) {
	rsp := &SqlQueryResultCloseResponse{}
	tick := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.Error("ResultClose panic recover", err)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	rowsWrap, exists := s.ctxMap.Get(cr.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("handle '%s' not found", cr.Handle)
		return rsp, nil
	}
	rowsWrap.release()
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *Service) execPythonBridge(br PythonBridge, ctx context.Context, req *InvokeRequest) (*ExecResponse, error) {
	rsp := &ExecResponse{}

	tick := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.Error("panic recover", err)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	exitCode, stdout, stderr, err := br.Invoke(ctx, req.Args, req.Stdin)
	if err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success, rsp.Reason = true, "success"
	}
	rsp.Result.InvokeResult = &InvokeResult{
		ExitCode: int32(exitCode),
		Stdout:   stdout,
		Stderr:   stderr,
	}
	return rsp, nil
}
