package bridge

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	bridgerpc "github.com/machbase/neo-server/v8/api/bridge"
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

// ////////////////////////////
// runtime service
func (s *svr) Exec(ctx context.Context, req *bridgerpc.ExecRequest) (*bridgerpc.ExecResponse, error) {
	rsp := &bridgerpc.ExecResponse{}
	tick := time.Now()
	conn, err := GetBridge(req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		return rsp, nil
	}
	switch br := conn.(type) {
	case SqlBridge:
		switch cmd := req.Command.(type) {
		case *bridgerpc.ExecRequest_SqlExec:
			return s.execSqlBridge(br, ctx, req)
		case *bridgerpc.ExecRequest_SqlQuery:
			return s.querySqlBridge(br, req)
		default:
			rsp.Reason = fmt.Sprintf("%s does not support %T", conn.String(), cmd)
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
	case PythonBridge:
		switch cmd := req.Command.(type) {
		case *bridgerpc.ExecRequest_Invoke:
			return s.execPythonBridge(br, ctx, cmd)
		default:
			rsp.Reason = fmt.Sprintf("%s does not support %T", conn.String(), cmd)
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

func (s *svr) execSqlBridge(br SqlBridge, ctx context.Context, req *bridgerpc.ExecRequest) (*bridgerpc.ExecResponse, error) {
	cmd := req.Command.(*bridgerpc.ExecRequest_SqlExec).SqlExec
	rsp := &bridgerpc.ExecResponse{}
	tick := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.Error("panic recover", err)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	params, err := ConvertFromDatum(cmd.Params...)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	conn, err := br.Connect(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, cmd.SqlText, params...)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	ret := &bridgerpc.SqlExecResult{}
	if br.SupportLastInsertId() {
		ret.LastInsertedId, err = result.LastInsertId()
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
	} else {
		ret.LastInsertedId = -1
	}
	ret.RowsAffected, err = result.RowsAffected()
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success, rsp.Reason = true, "success"
	rsp.Result = &bridgerpc.ExecResponse_SqlExecResult{SqlExecResult: ret}
	return rsp, nil
}

func (s *svr) querySqlBridge(br SqlBridge, req *bridgerpc.ExecRequest) (*bridgerpc.ExecResponse, error) {
	cmd := req.Command.(*bridgerpc.ExecRequest_SqlQuery).SqlQuery
	rsp := &bridgerpc.ExecResponse{}
	tick := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.Error("panic recover", err)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	params, err := ConvertFromDatum(cmd.Params...)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	// context should be managed independently,
	// to keep the state of sql conn and rows
	ctx := context.Background()
	conn, err := br.Connect(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rows, err := conn.QueryContext(ctx, cmd.SqlText, params...)
	if err != nil {
		rsp.Reason = err.Error()
		conn.Close()
		return rsp, nil
	}
	cols, err := rows.ColumnTypes()
	if err != nil {
		rsp.Reason = err.Error()
		rows.Close()
		conn.Close()
		return rsp, nil
	}

	ret := &bridgerpc.SqlQueryResult{}
	for _, c := range cols {
		length, _ := c.Length()
		ret.Fields = append(ret.Fields, &bridgerpc.SqlQueryResultField{
			Name:   c.Name(),
			Type:   c.DatabaseTypeName(),
			Size:   int32(length),
			Length: int32(length),
		})
	}
	rsp.Result = &bridgerpc.ExecResponse_SqlQueryResult{SqlQueryResult: ret}

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

func (s *svr) SqlQueryResultFetch(ctx context.Context, cr *bridgerpc.SqlQueryResult) (*bridgerpc.SqlQueryResultFetchResponse, error) {
	rsp := &bridgerpc.SqlQueryResultFetchResponse{}
	tick := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.Error("ResultFetch panic recover", err)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	rowsWrap, exists := s.ctxMap.Get(cr.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("handle '%s' not found", cr.Handle)
		return rsp, nil
	}
	if rowsWrap.rows == nil || rowsWrap.conn == nil {
		rsp.Reason = fmt.Sprintf("handle '%s' is invalid", cr.Handle)
		return rsp, nil
	}

	if !rowsWrap.rows.Next() {
		if err := rowsWrap.rows.Err(); err != nil {
			rsp.Success = false
			rsp.Reason = err.Error()
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
		rsp.Reason = err.Error()
		return rsp, nil
	}

	fields := make([]any, len(columns))
	for i, c := range columns {
		fields[i] = rowsWrap.bridge.NewScanType(c.ScanType().String(), strings.ToUpper(c.DatabaseTypeName()))
	}
	err = rowsWrap.rows.Scan(fields...)
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Values, err = ConvertToDatum(fields...)
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *svr) SqlQueryResultClose(ctx context.Context, cr *bridgerpc.SqlQueryResult) (*bridgerpc.SqlQueryResultCloseResponse, error) {
	rsp := &bridgerpc.SqlQueryResultCloseResponse{}
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

func (s *svr) execPythonBridge(br PythonBridge, ctx context.Context, req *bridgerpc.ExecRequest_Invoke) (*bridgerpc.ExecResponse, error) {
	rsp := &bridgerpc.ExecResponse{}

	tick := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.Error("panic recover", err)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	exitCode, stdout, stderr, err := br.Invoke(ctx, req.Invoke.Args, req.Invoke.Stdin)
	if err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success, rsp.Reason = true, "success"
	}
	rsp.Result = &bridgerpc.ExecResponse_InvokeResult{InvokeResult: &bridgerpc.InvokeResult{
		ExitCode: int32(exitCode),
		Stdout:   stdout,
		Stderr:   stderr,
	}}
	return rsp, nil
}
