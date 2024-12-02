package machcli_test

import (
	_ "embed"
	"os"
	"testing"

	"github.com/machbase/neo-server/v8/api/testsuite"
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
	testsuite.TestAll(t, testServer.DatabaseCLI())
	testServer.DropTestTables()
}
