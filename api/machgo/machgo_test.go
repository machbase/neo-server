package machgo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machgo"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/stretchr/testify/require"
)

var testServer *testsuite.Server

func TestMain(m *testing.M) {
	testServer = testsuite.NewServer("./testsuite_tmp")
	testServer.StartServer()
	code := m.Run()
	testServer.StopServer()
	os.Exit(code)
}

func TestAll(t *testing.T) {
	db := testServer.DatabaseGO()
	testsuite.CreateTestTables(db)
	testsuite.TestAll(t, db, tcTrustUser)
	testsuite.DropTestTables(db)
}

func tcTrustUser(t *testing.T) {
	db := testServer.DatabaseGO()

	ctx := context.Background()
	ok, _, err := db.UserAuth(ctx, "sys", "manager")
	require.NoError(t, err)
	require.True(t, ok)

	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	require.NoError(t, err)
	require.NotNil(t, conn)
	err = conn.Close()
	require.NoError(t, err)

	conn, err = db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	require.NotNil(t, conn)
	err = conn.Close()
	require.NoError(t, err)
}

func TestConnectFailureReturnsMaxOpenToken(t *testing.T) {
	db, err := machgo.NewDatabase(&machgo.Config{
		Host:        "127.0.0.1",
		Port:        1,
		MaxOpenConn: 1,
	})
	require.NoError(t, err)
	defer db.Close()

	limit, remains := db.MaxOpenConns()
	require.Equal(t, 1, limit)
	require.Equal(t, 1, remains)

	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()
	_, err = db.Connect(ctx1, api.WithPassword("sys", "manager"))
	require.Error(t, err)
	_, remains = db.MaxOpenConns()
	require.Equal(t, 1, remains, "token must be returned after failed connect")

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	_, err = db.Connect(ctx2, api.WithPassword("sys", "manager"))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "connect canceled")
	_, remains = db.MaxOpenConns()
	require.Equal(t, 1, remains, "token must remain available after repeated failed connects")
}
