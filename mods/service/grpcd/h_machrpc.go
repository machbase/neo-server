package grpcd

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/machbase/neo-grpc/machrpc"
	spi "github.com/machbase/neo-spi"
)

//// machrpc server handler

var _ machrpc.MachbaseServer = &grpcd{}

func (s *grpcd) Ping(pctx context.Context, req *machrpc.PingRequest) (*machrpc.PingResponse, error) {
	rsp := &machrpc.PingResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Explain panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	rsp.Success, rsp.Reason = true, "success"
	rsp.Token = req.Token
	return rsp, nil
}

func (s *grpcd) Explain(pctx context.Context, req *machrpc.ExplainRequest) (*machrpc.ExplainResponse, error) {
	rsp := &machrpc.ExplainResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Explain panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if plan, err := s.db.Explain(req.Sql); err == nil {
		rsp.Success, rsp.Reason = true, "success"
		rsp.Plan = plan
	} else {
		rsp.Success, rsp.Reason = false, err.Error()
	}
	return rsp, nil
}

func (s *grpcd) Exec(pctx context.Context, req *machrpc.ExecRequest) (*machrpc.ExecResponse, error) {
	rsp := &machrpc.ExecResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Exec panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	params := machrpc.ConvertPbToAny(req.Params)
	if result := s.db.Exec(req.Sql, params...); result.Err() == nil {
		rsp.RowsAffected = result.RowsAffected()
		rsp.Success = true
		rsp.Reason = result.Message()
	} else {
		rsp.Success = false
		rsp.Reason = result.Message()
	}
	return rsp, nil
}

func (s *grpcd) QueryRow(pctx context.Context, req *machrpc.QueryRowRequest) (*machrpc.QueryRowResponse, error) {
	rsp := &machrpc.QueryRowResponse{}

	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("QueryRow panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	params := machrpc.ConvertPbToAny(req.Params)
	row := s.db.QueryRow(req.Sql, params...)

	if row.Err() != nil {
		rsp.Reason = row.Err().Error()
		return rsp, nil
	}

	var err error
	rsp.Success = true
	rsp.Reason = "success"
	rsp.Values, err = machrpc.ConvertAnyToPb(row.Values())
	rsp.RowsAffected = row.RowsAffected()
	rsp.Message = row.Message()
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
	}

	return rsp, err
}

func (s *grpcd) Query(pctx context.Context, req *machrpc.QueryRequest) (*machrpc.QueryResponse, error) {
	rsp := &machrpc.QueryResponse{}

	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Query panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	params := machrpc.ConvertPbToAny(req.Params)
	realRows, err := s.db.Query(req.Sql, params...)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if realRows.IsFetchable() {
		handle := strconv.FormatInt(atomic.AddInt64(&contextIdSerial, 1), 10)
		// TODO leak detector
		s.ctxMap.Set(handle, &rowsWrap{
			id:   handle,
			rows: realRows,
			release: func() {
				s.ctxMap.RemoveCb(handle, func(key string, v interface{}, exists bool) bool {
					realRows.Close()
					return true
				})
			},
		})
		rsp.RowsHandle = &machrpc.RowsHandle{
			Handle: handle,
		}
		rsp.Reason = "success"
	} else {
		rsp.RowsAffected = realRows.RowsAffected()
		rsp.Reason = realRows.Message()
	}
	rsp.Success = true

	return rsp, nil
}

