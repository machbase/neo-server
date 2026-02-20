package testsuite_test

import (
	"context"
	_ "embed"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
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
	for _, db := range []api.Database{
		testServer.DatabaseSVR(),
		testServer.DatabaseRPC(),
		testServer.DatabaseCLI(),
		testServer.DatabaseGO(),
	} {
		if err := testsuite.CreateTestTables(db); err != nil {
			t.Fatalf("ERROR: %s", err)
		}
		testsuite.TestAll(t, db)
		if err := testsuite.DropTestTables(db); err != nil {
			t.Fatalf("ERROR: %s", err)
		}
		if runtime.GOOS == "windows" {
			// workaround for windows, it crash randomly when closing a connection of "drop table"
			time.Sleep(10 * time.Second)
		}
	}
}

func TestColumns(t *testing.T) {
	db := testServer.DatabaseSVR()
	if err := testsuite.CreateTestTables(db); err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	testsuite.ColumnsCases(t, db, context.TODO())
	if err := testsuite.DropTestTables(db); err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	if runtime.GOOS == "windows" {
		// workaround for windows, it crash randomly when closing a connection of "drop table"
		time.Sleep(10 * time.Second)
	}
}
