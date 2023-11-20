package grpcd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-grpc/machrpc"
	"github.com/machbase/neo-server/mods/leak"
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
	if conn, ok := s.pool[req.Conn.Handle]; !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		if pinger, ok := conn.rawConn.(spi.Pinger); ok {
			if _, err := pinger.Ping(); err != nil {
				rsp.Reason = err.Error()
			} else {
				rsp.Success, rsp.Reason = true, "success"
			}
		} else {
			rsp.Reason = "ping not supproted"
		}
	}
	rsp.Token = req.Token
	return rsp, nil
}

func (s *grpcd) Conn(pctx context.Context, req *machrpc.ConnRequest) (*machrpc.ConnResponse, error) {
	rsp := &machrpc.ConnResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Conn panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	connOpts := []spi.ConnectOption{}
	switch s.db.(type) {
	case spi.DatabaseClient:
		connOpts = append(connOpts, machrpc.WithPassword(req.User, req.Password))
	case spi.DatabaseServer:
		if strings.HasPrefix(req.Password, "$otp$:") {
			if passed, err := s.authServer.ValidateUserOtp(req.User, strings.TrimPrefix(req.Password, "$otp$:")); passed {
				connOpts = append(connOpts, mach.WithTrustUser(req.User))
			} else if err != nil {
				rsp.Reason = err.Error()
				return rsp, nil
			} else {
				rsp.Reason = "invalid user or password"
				return rsp, nil
			}
		} else {
			connOpts = append(connOpts, mach.WithPassword(req.User, req.Password))
		}
	}
	if conn, err := s.db.Connect(pctx, connOpts...); err != nil {
		rsp.Reason = err.Error()
	} else {
		h := s.authServer.GenerateSnowflake()
		parole := &connParole{
			rawConn: conn,
			handle:  h,
			cretime: tick,
		}
		s.pool[h] = parole
		rsp.Conn = &machrpc.ConnHandle{Handle: h}
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}

func (s *grpcd) ConnClose(pctx context.Context, req *machrpc.ConnCloseRequest) (*machrpc.ConnCloseResponse, error) {
	rsp := &machrpc.ConnCloseResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("ConnClose panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	h := req.Conn.Handle
	if parole, ok := s.pool[h]; !ok {
		rsp.Reason = fmt.Sprintf("Conn does not exist %q", h)
	} else {
		if err := parole.rawConn.Close(); err != nil {
			s.log.Warnf("connection %q close error, %s", h, err.Error())
			rsp.Reason = err.Error()
		} else {
			delete(s.pool, h)
			rsp.Success, rsp.Reason = true, "success"
		}
	}
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

	if conn, ok := s.pool[req.Conn.Handle]; !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		explainer, ok := conn.rawConn.(spi.Explainer)
		if !ok {
			return nil, fmt.Errorf("server info is unavailable")
		}
		if plan, err := explainer.Explain(pctx, req.Sql, req.Full); err == nil {
			rsp.Success, rsp.Reason = true, "success"
			rsp.Plan = plan
		} else {
			rsp.Success, rsp.Reason = false, err.Error()
		}
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

	if conn, ok := s.pool[req.Conn.Handle]; !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		params := machrpc.ConvertPbToAny(req.Params)
		if result := conn.rawConn.Exec(pctx, req.Sql, params...); result.Err() == nil {
			rsp.RowsAffected = result.RowsAffected()
			rsp.Success = true
			rsp.Reason = result.Message()
		} else {
			rsp.Success = false
			rsp.Reason = result.Message()
		}
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

	if conn, ok := s.pool[req.Conn.Handle]; !ok {
		rsp.Reason = "invalid connection handle"
		return rsp, nil
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
		return rsp, nil
	} else {
		params := machrpc.ConvertPbToAny(req.Params)
		row := conn.rawConn.QueryRow(pctx, req.Sql, params...)

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
	}

	return rsp, nil
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

	if conn, ok := s.pool[req.Conn.Handle]; !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		params := machrpc.ConvertPbToAny(req.Params)
		realRows, err := conn.rawConn.Query(pctx, req.Sql, params...)
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}

		if realRows.IsFetchable() {
			rows := s.leakDetector.DetainRows(realRows, req.Sql)
			rsp.RowsHandle = &machrpc.RowsHandle{
				Handle: rows.Id(),
			}
			rsp.Reason = "success"
		} else {
			rsp.RowsAffected = realRows.RowsAffected()
			rsp.Reason = realRows.Message()
		}
		rsp.Success = true
	}

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

	rowsWrap, err := s.leakDetector.Rows(rows.Handle)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	cols, err := rowsWrap.Rows.Columns()
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

	rowsWrap, err := s.leakDetector.Rows(rows.Handle)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if !rowsWrap.Rows.Next() {
		rsp.Success = true
		rsp.Reason = "success"
		rsp.HasNoRows = true
		return rsp, nil
	}

	columns, err := rowsWrap.Rows.Columns()
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}

	values := columns.MakeBuffer()
	err = rowsWrap.Rows.Scan(values...)
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

	rowsWrap, err := s.leakDetector.Rows(rows.Handle)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rowsWrap.Release()
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

	if conn, ok := s.pool[req.Conn.Handle]; !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		opts := []spi.AppenderOption{}
		if len(req.Timeformat) > 0 {
			switch conn.rawConn.(type) {
			case *machrpc.ClientConn:
				opts = append(opts, machrpc.AppenderTimeformat(req.Timeformat))
			default:
				opts = append(opts, mach.AppenderTimeformat(req.Timeformat))
			}
		}
		realAppender, err := conn.rawConn.Appender(ctx, req.TableName, opts...)
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		tableType := realAppender.TableType()
		tableName := realAppender.TableName()
		appender := s.leakDetector.DetainAppender(realAppender, tableName)

		rsp.Success = true
		rsp.Reason = "success"
		rsp.Handle = &machrpc.AppenderHandle{Handle: appender.Id()}
		rsp.TableName = tableName
		rsp.TableType = int32(tableType)
	}

	return rsp, nil
}