func (s *grpcd) Columns(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.ColumnsResponse, error) {
	rsp := &machrpc.ColumnsResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Columns panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	rowsWrapVal, exists := s.ctxMap.Get(rows.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("handle '%s' not found", rows.Handle)
		return rsp, nil
	}
	rowsWrap, ok := rowsWrapVal.(*rowsWrap)
	if !ok {
		rsp.Reason = fmt.Sprintf("handle '%s' is not valid", rows.Handle)
		return rsp, nil
	}

	cols, err := rowsWrap.rows.Columns()
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Columns = make([]*machrpc.Column, len(cols))
	for i, c := range cols {
		rsp.Columns[i] = &machrpc.Column{
			Name:   c.Name,
			Type:   c.Type,
			Size:   int32(c.Size),
			Length: int32(c.Length),
		}
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *grpcd) RowsFetch(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.RowsFetchResponse, error) {
	rsp := &machrpc.RowsFetchResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("RowsFetch panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	rowsWrapVal, exists := s.ctxMap.Get(rows.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("handle '%s' not found", rows.Handle)
		return rsp, nil
	}
	rowsWrap, ok := rowsWrapVal.(*rowsWrap)
	if !ok {
		rsp.Reason = fmt.Sprintf("handle '%s' is not valid", rows.Handle)
		return rsp, nil
	}

	if !rowsWrap.rows.Next() {
		rsp.Success = true
		rsp.Reason = "success"
		rsp.HasNoRows = true
		return rsp, nil
	}

	columns, err := rowsWrap.rows.Columns()
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}

	values := columns.MakeBuffer()
	err = rowsWrap.rows.Scan(values...)
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Values, err = machrpc.ConvertAnyToPb(values)
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *grpcd) RowsClose(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.RowsCloseResponse, error) {
	rsp := &machrpc.RowsCloseResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("RowsClose panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	rowsWrapVal, exists := s.ctxMap.Get(rows.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("handle '%s' not found", rows.Handle)
		return rsp, nil
	}
	rowsWrap, ok := rowsWrapVal.(*rowsWrap)
	if !ok {
		rsp.Reason = fmt.Sprintf("handle '%s' is not valid", rows.Handle)
		return rsp, nil
	}

	rowsWrap.release()
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *grpcd) Appender(ctx context.Context, req *machrpc.AppenderRequest) (*machrpc.AppenderResponse, error) {
	rsp := &machrpc.AppenderResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Appender panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	opts := []spi.AppendOption{}
	if len(req.Timeformat) > 0 {
		opts = append(opts, spi.AppendTimeformatOption(req.Timeformat))
	}
	realAppender, err := s.db.Appender(req.TableName, opts...)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	tableType := realAppender.TableType()
	tableName := realAppender.TableName()
	handle := strconv.FormatInt(atomic.AddInt64(&contextIdSerial, 1), 10)
	wrap := &appenderWrap{
		id:       handle,
		appender: realAppender,
		closed:   false,
	}
	wrap.release = func() {
		s.ctxMap.RemoveCb(handle, func(key string, v interface{}, exists bool) bool {
			if !wrap.closed {
				s.log.Tracef("close appender:%v", handle)
				realAppender.Close()
			}
			return true
		})
	}
	s.ctxMap.Set(handle, wrap)
	s.log.Tracef("open appender:%v", handle)
	rsp.Success = true
	rsp.Reason = "success"
	rsp.Handle = handle
	rsp.TableName = tableName
	rsp.TableType = int32(tableType)
	return rsp, nil
}

type appenderWrap struct {
	id       string
	appender spi.Appender
	release  func()
	closed   bool
}

