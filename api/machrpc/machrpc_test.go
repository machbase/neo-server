package machrpc_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/machbase/neo-server/v8/api/machrpc"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"google.golang.org/grpc"
)

var testServer *testsuite.Server

func TestMain(m *testing.M) {
	testServer = testsuite.NewServer("./testsuite_tmp")
	testServer.StartServer(m)
	testServer.CreateTestTables()

	sql.Register("machbase", &machrpc.Driver{
		ConnProvider: func() (*grpc.ClientConn, error) {
			return testServer.ClientConn(), nil
		},
		User:     "sys",
		Password: "manager",
	})
	code := m.Run()
	testServer.DropTestTables()
	testServer.StopServer(m)
	os.Exit(code)
}

func TestAll(t *testing.T) {
	testsuite.TestAll(t, testServer.DatabaseRPC())
}

// func TestExec(t *testing.T) {
// 	ctx := context.TODO()
// 	conn, err := database.Connect(ctx, api.WithPassword("sys", "manager"))
// 	if err != nil {
// 		t.Fatalf("connect error: %s", err.Error())
// 	}
// 	defer conn.Close()

// 	result := conn.Exec(context.TODO(), "insert into example (name, time, value) values(?, ?, ?)", 1, 2, 3)
// 	require.NotNil(t, result)
// 	require.Nil(t, result.Err())
// 	require.Equal(t, int64(1), result.RowsAffected())
// }

// func TestQueryRow(t *testing.T) {
// 	ctx := context.TODO()
// 	conn, err := database.Connect(ctx, api.WithPassword("sys", "manager"))
// 	if err != nil {
// 		t.Fatalf("connect error: %s", err.Error())
// 	}
// 	defer conn.Close()

// 	row := conn.QueryRow(context.TODO(), "select count(*) from tag_data where name = ?", "query1")
// 	require.NotNil(t, row)

// 	require.Nil(t, row.Err())
// 	require.Equal(t, int64(1), row.RowsAffected())
// 	require.Equal(t, "a row selected.", row.Message())
// 	columns, _ := row.Columns()
// 	require.Equal(t, 1, len(columns))

// 	var val int
// 	if err := row.Scan(&val); err != nil {
// 		t.Fatalf("row scan fail; %s", err.Error())
// 	}
// 	require.Equal(t, 123, val)
// }

// func TestQuery(t *testing.T) {
// 	ctx := context.TODO()
// 	conn, err := database.Connect(ctx, api.WithPassword("sys", "manager"))
// 	if err != nil {
// 		t.Fatalf("connect error: %s", err.Error())
// 	}
// 	defer conn.Close()

// 	rows, err := conn.Query(context.TODO(), "select * from example where name = ?", "query1")
// 	if err != nil {
// 		t.Fatalf("query fail, %q", err.Error())
// 	}
// 	defer rows.Close()

// 	require.True(t, rows.IsFetchable())
// 	require.Equal(t, int64(0), rows.RowsAffected())
// 	require.Equal(t, "success", rows.Message())

// 	columns, err := rows.Columns()
// 	if err != nil {
// 		t.Fatalf("columns error, %s", err.Error())
// 	}
// 	require.Equal(t, 3, len(columns))

// 	var name string
// 	var ts time.Time
// 	var value float64
// 	for rows.Next() {
// 		err := rows.Scan(&name, &ts, &value)
// 		if err != nil {
// 			t.Fatalf("rows scan error, %s", err.Error())
// 		}
// 	}

// 	require.Equal(t, "tag", name)
// 	require.Equal(t, time.Unix(0, 1).Nanosecond(), ts.Nanosecond())
// 	require.Equal(t, 3.14, value)
// }

// func TestAppend(t *testing.T) {
// 	ctx := context.TODO()
// 	conn, err := database.Connect(ctx, api.WithPassword("sys", "manager"))
// 	if err != nil {
// 		t.Fatalf("connect error: %s", err.Error())
// 	}
// 	defer conn.Close()

// 	appender, err := conn.Appender(context.TODO(), "example")
// 	if err != nil {
// 		t.Fatalf("appender error, %s", err.Error())
// 	}
// 	require.NotNil(t, appender)

// 	for i := 0; i < 10; i++ {
// 		err := appender.Append(i)
// 		if err != nil {
// 			t.Fatalf("append fail, %s", err.Error())
// 		}
// 	}

// 	succ, fail, err := appender.Close()
// 	if err != nil {
// 		t.Errorf("appender close error, %s", err.Error())
// 	}
// 	require.Equal(t, int64(10), succ)
// 	require.Equal(t, int64(0), fail)
// }

// var database api.Database

// func TestNewClient(t *testing.T) {
// 	var cli *machrpc.Client
// 	var err error

// 	// no server address
// 	cli, err = machrpc.NewClient(&machrpc.Config{})
// 	require.NotNil(t, err, "no error without server addr, want error")
// 	require.Nil(t, cli, "new client should fail")
// 	require.Equal(t, "server address is not specified", err.Error())

// 	// success creating client
// 	cli, err = machrpc.NewClient(&machrpc.Config{ServerAddr: MockServerAddr})
// 	if err != nil {
// 		t.Fatalf("new client: %s", err.Error())
// 	}

// 	ctx := context.TODO()

// 	// empty username, password
// 	conn, err := cli.Connect(ctx)
// 	require.NotNil(t, err)
// 	require.Equal(t, "no user specified, use WithPassword() option", err.Error())
// 	require.Nil(t, conn)

// 	// wrong password
// 	conn, err = cli.Connect(ctx, api.WithPassword("sys", "mm"))
// 	require.NotNil(t, err)
// 	require.Equal(t, "invalid username or password", err.Error())
// 	require.Nil(t, conn)

// 	// correct username, password
// 	conn, err = cli.Connect(ctx, api.WithPassword("sys", "manager"))
// 	require.Nil(t, err)
// 	require.NotNil(t, conn)

// 	conn.Close()
// }
