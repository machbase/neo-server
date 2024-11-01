package machcli_test

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
	// machcli
	testsuite.UserAuth(t, testServer.DatabaseCLI(), context.TODO())
}

func TestPing(t *testing.T) {
	// machcli
	testsuite.Ping(t, testServer.DatabaseCLI(), context.TODO())
}

func TestLicense(t *testing.T) {
	// machcli
	testsuite.License(t, testServer.DatabaseCLI(), context.TODO())
}

func TestDescribeTable(t *testing.T) {
	// machcli
	testsuite.DescribeTable(t, testServer.DatabaseCLI(), context.TODO())
}

func TestInsert(t *testing.T) {
	// machcli
	testsuite.InsertAndQuery(t, testServer.DatabaseCLI(), context.TODO())
}

func TestTables(t *testing.T) {
	// machcli
	testsuite.Tables(t, testServer.DatabaseCLI(), context.TODO())
}

func TestExistsTable(t *testing.T) {
	// machcli
	testsuite.ExistsTable(t, testServer.DatabaseCLI(), context.TODO())
}

func TestIndexes(t *testing.T) {
	// machcli
	testsuite.Indexes(t, testServer.DatabaseCLI(), context.TODO())
}

func TestColumns(t *testing.T) {
	// machcli
	testsuite.Columns(t, testServer.DatabaseCLI(), context.TODO())
}
