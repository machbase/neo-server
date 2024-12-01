package machsvr_test

import (
	"os"
	"testing"

	"github.com/machbase/neo-server/v8/api/machrpc"
	"github.com/machbase/neo-server/v8/api/testsuite"
)

var rpcClient machrpc.MachbaseClient

func TestMain(m *testing.M) {
	s := testsuite.NewServer("./testsuite_tmp")
	s.StartServer(m)
	rpcClient = machrpc.NewMachbaseClient(s.ClientConn())

	testsuite.CreateTestTables(s.DatabaseSVR())
	code := m.Run()
	testsuite.DropTestTables(s.DatabaseSVR())

	s.StopServer(m)
	os.Exit(code)
}

func TestAll(t *testing.T) {
	testsuite.TestAll(t, testsuite.Database_machsvr(t))
}
