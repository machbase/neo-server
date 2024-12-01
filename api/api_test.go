package api_test

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
	testsuite.TestAll(t, testServer.DatabaseSVR())

	testServer.CreateTestTables()
	db := testsuite.Database_machsvr(t)
	testsuite.LogTableExec(t, db, context.TODO())
	testsuite.LogTableAppend(t, db, context.TODO())
	testsuite.TagTableAppend(t, db, context.TODO())
	testsuite.WatchLogTable(t, db, context.TODO())
	testsuite.InsertNewTags(t, testServer.DatabaseCLI(), context.TODO())
	testServer.DropTestTables()
}
