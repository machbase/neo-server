package machsvr_test

import (
	"context"
	"os"
	"testing"

	"github.com/machbase/neo-server/api/machrpc"
	"github.com/machbase/neo-server/api/testsuite"
)

var rpcClient machrpc.MachbaseClient

func TestMain(m *testing.M) {
	s := testsuite.NewServer("./testsuite_tmp")
	s.StartServer(m)
	rpcClient = machrpc.NewMachbaseClient(s.ClientConn())
	code := m.Run()
	s.StopServer(m)
	os.Exit(code)
}

func TestLicense(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.License(t, db, context.TODO())
}

func TestDescribeTable(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.DescribeTable(t, db, context.TODO())
}

func TestInsert(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.InsertAndQuery(t, db, context.TODO())
}

func TestTables(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.ShowTables(t, db, context.TODO())
}

func TestExistsTable(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.ExistsTable(t, db, context.TODO())
}

func TestIndexes(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.Indexes(t, db, context.TODO())
}

func TestWatchLogTable(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.WatchLogTable(t, db, context.TODO())
}

func TestExplain(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.Explain(t, db, context.TODO())
}

func TestExplainFull(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.ExplainFull(t, db, context.TODO())
}

func TestColumns(t *testing.T) {
	db := testsuite.Database_machsvr(t)
	testsuite.Columns(t, db, context.TODO())
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
