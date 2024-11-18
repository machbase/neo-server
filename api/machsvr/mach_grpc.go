package machsvr

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machrpc"
	cmap "github.com/orcaman/concurrent-map"
	"golang.org/x/exp/rand"
)

type RPCServer struct {
	machrpc.UnimplementedMachbaseServer
	log          *slog.Logger
	db           *Database
	authProvider AuthProvider
	sessions     map[string]*ConnParole
	sessionsLock sync.Mutex
	inflightMap  cmap.ConcurrentMap
	idSerial     int64
}

var _ machrpc.MachbaseServer = (*RPCServer)(nil)

func NewRPCServer(db *Database, opts ...RPCServerOption) *RPCServer {
	s := &RPCServer{
		db:          db,
		sessions:    make(map[string]*ConnParole),
		inflightMap: cmap.New(),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.authProvider == nil {
		s.authProvider = &DefaultAuthProvider{}
	}
	return s
}

type RPCServerOption func(*RPCServer)

func WithLogger(log *slog.Logger) RPCServerOption {
	return func(s *RPCServer) {
		s.log = log
	}
}

type AuthProvider interface {
	ValidateUserOtp(user string, otp string) (bool, error)
	GenerateSnowflake() string
}

func WithAuthProvider(auth AuthProvider) RPCServerOption {
	return func(s *RPCServer) {
		s.authProvider = auth
	}
}

func (s *RPCServer) Ping(ctx context.Context, req *machrpc.PingRequest) (*machrpc.PingResponse, error) {
	tick := time.Now()
	rsp := &machrpc.PingResponse{Token: req.Token}
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Explain panic recover", "error", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if _, err := s.db.Ping(ctx); err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}

func (s *RPCServer) Conn(ctx context.Context, req *machrpc.ConnRequest) (*machrpc.ConnResponse, error) {
	rsp := &machrpc.ConnResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Conn panic recover", "error", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	connOpts := []api.ConnectOption{}
	if strings.HasPrefix(req.Password, "$otp$:") {
		if passed, err := s.authProvider.ValidateUserOtp(req.User, strings.TrimPrefix(req.Password, "$otp$:")); passed {
			connOpts = append(connOpts, api.WithTrustUser(req.User))
		} else if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		} else {
			rsp.Reason = "invalid user or password"
			return rsp, nil
		}
	} else {
		connOpts = append(connOpts, api.WithPassword(req.User, req.Password))
	}
	if conn, err := s.db.Connect(ctx, connOpts...); err != nil {
		rsp.Reason = err.Error()
	} else {
		h := s.authProvider.GenerateSnowflake()
		parole := &ConnParole{
			rawConn: conn.((*Conn)),
			handle:  h,
			creTime: tick,
		}
		s.setSession(h, parole)
		rsp.Conn = &machrpc.ConnHandle{Handle: h}
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil

}

func (s *RPCServer) ConnClose(ctx context.Context, req *machrpc.ConnCloseRequest) (*machrpc.ConnCloseResponse, error) {
	rsp := &machrpc.ConnCloseResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("ConnClose panic recover", "error", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	h := req.Conn.Handle
	if parole, ok := s.getSession(h); !ok {
		rsp.Reason = fmt.Sprintf("Conn does not exist %q", h)
	} else {
		if err := parole.rawConn.Close(); err != nil {
			s.log.Warn("connection close", "handle", h, "error", err.Error())
			rsp.Reason = err.Error()
		} else {
			s.removeSession(h)
			rsp.Success, rsp.Reason = true, "success"
		}
	}
	return rsp, nil
}

func (s *RPCServer) Explain(ctx context.Context, req *machrpc.ExplainRequest) (*machrpc.ExplainResponse, error) {
	rsp := &machrpc.ExplainResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Explain panic recover", "error", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if conn, ok := s.getSession(req.Conn.Handle); !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		if plan, err := conn.rawConn.Explain(ctx, req.Sql, req.Full); err == nil {
			rsp.Success, rsp.Reason = true, "success"
			rsp.Plan = plan
		} else {
			rsp.Success, rsp.Reason = false, err.Error()
		}
	}
	return rsp, nil
}

func (s *RPCServer) Exec(ctx context.Context, req *machrpc.ExecRequest) (*machrpc.ExecResponse, error) {
	rsp := &machrpc.ExecResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Exec panic recover", "error", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	var rawConn *Conn
	if conn, ok := s.getSession(req.Conn.Handle); !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		rawConn = conn.rawConn
	}

	params := machrpc.ConvertPbToAny(req.Params)
	if result := rawConn.Exec(ctx, req.Sql, params...); result.Err() == nil {
		rsp.RowsAffected = result.RowsAffected()
		rsp.Success = true
		rsp.Reason = result.Message()
	} else {
		rsp.Success = false
		rsp.Reason = result.Message()
	}
	return rsp, nil
}

