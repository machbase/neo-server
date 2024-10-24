package machsvr_test

import (
	"context"
	"testing"
	"time"

	"github.com/machbase/neo-server/api/machrpc"
	"github.com/stretchr/testify/require"
)

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

func TestRpcPing(t *testing.T) {
	ctx := context.TODO()
	conn := connectRpc(t, ctx)
	defer disconnectRpc(t, ctx, conn)

	rsp, err := rpcClient.Ping(ctx, &machrpc.PingRequest{
		Conn:  conn,
		Token: 1234567890,
	})
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.True(t, rsp.Success)
	require.Equal(t, int64(1234567890), rsp.Token)
}

func TestRpcUserAuth(t *testing.T) {
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

func TestRpcExplain(t *testing.T) {
	ctx := context.TODO()
	conn := connectRpc(t, ctx)
	defer disconnectRpc(t, ctx, conn)

	rsp, err := rpcClient.Explain(ctx, &machrpc.ExplainRequest{
		Conn: conn,
		Sql:  "select * from complex_tag order by time desc",
		Full: false,
	})
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.True(t, rsp.Success)
	require.NotEmpty(t, rsp.Plan)
}

func TestRpcExec(t *testing.T) {
	ctx := context.TODO()
	conn := connectRpc(t, ctx)
	defer disconnectRpc(t, ctx, conn)

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
	t.Log("Exec(insert)", execRsp.Reason, execRsp.RowsAffected)

	// QueryRow("select")
	time.Sleep(100 * time.Millisecond)
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
	require.NotEmpty(t, queryRowRsp.Message)
	require.Len(t, queryRowRsp.Values, 3, queryRowRsp.Reason)
	queryRowValues := machrpc.ConvertPbToAny(queryRowRsp.Values)
	require.Equal(t, "test", queryRowValues[0])
	require.Equal(t, now, queryRowValues[1])
	require.Equal(t, 123.456, queryRowValues[2])
	// FIXME: queryRowRsp.Message should be 'a row selected' instead of 'no row selected'
	t.Log("QueryRow(select)", queryRowRsp.Message, queryRowRsp.Values)

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

	colsRsp, err := rpcClient.Columns(ctx, queryRsp.RowsHandle)
	require.NoError(t, err)
	require.NotNil(t, colsRsp)
	require.Equal(t, 3, len(colsRsp.Columns))
	require.Equal(t, "NAME", colsRsp.Columns[0].Name)
	require.Equal(t, "string", colsRsp.Columns[0].Type)
	require.Equal(t, "TIME", colsRsp.Columns[1].Name)
	require.Equal(t, "datetime", colsRsp.Columns[1].Type)
	require.Equal(t, "VALUE", colsRsp.Columns[2].Name)
	require.Equal(t, "double", colsRsp.Columns[2].Type)

	// RowsFetch
	for {
		fetchRsp, err := rpcClient.RowsFetch(ctx, queryRsp.RowsHandle)
		require.NoError(t, err)
		require.NotNil(t, fetchRsp)
		require.True(t, fetchRsp.Success, fetchRsp.Reason)
		require.NotEmpty(t, fetchRsp.Reason)
		t.Log("RowsFetch", fetchRsp.Reason)

		values := machrpc.ConvertPbToAny(queryRowRsp.Values)
		require.Equal(t, "test", values[0])
		require.Equal(t, now, values[1])
		require.Equal(t, 123.456, values[2])
		t.Log("RowsFetch Values", values)

		if fetchRsp.HasNoRows {
			break
		}
	}

	// RowsClose
	closeRsp, err := rpcClient.RowsClose(ctx, queryRsp.RowsHandle)
	require.NoError(t, err)
	require.NotNil(t, closeRsp)
	require.True(t, closeRsp.Success, closeRsp.Reason)
	t.Log("RowsClose", closeRsp.Reason)

	time.Sleep(100 * time.Millisecond)
	execRsp, err = rpcClient.Exec(ctx, &machrpc.ExecRequest{
		Conn: conn,
		Sql:  "drop table test_tag",
	})
	require.NoError(t, err)
	require.NotNil(t, execRsp)
	require.True(t, execRsp.Success)
	require.NotEmpty(t, execRsp.Reason)

}
