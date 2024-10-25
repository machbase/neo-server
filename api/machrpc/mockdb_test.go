package machrpc_test

import (
	context "context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/machrpc"
	"google.golang.org/grpc"
)

type MockServer struct {
	machrpc.MachbaseServer
	svr *grpc.Server

	counter   int32
	conns     map[string]*MockConn
	rows      map[string]*MockRows
	appenders map[string]*MockAppender
}

type MockConn struct {
}

type MockRows struct {
	nrow int
}

type MockAppender struct {
	table string
	nrow  int
}

var _ machrpc.MachbaseServer = &MockServer{}

var MockServerAddr = "127.0.0.1:5655"

func (ms *MockServer) Start() error {
	svrOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(1 * 1024 * 1024 * 1024)),
		grpc.MaxSendMsgSize(int(1 * 1024 * 1024 * 1024)),
	}
	ms.svr = grpc.NewServer(svrOptions...)
	machrpc.RegisterMachbaseServer(ms.svr, ms)
	lsnr, err := net.Listen("tcp", "127.0.0.1:0")
	MockServerAddr = lsnr.Addr().String()
	if err != nil {
		return err
	}
	ms.conns = map[string]*MockConn{}
	ms.rows = map[string]*MockRows{}
	ms.appenders = map[string]*MockAppender{}
	go ms.svr.Serve(lsnr)
	return nil
}

func (ms *MockServer) Stop() {
	ms.svr.Stop()

	if len(ms.conns) != 0 {
		panic(fmt.Errorf("WARN!!!! connection leak!!! There are %d sessions remained", len(ms.conns)))
	}
	if len(ms.rows) != 0 {
		panic(fmt.Errorf("WARN!!!! rows leak!!! There are %d rows remained", len(ms.rows)))
	}
	// if len(ms.appenders) != 0 {
	// 	panic(fmt.Errorf("WARN!!!! appenders leak!!! There are %d appenders remained", len(ms.appenders)))
	// }
}

func (ms *MockServer) UserAuth(ctx context.Context, req *machrpc.UserAuthRequest) (*machrpc.UserAuthResponse, error) {
	auth := true
	reason := "success"
	if req.LoginName != "sys" || req.Password != "manager" {
		auth = false
		reason = "invalid username or password"
	}
	return &machrpc.UserAuthResponse{
		Success: auth,
		Reason:  reason,
		Elapse:  "1ms.",
	}, nil
}

func (ms *MockServer) Conn(ctx context.Context, req *machrpc.ConnRequest) (*machrpc.ConnResponse, error) {
	if req.User != "sys" || req.Password != "manager" {
		return &machrpc.ConnResponse{
			Success: false,
			Reason:  "invalid username or password",
			Elapse:  "1ms.",
		}, nil
	}
	connId := atomic.AddInt32(&ms.counter, 1)
	connHandle := fmt.Sprintf("conn#%d", connId)
	ms.conns[connHandle] = &MockConn{}
	return &machrpc.ConnResponse{
		Success: true,
		Reason:  "success",
		Elapse:  "1ms.",
		Conn:    &machrpc.ConnHandle{Handle: connHandle},
	}, nil
}

func (ms *MockServer) ConnClose(ctx context.Context, req *machrpc.ConnCloseRequest) (*machrpc.ConnCloseResponse, error) {
	delete(ms.conns, req.Conn.Handle)
	return &machrpc.ConnCloseResponse{
		Success: true,
		Reason:  "success",
		Elapse:  "1ms.",
	}, nil
}

func (ms *MockServer) Ping(ctx context.Context, req *machrpc.PingRequest) (*machrpc.PingResponse, error) {
	return &machrpc.PingResponse{
		Success: true,
		Reason:  "success",
		Elapse:  "1ms.",
		Token:   req.Token,
	}, nil
}

func (ms *MockServer) Explain(ctx context.Context, req *machrpc.ExplainRequest) (*machrpc.ExplainResponse, error) {
	ret := &machrpc.ExplainResponse{Success: true, Reason: "success", Elapse: "1ms."}
	_, ok := ms.conns[req.Conn.Handle]
	if !ok {
		ret.Success, ret.Reason = false, "invalid connection"
		return ret, nil
	}

	switch req.Sql {
	case `select * from dummy`:
		ret.Plan = "explain dummy result"
	default:
		ret.Success, ret.Reason = false, "unknown test case"
	}
	return ret, nil
}

func (ms *MockServer) Exec(ctx context.Context, req *machrpc.ExecRequest) (*machrpc.ExecResponse, error) {
	ret := &machrpc.ExecResponse{Success: true, Reason: "success", Elapse: "1ms."}
	_, ok := ms.conns[req.Conn.Handle]
	if !ok {
		ret.Success, ret.Reason = false, "invalid connection"
		return ret, nil
	}
	switch req.Sql {
	case `insert into example (name, time, value) values(?, ?, ?)`:
		ret.RowsAffected = 1
		ret.Reason = "a row inserted."
	default:
		ret.Success, ret.Reason = false, "unknown test case"
	}
	return ret, nil
}