func (s *grpcd) Append(stream machrpc.Machbase_AppendServer) error {
	var wrap *appenderWrap
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Append panic recover", panic)
		}
		if wrap == nil {
			return
		}
		wrap.release()
	}()

	tick := time.Now()
	for {
		m, err := stream.Recv()
		if err == io.EOF {
			//
			// Caution: m is nil
			//
			var successCount, failCount int64
			if wrap != nil && wrap.appender != nil {
				successCount, failCount, _ = wrap.appender.Close()
				s.log.Tracef("close appender:%v success:%d fail:%d", wrap.id, successCount, failCount)
				wrap.closed = true
			}

			return stream.SendAndClose(&machrpc.AppendDone{
				Success:      true,
				Reason:       "success",
				Elapse:       time.Since(tick).String(),
				SuccessCount: successCount,
				FailCount:    failCount,
			})
		} else if err != nil {
			return err
		}

		if wrap == nil {
			appenderWrapVal, exists := s.ctxMap.Get(m.Handle)
			if !exists {
				s.log.Error("handle not found", m.Handle)
				return fmt.Errorf("handle '%s' not found", m.Handle)
			}
			appenderWrap, ok := appenderWrapVal.(*appenderWrap)
			if !ok {
				s.log.Error("handle invalid", m.Handle)
				return fmt.Errorf("handle '%s' is not valid", m.Handle)
			}
			wrap = appenderWrap
		}

		if wrap.id != m.Handle {
			s.log.Error("handle changed", m.Handle)
			return fmt.Errorf("not allowed changing handle in a stream")
		}

		if len(m.Records) > 0 {
			for _, rec := range m.Records {
				values, err := machrpc.ConvertPbTupleToAny(rec.Tuple)
				if err != nil {
					s.log.Error("append-unmarshal", err.Error())
				}
				err = wrap.appender.Append(values...)
				if err != nil {
					s.log.Error("append", err.Error())
					return err
				}
			}
		}
		if len(m.Params) > 0 {
			// for gRPC client that utilizes protobuf.Any (e.g: Python, C#, Java)
			values := machrpc.ConvertPbToAny(m.Params)
			err = wrap.appender.Append(values...)
			if err != nil {
				s.log.Error("append", err.Error())
				return err
			}
		}
	}
}

func (s *grpcd) UserAuth(pctx context.Context, req *machrpc.UserAuthRequest) (*machrpc.UserAuthResponse, error) {
	rsp := &machrpc.UserAuthResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("UserAuth panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	if db, ok := s.db.(spi.DatabaseAuth); ok {
		passed, err := db.UserAuth(req.LoginName, req.Password)
		if err != nil {
			rsp.Reason = err.Error()
		} else if passed {
			rsp.Success = passed
			rsp.Reason = "success"
		} else {
			rsp.Reason = "invalid username or password"
		}
	} else {
		rsp.Reason = "database is not support user-auth"
	}

	return rsp, nil
}

func (s *grpcd) GetServerInfo(pctx context.Context, req *machrpc.ServerInfoRequest) (*machrpc.ServerInfo, error) {
	rsp := &machrpc.ServerInfo{
		Runtime: &machrpc.Runtime{},
		Version: &machrpc.Version{},
	}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("GetServerInfo panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	nfo, err := s.db.GetServerInfo()
	if err != nil {
		return nil, err
	}

	rsp.Runtime.OS = nfo.Runtime.OS
	rsp.Runtime.Arch = nfo.Runtime.Arch
	rsp.Runtime.Pid = nfo.Runtime.Pid
	rsp.Runtime.UptimeInSecond = nfo.Runtime.UptimeInSecond
	rsp.Runtime.Processes = nfo.Runtime.Processes
	rsp.Runtime.Goroutines = nfo.Runtime.Goroutines
	rsp.Runtime.MemSys = nfo.Runtime.MemSys
	rsp.Runtime.MemHeapSys = nfo.Runtime.MemHeapSys
	rsp.Runtime.MemHeapAlloc = nfo.Runtime.MemHeapAlloc
	rsp.Runtime.MemHeapInUse = nfo.Runtime.MemHeapInUse
	rsp.Runtime.MemStackSys = nfo.Runtime.MemStackSys
	rsp.Runtime.MemStackInUse = nfo.Runtime.MemStackInUse

	rsp.Version.Major = nfo.Version.Major
	rsp.Version.Minor = nfo.Version.Minor
	rsp.Version.Patch = nfo.Version.Patch
	rsp.Version.GitSHA = nfo.Version.GitSHA
	rsp.Version.BuildTimestamp = nfo.Version.BuildTimestamp
	rsp.Version.BuildCompiler = nfo.Version.BuildCompiler
	rsp.Version.Engine = nfo.Version.Engine

	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}
