package machcli_test

import (
	"context"
	_ "embed"
	"os"
	"testing"

	"github.com/machbase/neo-server/v8/api/testsuite"
)

var testServer *testsuite.Server

func TestMain(m *testing.M) {
	testServer = testsuite.NewServer("./testsuite_tmp")
	testServer.StartServer(m)
	testServer.CreateTestTables()
	code := m.Run()
	testServer.DropTestTables()
	testServer.StopServer(m)
	os.Exit(code)
}

func TestAll(t *testing.T) {
	testsuite.TestAll(t, testServer.DatabaseCLI())
}

func TestLogTableAppend(t *testing.T) {
	testsuite.LogTableAppend(t, testServer.DatabaseCLI(), context.TODO())
}
