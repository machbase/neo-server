package grpcd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-client/machrpc"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/leak"
)

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
	if conn, ok := s.getSession(req.Conn.Handle); !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		if _, err := conn.rawConn.Ping(); err != nil {
			rsp.Reason = err.Error()
		} else {
			rsp.Success, rsp.Reason = true, "success"
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
	connOpts := []mach.ConnectOption{}
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
	if conn, err := s.db.Connect(pctx, connOpts...); err != nil {
		rsp.Reason = err.Error()
	} else {
		h := s.authServer.GenerateSnowflake()
		parole := &connParole{
			rawConn: conn,
			handle:  h,
			cretime: tick,
		}
		s.setSession(h, parole)
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
	if parole, ok := s.getSession(h); !ok {
		rsp.Reason = fmt.Sprintf("Conn does not exist %q", h)
	} else {
		if err := parole.rawConn.Close(); err != nil {
			s.log.Warnf("connection %q close error, %s", h, err.Error())
			rsp.Reason = err.Error()
		} else {
			s.removeSession(h)
			rsp.Success, rsp.Reason = true, "success"
		}
	}
	return rsp, nil
}

type Explainer interface {
	Explain(ctx context.Context, sqlText string, full bool) (string, error)
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

	if conn, ok := s.getSession(req.Conn.Handle); !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		if plan, err := conn.rawConn.Explain(pctx, req.Sql, req.Full); err == nil {
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

	if conn, ok := s.getSession(req.Conn.Handle); !ok {
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

	if conn, ok := s.getSession(req.Conn.Handle); !ok {
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

	if conn, ok := s.getSession(req.Conn.Handle); !ok {
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

	cols, err := api.RowsColumns(rowsWrap.Rows)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Columns = make([]*machrpc.Column, len(cols))
	for i, c := range cols {
		rsp.Columns[i] = &machrpc.Column{
			Name: c.Name,
			Type: c.Type,
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

	columns, err := api.RowsColumns(rowsWrap.Rows)
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}

	values := api.MakeBuffer(columns)
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

	if conn, ok := s.getSession(req.Conn.Handle); !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		opts := []mach.AppenderOption{}
		if len(req.Timeformat) > 0 {
			opts = append(opts, mach.AppenderTimeformat(req.Timeformat))
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
	tick := time.Now()
	rsp, err := s.serverInfoFunc()
	if err != nil {
		return nil, err
	}
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("GetServerInfo panic recover", panic)
		}
		if rsp != nil {
			rsp.Elapse = time.Since(tick).String()
		}
	}()
	if s.serverInfoFunc == nil {
		return nil, fmt.Errorf("server info is unavailable (%T)", s.db)
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

	if s.servicePortsFunc == nil {
		return nil, fmt.Errorf("server info is unavailable")
	}

	list, err := s.servicePortsFunc(req.Service)
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

func (s *grpcd) Sessions(pctx context.Context, req *machrpc.SessionsRequest) (*machrpc.SessionsResponse, error) {
	rsp := &machrpc.SessionsResponse{}
	tick := time.Now()

	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Sessions panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if s.serverSessionsFunc == nil {
		return nil, fmt.Errorf("session info is unavailable")
	}

	if statz, sessions, err := s.serverSessionsFunc(req.Statz, req.Sessions); err != nil {
		rsp.Reason = err.Error()
		return nil, err
	} else {
		rsp.Success = true
		rsp.Reason = "success"
		rsp.Statz = statz
		rsp.Sessions = sessions
	}

	return rsp, nil
}

func (s *grpcd) KillSession(pctx context.Context, req *machrpc.KillSessionRequest) (*machrpc.KillSessionResponse, error) {
	rsp := &machrpc.KillSessionResponse{}
	tick := time.Now()

	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Sessions kill panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if s.serverKillSessionFunc == nil {
		return nil, fmt.Errorf("session kill is unavailable")
	}

	if err := s.serverKillSessionFunc(req.Id); err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success = true
		rsp.Reason = "success"
	}
	return rsp, nil
}
