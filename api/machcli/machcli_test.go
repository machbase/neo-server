package machcli_test

import (
	"context"
	_ "embed"
	"os"
	"testing"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/stretchr/testify/require"
)

var testServer *testsuite.Server

func TestMain(m *testing.M) {
	testServer = testsuite.NewServer("./testsuite_tmp")
	testServer.StartServer(m)
	code := m.Run()
	testServer.StopServer(m)
	os.Exit(code)
}

func TestAll(t *testing.T) {
	testServer.CreateTestTables()
	testsuite.TestAll(t, testServer.DatabaseCLI(), tcTrustUser)
	testServer.DropTestTables()
}

func tcTrustUser(t *testing.T) {
	ctx := context.Background()
	db := testServer.DatabaseCLI()
	ok, _, err := db.UserAuth(ctx, "sys", "manager")
	require.NoError(t, err)
	require.True(t, ok)

	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	require.NoError(t, err)
	require.NotNil(t, conn)
	err = conn.Close()
	require.NoError(t, err)
}