func (s *RPCServer) QueryRow(ctx context.Context, req *machrpc.QueryRowRequest) (*machrpc.QueryRowResponse, error) {
	rsp := &machrpc.QueryRowResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("QueryRow panic recover", "error", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	var rawConn *Conn
	if conn, ok := s.getSession(req.Conn.Handle); !ok {
		rsp.Reason = "invalid connection handle"
		return rsp, nil
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
		return rsp, nil
	} else {
		rawConn = conn.rawConn
	}

	params := machrpc.ConvertPbToAny(req.Params)
	row := rawConn.QueryRow(ctx, req.Sql, params...)

	if row.Err() != nil {
		rsp.Reason = row.Err().Error()
		return rsp, nil
	}

	var err error
	rsp.Success = true
	rsp.Reason = row.Message()
	rsp.Values, err = machrpc.ConvertAnyToPb(row.(*Row).values)
	rsp.RowsAffected = row.RowsAffected()
	if columns, err := row.Columns(); err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
	} else {
		for _, c := range columns {
			rsp.Columns = append(rsp.Columns, &machrpc.Column{
				Name:     c.Name,
				DataType: string(c.DataType),
				Length:   int32(c.Length),
				Type:     int32(c.Type),
				Flag:     int32(c.Flag),
			})
		}
	}
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
	}
	return rsp, nil
}

func (s *RPCServer) Query(ctx context.Context, req *machrpc.QueryRequest) (*machrpc.QueryResponse, error) {
	rsp := &machrpc.QueryResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Query panic recover", "error", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	var rawConn *Conn
	if conn, ok := s.getSession(req.Conn.Handle); !ok {
		rsp.Reason = "invalid connection handle"
	} else if conn.rawConn == nil {
		rsp.Reason = "invalid connection"
	} else {
		rawConn = conn.rawConn
	}
	params := machrpc.ConvertPbToAny(req.Params)
	realRows, err := rawConn.Query(ctx, req.Sql, params...)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if realRows.IsFetchable() {
		rows := s.detainRows(realRows.(*Rows), req.Sql)
		rsp.RowsHandle = &machrpc.RowsHandle{
			Handle: rows.Id(),
		}
		rsp.Reason = "success"
	} else {
		rsp.RowsAffected = realRows.RowsAffected()
		rsp.Reason = realRows.Message()
	}
	rsp.Success = true
	return rsp, nil
}