func (s *grpcd) Append(stream machrpc.Machbase_AppendServer) error {
	var wrap *leak.AppenderParole
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Append panic recover", panic)
		}
		if wrap == nil {
			return
		}
		wrap.Release()
	}()

	tick := time.Now()
	for {
		m, err := stream.Recv()
		if err == io.EOF {
			//
			// Caution: m is nil
			//
			var successCount, failCount int64
			if wrap != nil && wrap.Appender != nil {
				successCount, failCount, _ = wrap.Appender.Close()
				wrap.Release()
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
			wrap, err = s.leakDetector.Appender(m.Handle.Handle)
			if err != nil {
				s.log.Error("handle not found", m.Handle, err.Error())
				return err
			}
		}

		if wrap.Id() != m.Handle.Handle {
			s.log.Error("handle changed", m.Handle)
			return fmt.Errorf("not allowed changing handle in a stream")
		}

		if len(m.Records) > 0 {
			for _, rec := range m.Records {
				values, err := machrpc.ConvertPbTupleToAny(rec.Tuple)
				if err != nil {
					s.log.Error("append-unmarshal", err.Error())
				}
				err = wrap.Appender.Append(values...)
				if err != nil {
					s.log.Error("append", err.Error())
					return err
				}
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

	var passed bool
	var err error

	if strings.HasPrefix(req.Password, "$otp$:") {
		passed, err = s.authServer.ValidateUserOtp(req.LoginName, strings.TrimPrefix(req.Password, "$otp$:"))
	} else {
		passed, _, err = s.authServer.ValidateUserPassword(req.LoginName, req.Password)
	}

	if err != nil {
		rsp.Reason = err.Error()
	} else if !passed {
		rsp.Reason = "invalid username or password"
	} else {
		rsp.Success = passed
		rsp.Reason = "success"
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
	aux, ok := s.db.(spi.DatabaseAux)
	if !ok {
		return nil, fmt.Errorf("server info is unavailable")
	}
	nfo, err := aux.GetServerInfo()
	if err != nil {
		return nil, err
	}

	if req.Inflights {
		items := s.leakDetector.Inflights()
		rsp.Inflights = make([]*machrpc.Inflight, len(items))
		for i := range items {
			rsp.Inflights[i] = &machrpc.Inflight{
				Id:          items[i].Id,
				Type:        items[i].Type,
				SqlText:     items[i].SqlText,
				ElapsedTime: int64(items[i].Elapsed),
			}
		}
	}
	if req.Postflights {
		items := s.leakDetector.Postflights()
		rsp.Postflights = make([]*machrpc.Postflight, len(items))
		for i := range items {
			rsp.Postflights[i] = &machrpc.Postflight{
				SqlText:   items[i].SqlText,
				Count:     items[i].Count,
				TotalTime: int64(items[i].TotalTime),
			}
		}
	}
	if !req.Inflights && !req.Postflights {
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
	}

	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *grpcd) GetServicePorts(pctx context.Context, req *machrpc.ServicePortsRequest) (*machrpc.ServicePorts, error) {
	rsp := &machrpc.ServicePorts{}
	tick := time.Now()

	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("GetServicePorts panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	aux, ok := s.db.(spi.DatabaseAux)
	if !ok {
		return nil, fmt.Errorf("server info is unavailable")
	}

	list, err := aux.GetServicePorts(req.Service)
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		rsp.Ports = append(rsp.Ports, &machrpc.Port{Service: p.Service, Address: p.Address})
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}