func (ms *MockServer) QueryRow(ctx context.Context, req *machrpc.QueryRowRequest) (*machrpc.QueryRowResponse, error) {
	ret := &machrpc.QueryRowResponse{Success: true, Reason: "success", Elapse: "1ms."}
	_, ok := ms.conns[req.Conn.Handle]
	if !ok {
		ret.Success, ret.Reason = false, "invalid connection"
		return ret, nil
	}
	switch req.Sql {
	case `select count(*) from example where name = ?`:
		ret.Values, _ = machrpc.ConvertAnyToPb([]any{int64(123)})
		ret.Reason = "a row selected."
		ret.RowsAffected = 1
	default:
		ret.Success, ret.Reason = false, "unknown test case"
	}
	return ret, nil
}

func (ms *MockServer) Query(ctx context.Context, req *machrpc.QueryRequest) (*machrpc.QueryResponse, error) {
	ret := &machrpc.QueryResponse{Success: true, Reason: "success", Elapse: "1ms."}
	_, ok := ms.conns[req.Conn.Handle]
	if !ok {
		ret.Success, ret.Reason = false, "invalid connection"
		return ret, nil
	}
	params := machrpc.ConvertPbToAny(req.Params)
	switch req.Sql {
	case `select * from example where name = ?`:
		ret.RowsHandle = &machrpc.RowsHandle{}
		if len(params) == 1 && params[0] == "query1" {
			ret.RowsHandle = &machrpc.RowsHandle{
				Handle: "query1#1",
				Conn:   &machrpc.ConnHandle{Handle: req.Conn.Handle},
			}
			ret.RowsAffected = 0
			ms.rows[ret.RowsHandle.Handle] = &MockRows{}
		} else {
			ret.Success, ret.Reason = false, fmt.Sprintf("not implemented %+v", params)
		}
	default:
		ret.Success, ret.Reason = false, "unknown test case"
	}
	return ret, nil
}

func (ms *MockServer) Columns(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.ColumnsResponse, error) {
	ret := &machrpc.ColumnsResponse{Success: true, Reason: "success", Elapse: "1ms."}
	switch rows.Handle {
	case "query1#1":
		ret.Columns = []*machrpc.Column{
			{Name: "name", DataType: api.ColumnTypeVarchar.String(), Length: 40},
			{Name: "time", DataType: api.ColumnTypeDatetime.String(), Length: 8},
			{Name: "value", DataType: api.ColumnTypeDouble.String(), Length: 8},
		}
	default:
		ret.Success, ret.Reason = false, "unknown test case"
	}
	return ret, nil
}

func (ms *MockServer) RowsFetch(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.RowsFetchResponse, error) {
	mockRows, ok := ms.rows[rows.Handle]
	if !ok {
		return &machrpc.RowsFetchResponse{Success: false, Reason: "invalid rows handle", Elapse: "1ms."}, nil
	}

	ret := &machrpc.RowsFetchResponse{Success: true, Reason: "success", Elapse: "1ms."}

	var err error
	switch rows.Handle {
	case "query1#1":
		mockRows.nrow++
		if mockRows.nrow == 1 {
			ret.Values, err = machrpc.ConvertAnyToPb([]any{"tag", time.Unix(0, 1), 3.14})
		} else {
			ret.HasNoRows = true
		}
	}
	return ret, err
}

func (ms *MockServer) RowsClose(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.RowsCloseResponse, error) {
	if _, ok := ms.rows[rows.Handle]; !ok {
		return &machrpc.RowsCloseResponse{Success: false, Reason: "invalid rows handle", Elapse: "1ms."}, nil
	}
	delete(ms.rows, rows.Handle)
	return &machrpc.RowsCloseResponse{
		Success: true,
		Reason:  "success",
		Elapse:  "1ms.",
	}, nil
}

func (ms *MockServer) Appender(ctx context.Context, req *machrpc.AppenderRequest) (*machrpc.AppenderResponse, error) {
	if _, ok := ms.conns[req.Conn.Handle]; !ok {
		return &machrpc.AppenderResponse{Success: false, Reason: "invalid connection", Elapse: "1ms."}, nil
	}

	appenderId := atomic.AddInt32(&ms.counter, 1)
	appenderHandle := fmt.Sprintf("appender#%d", appenderId)
	appender := &MockAppender{
		table: req.TableName,
		nrow:  0,
	}
	ms.appenders[appenderHandle] = appender
	return &machrpc.AppenderResponse{
		Success: true,
		Reason:  "success",
		Elapse:  "1ms.",
		Handle: &machrpc.AppenderHandle{
			Handle: appenderHandle,
			Conn:   req.Conn,
		},
		TableName: strings.ToUpper(req.TableName),
	}, nil
}

func (ms *MockServer) Append(stream machrpc.Machbase_AppendServer) error {
	tick := time.Now()
	successCount := int64(0)
	failCount := int64(0)
	for {
		rec, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&machrpc.AppendDone{
					Success:      true,
					Reason:       "success",
					Elapse:       time.Since(tick).String(),
					SuccessCount: successCount,
					FailCount:    failCount,
				})
			}
			return err
		}
		successCount += int64(len(rec.Records))
	}
}