func (s *RPCServer) Columns(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.ColumnsResponse, error) {
	rsp := &machrpc.ColumnsResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Columns panic recover", "error", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	rowsWrap, err := s.getRows(rows.Handle)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Columns = make([]*machrpc.Column, len(rowsWrap.Rows.columns))
	for i, c := range rowsWrap.Rows.columns {
		rsp.Columns[i] = &machrpc.Column{
			Name:     c.Name,
			DataType: string(c.DataType),
			Length:   int32(c.Length),
			Type:     int32(c.Type),
			Flag:     int32(c.Flag),
		}
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *RPCServer) RowsFetch(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.RowsFetchResponse, error) {
	rsp := &machrpc.RowsFetchResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			debug.PrintStack()
			s.log.Error("RowsFetch panic recover", "error", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	rowsWrap, err := s.getRows(rows.Handle)
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

	values, err := rowsWrap.Rows.columns.MakeBuffer()
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}
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

func (s *RPCServer) RowsClose(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.RowsCloseResponse, error) {
	rsp := &machrpc.RowsCloseResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("RowsClose panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	rowsWrap, err := s.getRows(rows.Handle)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rowsWrap.Release()
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *RPCServer) Appender(ctx context.Context, req *machrpc.AppenderRequest) (*machrpc.AppenderResponse, error) {
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
		opts := []api.AppenderOption{}
		realAppender, err := conn.rawConn.Appender(ctx, req.TableName, opts...)
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		tableType := realAppender.(*Appender).TableType()
		tableName := realAppender.TableName()

		if columns, err := realAppender.Columns(); err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		} else {
			rsp.Columns = make([]*machrpc.Column, len(columns))
			for i, c := range columns {
				rsp.Columns[i] = &machrpc.Column{
					Name:     c.Name,
					DataType: string(c.DataType),
					Length:   int32(c.Length),
					Type:     int32(c.Type),
					Flag:     int32(c.Flag),
				}
			}
		}

		appender := s.detainAppender(realAppender.(*Appender), tableName)

		rsp.Success = true
		rsp.Reason = "success"
		rsp.Handle = &machrpc.AppenderHandle{Handle: appender.Id()}
		rsp.TableName = tableName
		rsp.TableType = int32(tableType)
	}

	return rsp, nil
}

func (s *RPCServer) Append(stream machrpc.Machbase_AppendServer) error {
	var wrap *AppenderParole
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
			wrap, err = s.getAppender(m.Handle.Handle)
			if err != nil {
				s.log.Error("handle not found", "handle", m.Handle, "error", err.Error())
				return err
			}
		}

		if wrap.Id() != m.Handle.Handle {
			s.log.Error("handle changed", "handle", m.Handle)
			return fmt.Errorf("not allowed changing handle in a stream")
		}

		if len(m.Records) > 0 {
			for _, rec := range m.Records {
				values, err := machrpc.ConvertPbTupleToAny(rec.Tuple)
				if err != nil {
					s.log.Error("append-unmarshal", "error", err.Error())
				}
				err = wrap.Appender.Append(values...)
				if err != nil {
					s.log.Error("append", "error", err.Error())
					return err
				}
			}
		}
	}
}

