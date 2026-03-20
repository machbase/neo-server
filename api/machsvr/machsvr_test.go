package machsvr_test

import (
	"os"
	"testing"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/stretchr/testify/require"
)

var machsvrDB api.Database

func TestMain(m *testing.M) {
	s := testsuite.NewServer("./testsuite_tmp")
	s.StartServer()
	machsvrDB = s.DatabaseSVR()

	code := m.Run()

	s.StopServer()
	os.Exit(code)
}

func TestAll(t *testing.T) {
	testsuite.CreateTestTables(machsvrDB)
	testsuite.TestAll(t, machsvrDB)
	testsuite.DropTestTables(machsvrDB)

	testsuite.CreateTestTables(machsvrDB)
	testsuite.TestAll(t, machsvrDB,
		tcSetMaxConn,
	)
	testsuite.DropTestTables(machsvrDB)
}

func tcSetMaxConn(t *testing.T) {
	engine := machsvrDB.(*machsvr.Database)
	expectLimit, open := engine.MaxOpenConn()
	require.NotZero(t, expectLimit)
	require.LessOrEqual(t, -1, open)

	expectLimit = 1000
	engine.SetMaxOpenConn(expectLimit)
	limit, open := engine.MaxOpenConn()
	require.Equal(t, expectLimit, limit)
	require.LessOrEqual(t, -1, open)
}
