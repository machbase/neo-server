package machrpc_test

import (
	context "context"
	"testing"
	"time"

	"github.com/machbase/neo-server/api/machrpc"
	"github.com/stretchr/testify/require"
)

func newClient(t *testing.T) *machrpc.Client {
	t.Helper()
	cli, err := machrpc.NewClient(&machrpc.Config{
		ServerAddr:    MockServerAddr,
		QueryTimeout:  3 * time.Second,
		AppendTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %s", err.Error())
	}
	return cli
}

func newConn(t *testing.T) *machrpc.Conn {
	t.Helper()
	cli := newClient(t)
	conn, err := cli.Connect(context.TODO(), machrpc.WithPassword("sys", "manager"))
	require.Nil(t, err)
	require.NotNil(t, conn)
	return conn
}

func TestAuth(t *testing.T) {
	cli, err := machrpc.NewClient(&machrpc.Config{ServerAddr: MockServerAddr})
	if err != nil {
		t.Fatalf("new client: %s", err.Error())
	}
	ok, err := cli.UserAuth("sys", "mm")
	require.NotNil(t, err)
	require.Equal(t, "invalid username or password", err.Error())
	require.False(t, ok)

	ok, err = cli.UserAuth("sys", "manager")
	if err != nil {
		t.Fatalf("UserAuth failed: %s", err.Error())
	}
	require.True(t, ok)
}

func TestNewClient(t *testing.T) {
	var cli *machrpc.Client
	var err error

	// no server address
	cli, err = machrpc.NewClient(&machrpc.Config{})
	require.NotNil(t, err, "no error without server addr, want error")
	require.Nil(t, cli, "new client should fail")
	require.Equal(t, "server address is not specified", err.Error())

	// success creating client
	cli, err = machrpc.NewClient(&machrpc.Config{ServerAddr: MockServerAddr})
	if err != nil {
		t.Fatalf("new client: %s", err.Error())
	}

	ctx := context.TODO()

	// empty username, password
	conn, err := cli.Connect(ctx)
	require.NotNil(t, err)
	require.Equal(t, "no user specified, use WithPassword() option", err.Error())
	require.Nil(t, conn)

	// wrong password
	conn, err = cli.Connect(ctx, machrpc.WithPassword("sys", "mm"))
	require.NotNil(t, err)
	require.Equal(t, "invalid username or password", err.Error())
	require.Nil(t, conn)

	// correct username, password
	conn, err = cli.Connect(ctx, machrpc.WithPassword("sys", "manager"))
	require.Nil(t, err)
	require.NotNil(t, conn)

	conn.Close()
}

type Pinger interface {
	Ping() (time.Duration, error)
}

func TestPing(t *testing.T) {
	pinger := newConn(t)
	_, err := pinger.Ping()
	require.Nil(t, err)
	pinger.Close()
}

type Explainer interface {
	// Explain retrieves execution plan of the given SQL statement.
	Explain(ctx context.Context, sqlText string, full bool) (string, error)
}

func TestExplainTest(t *testing.T) {
	exp := newConn(t)
	defer exp.Close()
	result, err := exp.Explain(context.TODO(), "select * from dummy", true)
	if err != nil {
		t.Fatalf("Explain error: %s", err.Error())
	}
	require.Equal(t, "explain dummy result", result)
}

func TestExec(t *testing.T) {
	conn := newConn(t)
	defer conn.Close()
	result := conn.Exec(context.TODO(), "insert into example (name, time, value) values(?, ?, ?)", 1, 2, 3)
	require.NotNil(t, result)
	require.Nil(t, result.Err())
	require.Equal(t, int64(1), result.RowsAffected())
}

func TestQueryRow(t *testing.T) {
	conn := newConn(t)
	defer conn.Close()
	row := conn.QueryRow(context.TODO(), "select count(*) from example where name = ?", "query1")
	require.NotNil(t, row)

	require.True(t, row.Success())
	require.Nil(t, row.Err())
	require.Equal(t, int64(1), row.RowsAffected())
	require.Equal(t, "a row selected.", row.Message())
	require.Equal(t, 1, len(row.Values()))

	var val int
	if err := row.Scan(&val); err != nil {
		t.Fatalf("row scan fail; %s", err.Error())
	}
	require.Equal(t, 123, val)
}

func TestQuery(t *testing.T) {
	conn := newConn(t)
	defer conn.Close()
	rows, err := conn.Query(context.TODO(), "select * from example where name = ?", "query1")
	if err != nil {
		t.Fatalf("query fail, %q", err.Error())
	}
	defer rows.Close()

	require.True(t, rows.IsFetchable())
	require.Equal(t, int64(0), rows.RowsAffected())
	require.Equal(t, "success", rows.Message())

	names, types, err := rows.Columns()
	if err != nil {
		t.Fatalf("columns error, %s", err.Error())
	}
	require.Equal(t, 3, len(names))
	require.Equal(t, 3, len(types))

	var name string
	var ts time.Time
	var value float64
	for rows.Next() {
		err := rows.Scan(&name, &ts, &value)
		if err != nil {
			t.Fatalf("rows scan error, %s", err.Error())
		}
	}

	require.Equal(t, "tag", name)
	require.Equal(t, time.Unix(0, 1).Nanosecond(), ts.Nanosecond())
	require.Equal(t, 3.14, value)
}

func TestAppend(t *testing.T) {
	conn := newConn(t)
	defer conn.Close()
	appender, err := conn.Appender(context.TODO(), "example")
	if err != nil {
		t.Fatalf("appender error, %s", err.Error())
	}
	require.NotNil(t, appender)

	for i := 0; i < 10; i++ {
		err := appender.Append(i)
		if err != nil {
			t.Fatalf("append fail, %s", err.Error())
		}
	}

	succ, fail, err := appender.Close()
	if err != nil {
		t.Errorf("appender close error, %s", err.Error())
	}
	require.Equal(t, int64(10), succ)
	require.Equal(t, int64(0), fail)
}