func (s *RPCServer) UserAuth(ctx context.Context, req *machrpc.UserAuthRequest) (*machrpc.UserAuthResponse, error) {
	rsp := &machrpc.UserAuthResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("UserAuth panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	var passed bool
	var reason string
	var err error

	if strings.HasPrefix(req.Password, "$otp$:") {
		passed, err = s.authProvider.ValidateUserOtp(req.LoginName, strings.TrimPrefix(req.Password, "$otp$:"))
	} else {
		passed, reason, err = s.db.UserAuth(ctx, req.LoginName, req.Password)
	}

	if err != nil {
		rsp.Reason = err.Error()
	} else if !passed {
		rsp.Reason = reason
	} else {
		rsp.Success = passed
		rsp.Reason = "success"
	}
	return rsp, nil
}

func (s *RPCServer) getRows(id string) (*RowsParole, error) {
	value, exists := s.inflightMap.Get(id)
	if !exists {
		return nil, fmt.Errorf("handle '%s' not found", id)
	}
	ret, ok := value.(*RowsParole)
	if !ok {
		return nil, fmt.Errorf("handle '%s' is not valid", id)
	}
	ret.lastAccessTime = time.Now()
	return ret, nil
}

func (s *RPCServer) detainRows(rows *Rows, sqlText string) *RowsParole {
	ser := atomic.AddInt64(&s.idSerial, 1)
	key := fmt.Sprintf("%p#%d", rows, ser)
	return s.detainRows0(key, rows, sqlText)
}

func (s *RPCServer) detainRows0(key string, rows *Rows, sqlText string) *RowsParole {
	ret := &RowsParole{
		Rows:       rows,
		id:         key,
		sqlText:    sqlText,
		createTime: time.Now(),
	}
	ret.lastAccessTime = ret.createTime
	ret.release = func() {
		s.inflightMap.RemoveCb(ret.id, func(key string, v any, exists bool) bool {
			if ret.Rows != nil {
				err := ret.Rows.Close()
				if err != nil {
					s.log.Warn("error on rows.close; %s,", "error", err.Error(), "statement", ret.String())
				}
				ret.releaseTime = time.Now()
			}
			return true
		})
	}
	s.inflightMap.Set(key, ret)
	return ret
}

func (s *RPCServer) getAppender(id string) (*AppenderParole, error) {
	value, exists := s.inflightMap.Get(id)
	if !exists {
		return nil, fmt.Errorf("handle '%s' not found", id)
	}
	ret, ok := value.(*AppenderParole)
	if !ok {
		return nil, fmt.Errorf("handle '%s' is not valid", id)
	}
	return ret, nil
}

func (s *RPCServer) detainAppender(appender *Appender, tableName string) *AppenderParole {
	ser := atomic.AddInt64(&s.idSerial, 1)
	key := fmt.Sprintf("%p#%d", appender, ser)
	return s.detainAppender0(key, appender, tableName)
}

func (s *RPCServer) detainAppender0(key string, appender *Appender, tableName string) *AppenderParole {
	ret := &AppenderParole{
		Appender:   appender,
		id:         key,
		tableName:  tableName,
		createTime: time.Now(),
	}
	ret.release = func() {
		s.inflightMap.RemoveCb(ret.id, func(key string, v any, exists bool) bool {
			if ret.Appender != nil {
				success, fail, err := ret.Appender.Close()
				if err != nil {
					s.log.Warn("close APND", "id", ret.id, "success", success, "fail", fail, "error", err.Error())
				} else {
					if fail == 0 {
						s.log.Debug("close APND", "id", ret.id, "success", success)
					} else {
						s.log.Debug("close APND", "id", ret.id, "success", success, "fail", fail)
					}
				}
				ret.releaseTime = time.Now()
			}
			return true
		})
	}
	s.inflightMap.Set(key, ret)
	return ret
}

type ConnParole struct {
	rawConn *Conn
	handle  string
	creTime time.Time
}

func (svr *RPCServer) getSession(handle string) (*ConnParole, bool) {
	svr.sessionsLock.Lock()
	ret, ok := svr.sessions[handle]
	svr.sessionsLock.Unlock()
	return ret, ok
}

func (svr *RPCServer) setSession(handle string, conn *ConnParole) {
	svr.sessionsLock.Lock()
	svr.sessions[handle] = conn
	svr.sessionsLock.Unlock()
}

func (svr *RPCServer) removeSession(handle string) {
	svr.sessionsLock.Lock()
	delete(svr.sessions, handle)
	svr.sessionsLock.Unlock()
}

type RowsParole struct {
	Rows        *Rows
	id          string
	release     func()
	releaseOnce sync.Once
	sqlText     string

	createTime     time.Time
	lastAccessTime time.Time
	releaseTime    time.Time
}

func (rp *RowsParole) String() string {
	return fmt.Sprintf("ROWS %s %s %s", rp.id, time.Since(rp.createTime).String(), rp.sqlText)
}

func (rp *RowsParole) Id() string {
	return rp.id
}

func (rp *RowsParole) Release() {
	if rp.release != nil {
		rp.releaseOnce.Do(rp.release)
	}
}

type AppenderParole struct {
	Appender    *Appender
	id          string
	release     func()
	releaseOnce sync.Once
	tableName   string
	createTime  time.Time
	releaseTime time.Time
}

func (ap *AppenderParole) String() string {
	return fmt.Sprintf("APPEND %s %s %s", ap.id, time.Since(ap.createTime), ap.tableName)
}

func (ap *AppenderParole) Id() string {
	return ap.id
}

func (ap *AppenderParole) Release() {
	if ap.release != nil {
		ap.releaseOnce.Do(ap.release)
	}
}

type DefaultAuthProvider struct {
}

var _ AuthProvider = (*DefaultAuthProvider)(nil)

func (dap *DefaultAuthProvider) ValidateUserOtp(user string, otp string) (bool, error) {
	return false, nil
}

func (dap *DefaultAuthProvider) GenerateSnowflake() string {
	r := rand.Float64()
	return fmt.Sprintf("%x", math.Float64bits(r))
}
