package machsvr_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machrpc"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/stretchr/testify/require"
)

var rpcClient machrpc.MachbaseClient
var machsvrDB api.Database
var machrpcDB api.Database

func TestMain(m *testing.M) {
	s := testsuite.NewServer("./testsuite_tmp")
	s.StartServer(m)
	rpcClient = machrpc.NewMachbaseClient(s.ClientConn())
	machsvrDB = s.DatabaseSVR()
	machrpcDB = s.DatabaseRPC()

	code := m.Run()

	s.StopServer(m)
	os.Exit(code)
}

func TestAll(t *testing.T) {
	testsuite.CreateTestTables(machsvrDB)
	testsuite.TestAll(t, machsvrDB)
	testsuite.DropTestTables(machsvrDB)

	testsuite.CreateTestTables(machsvrDB)
	testsuite.TestAll(t, machrpcDB,
		tcRpcPing,
		tcRpcUserAuth,
		tcRpcExplain,
		tcRpcExec,
	)
	testsuite.DropTestTables(machsvrDB)
}

func connectRpc(t *testing.T, ctx context.Context) *machrpc.ConnHandle {
	t.Helper()
	connRsp, err := rpcClient.Conn(ctx, &machrpc.ConnRequest{
		User:     "sys",
		Password: "manager",
	})
	require.NoError(t, err)
	require.True(t, connRsp.Success)
	require.NotNil(t, connRsp.Conn)
	return connRsp.Conn
}

func disconnectRpc(t *testing.T, ctx context.Context, handle *machrpc.ConnHandle) {
	t.Helper()
	closeRsp, err := rpcClient.ConnClose(ctx, &machrpc.ConnCloseRequest{
		Conn: handle,
	})
	require.NoError(t, err)
	require.True(t, closeRsp.Success)
}

func tcRpcPing(t *testing.T) {
	ctx := context.TODO()
	rsp, err := rpcClient.Ping(ctx, &machrpc.PingRequest{
		Token: 1234567890,
	})
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.True(t, rsp.Success)
	require.Equal(t, int64(1234567890), rsp.Token)
}

func tcRpcUserAuth(t *testing.T) {
	ctx := context.TODO()
	rsp, err := rpcClient.UserAuth(ctx, &machrpc.UserAuthRequest{
		LoginName: "sys",
		Password:  "manager",
	})
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.True(t, rsp.Success)
	require.Equal(t, "success", rsp.Reason)

	rsp, err = rpcClient.UserAuth(ctx, &machrpc.UserAuthRequest{
		LoginName: "sys",
		Password:  "wrong",
	})
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.False(t, rsp.Success)
	require.Equal(t, "invalid username or password", rsp.Reason)
}

func tcRpcExplain(t *testing.T) {
	ctx := context.TODO()
	conn := connectRpc(t, ctx)
	defer disconnectRpc(t, ctx, conn)

	rsp, err := rpcClient.Explain(ctx, &machrpc.ExplainRequest{
		Conn: conn,
		Sql:  "select * from tag_data order by time desc",
		Full: false,
	})
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.True(t, rsp.Success)
	require.NotEmpty(t, rsp.Plan)
}

