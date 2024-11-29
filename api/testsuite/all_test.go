package testsuite_test

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
	code := m.Run()
	testServer.StopServer(m)
	os.Exit(code)
}

func TestUserAuth(t *testing.T) {
	// machsvr
	testsuite.UserAuth(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.UserAuth(t, testServer.DatabaseRPC(), context.TODO())
	// machcli
	// testsuite.UserAuth(t, testServer.DatabaseCLI(), context.TODO())
}

func TestPing(t *testing.T) {
	// machsvr
	testsuite.Ping(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.Ping(t, testServer.DatabaseRPC(), context.TODO())
	// machcli
	// testsuite.Ping(t, testServer.DatabaseCLI(), context.TODO())
}

func TestLicense(t *testing.T) {
	// machsvr
	testsuite.License(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.License(t, testServer.DatabaseRPC(), context.TODO())
	// machcli
	// testsuite.License(t, testServer.DatabaseCLI(), context.TODO())
}

func TestDescribeTable(t *testing.T) {
	// machsvr
	testsuite.DescribeTable(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.DescribeTable(t, testServer.DatabaseRPC(), context.TODO())
	// machcli
	// testsuite.DescribeTable(t, testServer.DatabaseCLI(), context.TODO())
}

func TestInsert(t *testing.T) {
	// machsvr
	testsuite.InsertAndQuery(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.InsertAndQuery(t, testServer.DatabaseRPC(), context.TODO())
	// machcli
	// testsuite.InsertAndQuery(t, testServer.DatabaseCLI(), context.TODO())
}

func TestAppendTag(t *testing.T) {
	// machsvr
	testsuite.AppendTag(t, testServer.DatabaseSVR(), context.TODO())
	testsuite.AppendTagNotExist(t, testServer.DatabaseSVR(), context.TODO())
}

func TestShowTables(t *testing.T) {
	// machsvr
	testsuite.ShowTables(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.ShowTables(t, testServer.DatabaseRPC(), context.TODO())
	// machcli
	// testsuite.ShowTables(t, testServer.DatabaseCLI(), context.TODO())
}

func TestExistsTable(t *testing.T) {
	// machsvr
	testsuite.ExistsTable(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.ExistsTable(t, testServer.DatabaseRPC(), context.TODO())
	// machcli
	// testsuite.ExistsTable(t, testServer.DatabaseCLI(), context.TODO())
}

func TestIndexes(t *testing.T) {
	// machsvr
	testsuite.Indexes(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.Indexes(t, testServer.DatabaseRPC(), context.TODO())
	// machcli
	// testsuite.Indexes(t, testServer.DatabaseCLI(), context.TODO())
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
	// machcli
	// testsuite.Columns(t, testServer.DatabaseCLI(), context.TODO())
}

func TestLogTableExec(t *testing.T) {
	// machsvr
	testsuite.LogTableExec(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.LogTableExec(t, testServer.DatabaseRPC(), context.TODO())
	// machcli
	//	testsuite.LogTableExec(t, testServer.DatabaseCLI(), context.TODO())
}

func TestLogTableAppend(t *testing.T) {
	// machsvr
	testsuite.LogTableAppend(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.LogTableAppend(t, testServer.DatabaseRPC(), context.TODO())
}

func TestTagTableAppend(t *testing.T) {
	// machsvr
	testsuite.TagTableAppend(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.TagTableAppend(t, testServer.DatabaseRPC(), context.TODO())
}

func TestWatchLogTable(t *testing.T) {
	// machsvr
	testsuite.WatchLogTable(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.WatchLogTable(t, testServer.DatabaseRPC(), context.TODO())
}

func TestDemoUser(t *testing.T) {
	// machsvr
	testsuite.DemoUser(t, testServer.DatabaseSVR(), context.TODO())
	// machrpc
	testsuite.DemoUser(t, testServer.DatabaseRPC(), context.TODO())
}
