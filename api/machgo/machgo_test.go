package machgo_test

import (
	"context"
	"os"
	"testing"

	"github.com/machbase/neo-server/v8/api"
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
