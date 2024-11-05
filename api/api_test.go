package api_test

import (
	"context"
	_ "embed"
	"os"
	"testing"

	"github.com/machbase/neo-server/api/testsuite"
)

var testServer *testsuite.Server

func TestMain(m *testing.M) {
	testServer = testsuite.NewServer("./testsuite_tmp")
	testServer.StartServer(m)
	code := m.Run()
	testServer.StopServer(m)
	os.Exit(code)
}

func TestUserAuth(t *testing.T) {
	// machsvr
	testsuite.UserAuth(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.UserAuth(t, testServer.DatabaseRPC(), context.TODO())
}

func TestPing(t *testing.T) {
	// machsvr
	testsuite.Ping(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.Ping(t, testServer.DatabaseRPC(), context.TODO())
}

func TestLicense(t *testing.T) {
	// machsvr
	testsuite.License(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.License(t, testServer.DatabaseRPC(), context.TODO())
}

func TestDescribeTable(t *testing.T) {
	// machsvr
	testsuite.DescribeTable(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.DescribeTable(t, testServer.DatabaseRPC(), context.TODO())
}

func TestInsert(t *testing.T) {
	// machsvr
	testsuite.InsertAndQuery(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.InsertAndQuery(t, testServer.DatabaseRPC(), context.TODO())
}

func TestTables(t *testing.T) {
	// machsvr
	testsuite.ShowTables(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.ShowTables(t, testServer.DatabaseRPC(), context.TODO())
}

func TestExistsTable(t *testing.T) {
	// machsvr
	testsuite.ExistsTable(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.ExistsTable(t, testServer.DatabaseRPC(), context.TODO())
}

func TestIndexes(t *testing.T) {
	// machsvr
	testsuite.Indexes(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.Indexes(t, testServer.DatabaseRPC(), context.TODO())
}

func TestExplain(t *testing.T) {
	// machsvr
	testsuite.Explain(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.Explain(t, testServer.DatabaseRPC(), context.TODO())
}

func TestExplainFull(t *testing.T) {
	// machsvr
	testsuite.ExplainFull(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.ExplainFull(t, testServer.DatabaseRPC(), context.TODO())
}

func TestColumns(t *testing.T) {
	// machsvr
	testsuite.Columns(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.Columns(t, testServer.DatabaseRPC(), context.TODO())
}

func TestLogTableExec(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.LogTableExec(t, db, context.TODO())
}

func TestLogTableAppend(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.LogTableAppend(t, db, context.TODO())
}

func TestTagTableAppend(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.TagTableAppend(t, db, context.TODO())
}

func TestWatchLogTable(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.WatchLogTable(t, db, context.TODO())
}

func TestInsertNewTags(t *testing.T) {
	testsuite.InsertNewTags(t, testServer.DatabaseCLI(), context.TODO())
}
