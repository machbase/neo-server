package api_test

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/machsvr"
)

//go:embed api_test.conf
var machbase_conf []byte
var machbase_port = 15656

func TestMain(m *testing.M) {
	// prepare
	tmpPath := filepath.Join(".", "machbase_test_tmp")
	homePath, err := filepath.Abs(filepath.Join(tmpPath, "machbase"))
	if err != nil {
		panic(err)
	}
	confPath := filepath.Join(homePath, "conf", "machbase.conf")

	os.RemoveAll(homePath)
	os.MkdirAll(homePath, 0755)
	os.MkdirAll(filepath.Join(homePath, "conf"), 0755)
	os.MkdirAll(filepath.Join(homePath, "trc"), 0755)
	os.MkdirAll(filepath.Join(homePath, "dbs"), 0755)
	os.WriteFile(confPath, machbase_conf, 0644)

	if err := machsvr.Initialize(homePath, machbase_port, machsvr.OPT_SIGHANDLER_SIGINT_OFF); err != nil {
		panic(err)
	}

	if !machsvr.ExistsDatabase() {
		if err := machsvr.CreateDatabase(); err != nil {
			panic(err)
		}
	}

	// setup
	db, err := machsvr.NewDatabase()
	if err != nil {
		panic(err)
	}

	if err := db.Startup(); err != nil {
		panic(err)
	}

	// create test tables

	ctx := context.TODO()
	conn, _ := db.Connect(ctx, api.WithTrustUser("sys"))
	result := conn.Exec(ctx, api.SqlTidy(`
		create tag table tag_data(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double,
			short_value     short,
			int_value       integer,
			long_value      long,
			str_value       varchar(400),
			json_value      json
		)
	`))
	if err := result.Err(); err != nil {
		panic(err)
	}

	result = conn.Exec(ctx, api.SqlTidy(`
		create tag table tag_simple(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double
		)
	`))
	if err := result.Err(); err != nil {
		panic(err)
	}

	result = conn.Exec(ctx, api.SqlTidy(`
		create table log_data(
			short_value  short,
			int_value    integer,
			long_value   long,
			double_value double,
			float_value  float,
			str_value 	 varchar(400),
			json_value 	 json,
			ipv4_value   ipv4,
			ipv6_value   ipv6
		)
	`))
	if err := result.Err(); err != nil {
		panic(err)
	}

	// run tests
	code := m.Run()

	result = conn.Exec(ctx, `drop table tag_data`)
	if err := result.Err(); err != nil {
		panic(err)
	}

	result = conn.Exec(ctx, `drop table log_data`)
	if err := result.Err(); err != nil {
		panic(err)
	}

	// teardown
	if err := db.Shutdown(); err != nil {
		panic(err)
	}

	machsvr.Finalize()
	os.RemoveAll(tmpPath)
	os.Exit(code)
}

type TestingT interface {
	Log(args ...any)
	Fatal(args ...any)
	Fail()
	Fatalf(format string, args ...any)
}

func machsvrDatabase(t TestingT) api.Database {
	var db api.Database
	if machsvr_db, err := machsvr.NewDatabase(); err != nil {
		t.Log("Error", err.Error())
		t.Fail()
	} else {
		db = machsvr_db
	}
	return db
}

// TODO machcli
//
// func machcliDatabase(t TestingT) api.Database {
// 	cli, err := machrpc.NewClient(&machrpc.Config{
// 		ServerAddr:    fmt.Sprintf("127.0.0.1:%d", machbase_port),
// 		QueryTimeout:  3 * time.Second,
// 		AppendTimeout: 3 * time.Second,
// 	})
// 	if err != nil {
// 		t.Fatalf("new client: %s", err.Error())
// 	}
// 	return cli
// }