func tcRpcExec(t *testing.T) {
	ctx := context.TODO()
	conn := connectRpc(t, ctx)

	execRsp, err := rpcClient.Exec(ctx, &machrpc.ExecRequest{
		Conn: conn,
		Sql:  "create tag table test_tag (name varchar(100) primary key, time datetime basetime, value double)",
	})
	require.NoError(t, err)
	require.NotNil(t, execRsp)
	require.True(t, execRsp.Success, execRsp.Reason)
	require.NotEmpty(t, execRsp.Reason)

	// Exec("insert")
	now := time.Now().In(time.UTC)
	execReq := &machrpc.ExecRequest{
		Conn: conn,
		Sql:  "insert into test_tag values (?, ?, ?)",
	}
	if params, err := machrpc.ConvertAnyToPb([]any{"test", now, 123.456}); err != nil {
		t.Fatal(err)
	} else {
		execReq.Params = params
	}
	execRsp, err = rpcClient.Exec(ctx, execReq)
	require.NoError(t, err)
	require.NotNil(t, execRsp)
	require.True(t, execRsp.Success, execRsp.Reason)
	require.Equal(t, int64(1), execRsp.RowsAffected)

	execReq = &machrpc.ExecRequest{
		Conn: conn,
		Sql:  "exec table_flush(test_tag)",
	}
	execRsp, err = rpcClient.Exec(ctx, execReq)
	require.NoError(t, err)
	require.NotNil(t, execRsp)
	require.True(t, execRsp.Success, execRsp.Reason)

	// QueryRow("select")
	queryRowReq := &machrpc.QueryRowRequest{
		Conn: conn,
		Sql:  "select name, time, value from test_tag where name = ?",
	}
	if params, err := machrpc.ConvertAnyToPb([]any{"test"}); err != nil {
		t.Fatal(err)
	} else {
		queryRowReq.Params = params
	}
	queryRowRsp, err := rpcClient.QueryRow(ctx, queryRowReq)
	require.NoError(t, err)
	require.NotNil(t, queryRowRsp)
	require.True(t, queryRowRsp.Success, queryRowRsp.Reason)
	require.NotEmpty(t, queryRowRsp.Reason)
	require.Len(t, queryRowRsp.Values, 3, queryRowRsp.Reason)
	queryRowValues := machrpc.ConvertPbToAny(queryRowRsp.Values)
	require.Equal(t, "test", queryRowValues[0])
	require.Equal(t, now, queryRowValues[1])
	require.Equal(t, 123.456, queryRowValues[2])
	expectColumns := []*machrpc.Column{
		{Name: "NAME", DataType: "string", Length: 100},
		{Name: "TIME", DataType: "datetime", Length: 8},
		{Name: "VALUE", DataType: "double", Length: 8},
	}
	require.Equal(t, "a row selected.", queryRowRsp.Reason)
	require.Equal(t, len(expectColumns), len(queryRowRsp.Columns))
	for i, col := range queryRowRsp.Columns {
		require.Equal(t, expectColumns[i].Name, col.Name)
		require.Equal(t, expectColumns[i].DataType, col.DataType, "diff column: "+col.Name)
		require.Equal(t, expectColumns[i].Length, col.Length, "diff column: "+col.Name)
		require.Equal(t, expectColumns[i].Flag, col.Flag, "diff column: "+col.Name)
		require.Equal(t, expectColumns[i].Type, col.Type, "diff column: "+col.Name)
	}

	// Append Open
	appendConn := connectRpc(t, ctx)
	appendRsp, err := rpcClient.Appender(ctx, &machrpc.AppenderRequest{
		Conn:      conn,
		TableName: "test_tag",
	})
	require.NoError(t, err)
	require.True(t, appendRsp.Success, appendRsp.Reason)

	appendClient, err := rpcClient.Append(ctx)
	require.NoError(t, err)
	require.NotNil(t, appendClient)

	// Append
	for i := 0; i < 100; i++ {
		values, err := machrpc.ConvertAnyToPbTuple([]any{"test", now.Add(time.Duration(i)), 123.456})
		require.NoError(t, err)
		appendClient.Send(&machrpc.AppendData{
			Handle:  appendRsp.Handle,
			Records: []*machrpc.AppendRecord{{Tuple: values}},
		})
	}

	// Append Close
	appendDone, err := appendClient.CloseAndRecv()
	require.NoError(t, err)
	require.NotNil(t, appendDone)
	require.Equal(t, int64(100), appendDone.SuccessCount)
	require.Equal(t, int64(0), appendDone.FailCount)
	disconnectRpc(t, ctx, appendConn)

	// Query()
	queryReq := &machrpc.QueryRequest{
		Conn: conn,
		Sql:  "select name, time, value from test_tag where name = ?",
	}
	if params, err := machrpc.ConvertAnyToPb([]any{"test"}); err != nil {
		t.Fatal(err)
	} else {
		queryReq.Params = params
	}
	queryRsp, err := rpcClient.Query(ctx, queryReq)
	require.NoError(t, err)
	require.NotNil(t, queryRsp)
	require.True(t, queryRsp.Success, queryRsp.Reason)
	require.NotEmpty(t, queryRsp.Reason)

	// Columns
	colsRsp, err := rpcClient.Columns(ctx, queryRsp.RowsHandle)
	require.NoError(t, err)
	require.NotNil(t, colsRsp)

	expectColumns = []*machrpc.Column{
		{Name: "NAME", DataType: "string", Length: 0},
		{Name: "TIME", DataType: "datetime", Length: 0},
		{Name: "VALUE", DataType: "double", Length: 0},
	}
	require.Equal(t, len(expectColumns), len(colsRsp.Columns))

	for i, col := range colsRsp.Columns {
		require.Equal(t, col.Name, colsRsp.Columns[i].Name)
		require.Equal(t, col.DataType, colsRsp.Columns[i].DataType)
		require.Equal(t, col.Length, colsRsp.Columns[i].Length)
		require.Equal(t, col.Flag, colsRsp.Columns[i].Flag)
		require.Equal(t, col.Type, colsRsp.Columns[i].Type)
	}

	// RowsFetch
	for {
		fetchRsp, err := rpcClient.RowsFetch(ctx, queryRsp.RowsHandle)
		require.NoError(t, err)
		require.NotNil(t, fetchRsp)
		require.True(t, fetchRsp.Success, fetchRsp.Reason)
		require.NotEmpty(t, fetchRsp.Reason)

		values := machrpc.ConvertPbToAny(queryRowRsp.Values)
		require.Equal(t, "test", values[0])
		require.Equal(t, now, values[1])
		require.Equal(t, 123.456, values[2])

		if fetchRsp.HasNoRows {
			break
		}
	}

	// RowsClose
	closeRsp, err := rpcClient.RowsClose(ctx, queryRsp.RowsHandle)
	require.NoError(t, err)
	require.NotNil(t, closeRsp)
	require.True(t, closeRsp.Success, closeRsp.Reason)

	execReq = &machrpc.ExecRequest{
		Conn: conn,
		Sql:  "exec table_flush(test_tag)",
	}
	execRsp, err = rpcClient.Exec(ctx, execReq)
	require.NoError(t, err)
	require.NotNil(t, execRsp)
	require.True(t, execRsp.Success, execRsp.Reason)

	execRsp, err = rpcClient.Exec(ctx, &machrpc.ExecRequest{
		Conn: conn,
		Sql:  "drop table test_tag",
	})
	require.NoError(t, err)
	require.NotNil(t, execRsp)
	require.True(t, execRsp.Success)
	require.NotEmpty(t, execRsp.Reason)

}
