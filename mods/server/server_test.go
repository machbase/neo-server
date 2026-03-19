package server

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	server_api "github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/ory/dockertest/v4"
	"github.com/stretchr/testify/require"
)

var testTimeTick = time.Unix(1705291859, 0)

var machServerAddress = ""

var mqttServer *mqttd
var mqttServerAddress = ""

var httpServer *httpd
var httpServerAddress = ""

var shellArgs = []string{}

func TestMain(m *testing.M) {
	// logging
	logging.Configure(&logging.Config{
		Console:                     true,
		Filename:                    "-",
		Append:                      false,
		DefaultPrefixWidth:          10,
		DefaultEnableSourceLocation: true,
		DefaultLevel:                "INFO",
	})

	dataPath := "./testsuite_tmp"
	// database
	testServer := testsuite.NewServer(dataPath)
	testServer.StartServer()
	testServer.CreateTestTables()
	database := testServer.DatabaseSVR()

	// default database
	api.SetDefault(database)

	func(db api.Database) {
		ctx := context.TODO()
		conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		result := conn.Exec(ctx, `CREATE TAG TABLE example (
				name VARCHAR(40) PRIMARY KEY,
				time DATETIME BASETIME,
				value DOUBLE SUMMARIZED
			) TAG_DUPLICATE_CHECK_DURATION=1`)
		if result.Err() != nil {
			panic(result.Err())
		}

		rows := [][]any{
			{"temp", testTimeTick, 3.14},
		}
		for i := 1; i <= 10; i++ {
			rows = append(rows,
				[]any{"test.query", testTimeTick.Add(time.Duration(i) * time.Second), 1.5 * float64(i)},
			)
		}
		for _, row := range rows {
			result = conn.Exec(ctx, `INSERT INTO example VALUES (?, ?, ?)`, row[0], row[1], row[2])
			if result.Err() != nil {
				panic(result.Err())
			}
		}
		conn.Exec(ctx, `EXEC table_flush(example)`)
	}(database)

	machServerAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testServer.MachPort())

	// metric
	server_api.StartMetrics()
	// append worker
	server_api.StartAppendWorkers()

	var projRoot = filepath.FromSlash("../../")
	prefDir := filepath.Join(projRoot, "tmp", "test", "pref")
	filesDir := filepath.Join(projRoot, "tmp", "test", "files")
	// cleanup pref and files directories before test
	os.RemoveAll(prefDir)
	os.RemoveAll(filesDir)
	// create server instance
	var server, _ = NewServer(&Config{
		PrefDir:  prefDir,
		FileDirs: []string{filesDir},
	})
	server.preparePrefDir()
	server.startModelService()
	server.startBridgeAndSchedulerService()
	server.AddServicePort("mach", machServerAddress)
	RegisterJsonRpcHandlers(server)

	// tql
	fileDirs := []string{"/=./test"}
	serverFs, _ := ssfs.NewServerSideFileSystem(fileDirs)
	ssfs.SetDefault(serverFs)
	tql.Init()
	defer tql.Deinit()

	// http server
	httpOpts := []HttpOption{
		WithHttpListenAddress("tcp://127.0.0.1:0"),
		WithHttpTqlLoader(tql.NewLoader()),
		WithHttpEulaFilePath("./testsuite_tmp/eula.txt"),
		WithHttpPathMap("data", dataPath),
	}
	if svr, err := NewHttp(database, httpOpts...); err != nil {
		panic(err)
	} else {
		httpServer = svr
	}
	if err := httpServer.Start(); err != nil {
		panic(err)
	}

	// get http listener address
	if addr := httpServer.listeners[0].Addr().String(); addr == "" {
		panic("Listener not found")
	} else {
		httpServerAddress = "http://" + strings.TrimPrefix(addr, "tcp://")
	}

	// mqtt broker
	mqttOpts := []MqttOption{
		WithMqttTcpListener("127.0.0.1:0", nil),
		WithMqttTqlLoader(tql.NewLoader()),
	}
	if svr, err := NewMqtt(database, mqttOpts...); err != nil {
		panic(err)
	} else {
		mqttServer = svr
	}

	if err := mqttServer.Start(); err != nil {
		panic(err)
	}

	// get mqtt listener address
	if addr, ok := mqttServer.broker.Listeners.Get("mqtt-tcp-0"); !ok {
		panic("Listener not found")
	} else {
		mqttServerAddress = strings.TrimPrefix(addr.Address(), "tcp://")
	}

	// build shell binary for shell tests
	func() {
		var projRoot = filepath.FromSlash("../../")
		var binPath = filepath.Join(projRoot, "tmp", "machbase-neo")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
		buildShellCmd := []string{
			"go", "build", "-o", binPath, filepath.Join(projRoot, "main", "machbase-neo"),
		}
		err := exec.Command(buildShellCmd[0], buildShellCmd[1:]...).Run()
		if err != nil {
			panic(err)
		}

		shellArgs = []string{
			binPath,
			"shell",
			"--server", httpServerAddress,
			"--user", "sys",
			"--password", "manager",
			"-v", "/work=./test",
		}
	}()

	// run tests
	m.Run()

	// cleanup
	mqttServer.Stop()
	httpServer.Stop()
	server_api.StopMetrics()
	testServer.DropTestTables()
	testServer.StopServer()
}

func TestRepresentativePort(t *testing.T) {
	require.Equal(t, "  > Local:   http://127.0.0.1:1234", representativePort("tcp://127.0.0.1:1234"))
	require.Equal(t, "  > Network: http://192.168.1.100:1234", representativePort("http://192.168.1.100:1234"))
	if runtime.GOOS == "windows" {
		require.Equal(t, `  > Unix:    C:\var\run\neo-server.sock`, representativePort(`unix://C:\var\run\neo-server.sock`))
	} else {
		require.Equal(t, "  > Unix:    /var/run/neo-server.sock", representativePort("unix:///var/run/neo-server.sock"))
	}
}

type ShellTestCase struct {
	name      string
	args      []string
	expect    []string
	expectErr string
}

func runShellTestCase(t *testing.T, tt ShellTestCase) {
	t.Helper()
	t.Run(tt.name, func(t *testing.T) {
		t.Helper()
		cmd := exec.Command(tt.args[0], tt.args[1:]...)
		output, err := cmd.CombinedOutput()
		if tt.expectErr != "" {
			require.Error(t, err, "Expected error: %s", tt.expectErr)
			require.Contains(t, string(output), tt.expectErr, "Expected error message not found")
			return
		}
		require.NoError(t, err, "Shell command failed: %s", string(output))
		outputLines := strings.Split(string(output), "\n")
		for i, outputLine := range outputLines {
			if i >= len(tt.expect) {
				if outputLine != "" || i != len(outputLines)-1 {
					require.Fail(t, "Unexpected extra output", "Line: %s", outputLine)
				}
				continue
			}
			expect := tt.expect[i]
			if strings.HasPrefix(expect, "/r/") {
				// regular expression match
				pattern := expect[3:]
				matched, err := regexp.MatchString(pattern, outputLine)
				require.NoError(t, err, "Invalid regular expression: %s", pattern)
				require.True(t, matched, "Output line does not match pattern. Line: %s, Pattern: %s", outputLine, pattern)
			} else {
				require.Equal(t, expect, outputLine, "Outputs:\n%s", strings.Join(outputLines, "\n"))
			}
		}
	})
}

func TestShellShow(t *testing.T) {
	tests := []ShellTestCase{
		{
			name: "show_license",
			args: append(shellArgs, "show", "license", "--box-style", "simple"),
			expect: []string{
				"+--------+----------+-----------+----------+---------+--------------+---------------------+-------------+--------+",
				"| ROWNUM | ID       | TYPE      | CUSTOMER | PROJECT | COUNTRY_CODE | INSTALL_DATE        |  ISSUE_DATE | STATUS |",
				"+--------+----------+-----------+----------+---------+--------------+---------------------+-------------+--------+",
				`/r/|      1 | 00000000 | COMMUNITY | NONE     | NONE    | KR           | [0-9\- :]+ | 20991231    | VALID  |`,
				"+--------+----------+-----------+----------+---------+--------------+---------------------+-------------+--------+",
			},
		},
		{
			name: "show_tables",
			args: append(shellArgs, "show", "tables", "--format", "csv"),
			expect: []string{
				"ROWNUM,DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG",
				"1,MACHBASEDB,SYS,EXAMPLE,19,Tag,",
				"2,MACHBASEDB,SYS,LOG_DATA,13,Log,",
				"3,MACHBASEDB,SYS,TAG_DATA,6,Tag,",
				"4,MACHBASEDB,SYS,TAG_SIMPLE,12,Tag,",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func supportDockerTest() bool {
	if os.Getenv("CI") == "true" {
		return false
	}
	if runtime.GOOS == "linux" {
		return runtime.GOARCH == "amd64"
	}
	if runtime.GOOS == "windows" {
		return false
	}
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err == nil {
			// new docker path for mac docker desktop
			path := filepath.Join(home, ".docker", "run", "docker.sock")
			_, err = os.Stat(path)
			if err == nil {
				os.Setenv("DOCKER_HOST", "unix://"+path)
				return true
			}
		}
		// fallback to old docker path for mac docker desktop
		_, err = os.Stat("/var/run/docker.sock")
		if err != nil {
			return false
		}
	}
	return true
}

func TestShellBridge(t *testing.T) {
	if !supportDockerTest() {
		t.Skip("dockertest does not work in this environment")
	}
	// dockertest pool
	pool := dockertest.NewPoolT(t, "")
	//
	// start postgreSQL
	//
	postgres := pool.RunT(t, "postgres",
		dockertest.WithTag("16"),
		dockertest.WithEnv([]string{
			"POSTGRES_USER=dbuser",
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_DB=db",
		}),
	)
	//
	// start MSSQL
	//
	mssqlImage := "mcr.microsoft.com/mssql/server"
	mssqlTag := "2025-latest"
	// azure-sql-edge was deprecated
	//
	// if runtime.GOARCH == "arm64" { // for arm64, use azure-sql-edge which supports arm64
	// 	mssqlImage = "mcr.microsoft.com/azure-sql-edge"
	// 	mssqlTag = "2.0.0"
	// }
	mssql := pool.RunT(t, mssqlImage,
		dockertest.WithTag(mssqlTag),
		dockertest.WithEnv([]string{
			"ACCEPT_EULA=Y",
			"MSSQL_SA_PASSWORD=Your_password123",
		}),
	)
	//
	// start MYSQL
	//
	mysql := pool.RunT(t, "mysql",
		dockertest.WithTag("8.0"),
		dockertest.WithEnv([]string{
			"MYSQL_ROOT_PASSWORD=secret",
			"MYSQL_DATABASE=db",
			"MYSQL_USER=dbuser",
			"MYSQL_PASSWORD=secret",
		}),
	)
	//
	// start Mosquitto MQTT broker
	//
	// find directory neo-server/mods/server/test
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Failed to get current file path")
	}
	testDir := filepath.Join(filepath.Dir(filename), "test")

	mosquitto := pool.RunT(t, "eclipse-mosquitto",
		dockertest.WithTag("2.0"),
		dockertest.WithMounts([]string{filepath.Join(testDir, "mosquitto.conf") + ":/mosquitto/config/mosquitto.conf:ro"}),
	)
	//
	// start NATS server
	//
	nats := pool.RunT(t, "nats",
		dockertest.WithTag("2.12"),
	)

	// wait for mosquitto to be ready
	var mosquittoHostPort string
	err := pool.Retry(t.Context(), 60*time.Second, func() error {
		mosquittoHostPort = mosquitto.GetHostPort("1883/tcp")
		conn, err := net.Dial("tcp", mosquittoHostPort)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	})
	require.NoError(t, err, "Mosquitto did not start in time")

	// wait for nats to be ready
	var natsHostPort string
	err = pool.Retry(t.Context(), 60*time.Second, func() error {
		natsHostPort = nats.GetHostPort("4222/tcp")
		conn, err := net.Dial("tcp", natsHostPort)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	})

	// wait for postgres to be ready
	var postgresDSN string
	err = pool.Retry(t.Context(), 60*time.Second, func() error {
		hostPort := postgres.GetHostPort("5432/tcp")
		host, port, _ := net.SplitHostPort(hostPort)
		postgresDSN = fmt.Sprintf("host=%s port=%s dbname=db user=dbuser password=secret sslmode=disable", host, port)
		db, err := sql.Open("postgres", postgresDSN)
		if err != nil {
			return err
		}
		return db.Ping()
	})
	if err != nil {
		t.Fatalf("could not connect to postgres: %v", err)
	}

	// wait for mssql to be ready
	var mssqlDSN string
	err = pool.Retry(t.Context(), 60*time.Second, func() error {
		hostPort := mssql.GetHostPort("1433/tcp")
		db, err := sql.Open("sqlserver", fmt.Sprintf("sqlserver://sa:Your_password123@%s?database=master", hostPort))
		if err != nil {
			return err
		}
		mssqlDSN = fmt.Sprintf("server=%s user=sa password=Your_password123 database=master encrypt=disable", hostPort)
		return db.Ping()
	})
	if err != nil {
		t.Fatalf("could not connect to mssql: %v", err)
	}

	// wait for mysql to be ready
	var mysqlDSN string
	err = pool.Retry(t.Context(), 60*time.Second, func() error {
		hostPort := mysql.GetHostPort("3306/tcp")
		mysqlDSN = fmt.Sprintf("dbuser:secret@tcp(%s)/db?parseTime=true", hostPort)
		db, err := sql.Open("mysql", mysqlDSN)
		if err != nil {
			return err
		}
		return db.Ping()
	})
	if err != nil {
		t.Fatalf("could not connect to mysql: %v", err)
	}
	t.Run("shellBridgeSqliteTest", func(t *testing.T) {
		shellBridgeSqliteTest(t)
	})
	t.Run("shellBridgePostgresTest", func(t *testing.T) {
		shellBridgePostgresTest(t, postgresDSN)
	})
	t.Run("shellBridgeMSSqlTest", func(t *testing.T) {
		shellBridgeMSSqlTest(t, mssqlDSN)
	})
	t.Run("shellBridgeMySqlTest", func(t *testing.T) {
		shellBridgeMySqlTest(t, mysqlDSN)
	})
	t.Run("shellBridgeMqttTest", func(t *testing.T) {
		shellBridgeMqttTest(t, mosquittoHostPort)
	})
	t.Run("shellBridgeNatsTest", func(t *testing.T) {
		shellBridgeNatsTest(t, natsHostPort)
	})

}

func shellBridgeSqliteTest(t *testing.T) {
	tests := []ShellTestCase{
		{
			name: "bridge_list",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
		{
			name: "bridge_add_sqlite",
			args: append(shellArgs, "bridge", "add", "br-sqlite", "--type", "sqlite", "file::memory:?cache=shared"),
			expect: []string{
				"Adding bridge... br-sqlite type: sqlite path: file::memory:?cache=shared",
			},
		},
		{
			name: "bridge_list_after_add",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬───────────┬────────┬────────────────────────────┐",
				"│ ROWNUM │ NAME      │ TYPE   │ CONNECTION                 │",
				"├────────┼───────────┼────────┼────────────────────────────┤",
				"│      1 │ br-sqlite │ sqlite │ file::memory:?cache=shared │",
				"└────────┴───────────┴────────┴────────────────────────────┘",
			},
		},
		{
			name: "bridge_test_sqlite",
			args: append(shellArgs, "bridge", "test", "br-sqlite"),
			expect: []string{
				"Testing bridge... br-sqlite",
				"OK.",
			},
		},
		{
			name: "bridge_exec_sqlite_create_table",
			args: append(shellArgs, "bridge", "exec", "br-sqlite", "CREATE TABLE IF NOT EXISTS ids(id INTEGER NOT NULL PRIMARY KEY, memo TEXT)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_sqlite_insert_1",
			args: append(shellArgs, "bridge", "exec", "br-sqlite", "INSERT INTO ids(id, memo) VALUES(1, 'test-1')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_sqlite_insert_2",
			args: append(shellArgs, "bridge", "exec", "br-sqlite", "INSERT INTO ids(id, memo) VALUES(2, 'test-2')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_sqlite_query",
			args: append(shellArgs, "bridge", "query", "br-sqlite", "SELECT * FROM ids ORDER BY id"),
			expect: []string{
				"┌────────┬────┬────────┐",
				"│ ROWNUM │ ID │ MEMO   │",
				"├────────┼────┼────────┤",
				"│      1 │  1 │ test-1 │",
				"│      2 │  2 │ test-2 │",
				"└────────┴────┴────────┘",
			},
		},
		{
			name: "bridge_del_sqlite",
			args: append(shellArgs, "bridge", "del", "br-sqlite"),
			expect: []string{
				"Deleted.",
			},
		},
		{
			name: "bridge_list_after_del",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func shellBridgePostgresTest(t *testing.T, dsn string) {
	tests := []ShellTestCase{
		{
			name: "bridge_list",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
		{
			name: "bridge_add_postgres",
			args: append(shellArgs, "bridge", "add", "br-postgres", "--type", "postgres", dsn),
			expect: []string{
				"Adding bridge... br-postgres type: postgres path: " + dsn,
			},
		},
		{
			name: "bridge_list_after_add",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬─────────────┬──────────┬─────────────────────────────────────────────────────────────────────────────────┐",
				"│ ROWNUM │ NAME        │ TYPE     │ CONNECTION                                                                      │",
				"├────────┼─────────────┼──────────┼─────────────────────────────────────────────────────────────────────────────────┤",
				"│      1 │ br-postgres │ postgres │ " + dsn + " │",
				"└────────┴─────────────┴──────────┴─────────────────────────────────────────────────────────────────────────────────┘",
			},
		},
		{
			name: "bridge_test_postgres",
			args: append(shellArgs, "bridge", "test", "br-postgres"),
			expect: []string{
				"Testing bridge... br-postgres",
				"OK.",
			},
		},
		{
			name: "bridge_exec_postgres_create_table",
			args: append(shellArgs, "bridge", "exec", "br-postgres", "CREATE TABLE IF NOT EXISTS ids(id SERIAL PRIMARY KEY, memo TEXT)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_insert_1",
			args: append(shellArgs, "bridge", "exec", "br-postgres", "INSERT INTO ids(memo) VALUES('pg-1')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_insert_2",
			args: append(shellArgs, "bridge", "exec", "br-postgres", "INSERT INTO ids(memo) VALUES('pg-2')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_query",
			args: append(shellArgs, "bridge", "query", "br-postgres", "SELECT * FROM ids ORDER BY id"),
			expect: []string{
				"┌────────┬────┬──────┐",
				"│ ROWNUM │ ID │ MEMO │",
				"├────────┼────┼──────┤",
				"│      1 │  1 │ pg-1 │",
				"│      2 │  2 │ pg-2 │",
				"└────────┴────┴──────┘",
			},
		},
		{
			name: "bridge_exec_postgres_query_no_rows",
			args: append(shellArgs, "bridge", "query", "br-postgres", "SELECT * FROM ids WHERE id < 0 ORDER BY id"),
			expect: []string{
				"┌────────┬────┬──────┐",
				"│ ROWNUM │ ID │ MEMO │",
				"├────────┼────┼──────┤",
				"└────────┴────┴──────┘",
			},
		},
		{
			name: "bridge_del_postgres",
			args: append(shellArgs, "bridge", "del", "br-postgres"),
			expect: []string{
				"Deleted.",
			},
		},
		{
			name: "bridge_list_after_del",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func shellBridgeMSSqlTest(t *testing.T, dsn string) {
	tests := []ShellTestCase{
		{
			name: "bridge_list",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
		{
			name: "bridge_add_mssql",
			args: append(shellArgs, "bridge", "add", "br-ms", "--type", "mssql", dsn),
			expect: []string{
				"Adding bridge... br-ms type: mssql path: " + dsn,
			},
		},
		{
			name: "bridge_list_after_add",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬───────┬───────┬──────────────────────────────────────────────────────────────────────────────────────────┐",
				"│ ROWNUM │ NAME  │ TYPE  │ CONNECTION                                                                               │",
				"├────────┼───────┼───────┼──────────────────────────────────────────────────────────────────────────────────────────┤",
				"│      1 │ br-ms │ mssql │ " + dsn + " │",
				"└────────┴───────┴───────┴──────────────────────────────────────────────────────────────────────────────────────────┘",
			},
		},
		{
			name: "bridge_test_mssql",
			args: append(shellArgs, "bridge", "test", "br-ms"),
			expect: []string{
				"Testing bridge... br-ms",
				"OK.",
			},
		},
		{
			name: "bridge_exec_mssql_create_table",
			args: append(shellArgs, "bridge", "exec", "br-ms", "CREATE TABLE ids(id INT NOT NULL PRIMARY KEY, memo TEXT)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mssql_insert_1",
			args: append(shellArgs, "bridge", "exec", "br-ms", "INSERT INTO ids(id, memo) VALUES(1, 'ms-1')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mssql_insert_2",
			args: append(shellArgs, "bridge", "exec", "br-ms", "INSERT INTO ids(id, memo) VALUES(2, 'ms-2')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mssql_query",
			args: append(shellArgs, "bridge", "query", "br-ms", "SELECT * FROM ids ORDER BY id"),
			expect: []string{
				"┌────────┬────┬──────┐",
				"│ ROWNUM │ ID │ MEMO │",
				"├────────┼────┼──────┤",
				"│      1 │  1 │ ms-1 │",
				"│      2 │  2 │ ms-2 │",
				"└────────┴────┴──────┘",
			},
		},
		{
			name: "bridge_exec_mssql_query_no_rows",
			args: append(shellArgs, "bridge", "query", "br-ms", "SELECT * FROM ids WHERE id < 0 ORDER BY id"),
			expect: []string{
				"┌────────┬────┬──────┐",
				"│ ROWNUM │ ID │ MEMO │",
				"├────────┼────┼──────┤",
				"└────────┴────┴──────┘",
			},
		},
		{
			name: "bridge_del_mssql",
			args: append(shellArgs, "bridge", "del", "br-ms"),
			expect: []string{
				"Deleted.",
			},
		},
		{
			name: "bridge_list_after_del",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func shellBridgeMySqlTest(t *testing.T, dsn string) {
	tests := []ShellTestCase{
		{
			name: "bridge_list",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
		{
			name: "bridge_add_mysql",
			args: append(shellArgs, "bridge", "add", "br-my", "--type", "mysql", dsn),
			expect: []string{
				"Adding bridge... br-my type: mysql path: " + dsn,
			},
		},
		{
			name: "bridge_list_after_add",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬───────┬───────┬──────────────────────────────────────────────────────┐",
				"│ ROWNUM │ NAME  │ TYPE  │ CONNECTION                                           │",
				"├────────┼───────┼───────┼──────────────────────────────────────────────────────┤",
				"│      1 │ br-my │ mysql │ " + dsn + " │",
				"└────────┴───────┴───────┴──────────────────────────────────────────────────────┘",
			},
		},
		{
			name: "bridge_test_mysql",
			args: append(shellArgs, "bridge", "test", "br-my"),
			expect: []string{
				"Testing bridge... br-my",
				"OK.",
			},
		},
		{
			name: "bridge_exec_mysql_create_table",
			args: append(shellArgs, "bridge", "exec", "br-my", "CREATE TABLE IF NOT EXISTS ids(id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, memo TEXT)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mysql_insert_1",
			args: append(shellArgs, "bridge", "exec", "br-my", "INSERT INTO ids(memo) VALUES('my-1')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mysql_insert_2",
			args: append(shellArgs, "bridge", "exec", "br-my", "INSERT INTO ids(memo) VALUES('my-2')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mysql_query",
			args: append(shellArgs, "bridge", "query", "br-my", "SELECT * FROM ids ORDER BY id"),
			expect: []string{
				"┌────────┬────┬──────┐",
				"│ ROWNUM │ ID │ MEMO │",
				"├────────┼────┼──────┤",
				"│      1 │  1 │ my-1 │",
				"│      2 │  2 │ my-2 │",
				"└────────┴────┴──────┘",
			},
		},
		{
			name: "bridge_exec_mysql_query_no_rows",
			args: append(shellArgs, "bridge", "query", "br-my", "SELECT * FROM ids WHERE id < 0 ORDER BY id"),
			expect: []string{
				"┌────────┬────┬──────┐",
				"│ ROWNUM │ ID │ MEMO │",
				"├────────┼────┼──────┤",
				"└────────┴────┴──────┘",
			},
		},
		{
			name: "bridge_del_mysql",
			args: append(shellArgs, "bridge", "del", "br-my"),
			expect: []string{
				"Deleted.",
			},
		},
		{
			name: "bridge_list_after_del",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func shellBridgeMqttTest(t *testing.T, broker string) {
	tests := []ShellTestCase{
		{
			name: "bridge_list",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
		{
			name: "bridge_add_mqtt",
			args: append(shellArgs, "bridge", "add", "br-mqtt", "--type", "mqtt", fmt.Sprintf("broker=%s", broker)),
			expect: []string{
				"Adding bridge... br-mqtt type: mqtt path: " + fmt.Sprintf("broker=%s", broker),
			},
		},
		{
			name: "bridge_list_after_add",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬─────────┬──────┬────────────────────────┐",
				"│ ROWNUM │ NAME    │ TYPE │ CONNECTION             │",
				"├────────┼─────────┼──────┼────────────────────────┤",
				"│      1 │ br-mqtt │ mqtt │ broker=" + broker + " │",
				"└────────┴─────────┴──────┴────────────────────────┘",
			},
		},
		{
			name: "subscriber_add",
			args: append(shellArgs, "subscriber", "add", "--autostart", "--qos", "1", "sub-mqtt", "br-mqtt", "test/topic", "db/write/example"),
			expect: []string{
				"Subscriber 'sub-mqtt' added successfully.",
			},
		},
		{
			name:   "wait_for_mqtt_subscribe",
			args:   append(shellArgs, "sleep", "3"), // wait for data to arrive and be processed
			expect: []string{},
		},
		{
			name: "subscriber_list_after_add",
			args: append(shellArgs, "subscriber", "list"),
			expect: []string{
				"┌────────┬──────────┬─────────┬────────────┬──────────────────┬───────────┬─────────┐",
				"│ ROWNUM │ NAME     │ BRIDGE  │ TOPIC      │ DESTINATION      │ AUTOSTART │ STATE   │",
				"├────────┼──────────┼─────────┼────────────┼──────────────────┼───────────┼─────────┤",
				"│      1 │ SUB-MQTT │ br-mqtt │ test/topic │ db/write/example │ YES       │ RUNNING │",
				"└────────┴──────────┴─────────┴────────────┴──────────────────┴───────────┴─────────┘",
			},
		},
		{
			name: "mqtt_pub",
			args: append(shellArgs, "mqtt_pub",
				"--broker", broker,
				"--topic", "test/topic",
				"--message", `[["mqtt-test",1773466141000000000,42],["mqtt-test",1773466142000000000,43]]`),
			expect: []string{},
		},
		{
			name:   "wait_for_mqtt_publish",
			args:   append(shellArgs, "sleep", "3"), // wait for data to arrive and be processed
			expect: []string{},
		},
		{
			name: "mqtt_pub_result",
			args: append(shellArgs, "sql", "--tz", "GMT", "SELECT * FROM example WHERE name='mqtt-test' ORDER BY time"),
			expect: []string{
				"┌────────┬───────────┬─────────────────────┬───────┐",
				"│ ROWNUM │ NAME      │ TIME                │ VALUE │",
				"├────────┼───────────┼─────────────────────┼───────┤",
				"│      1 │ mqtt-test │ 2026-03-14 05:29:01 │    42 │",
				"│      2 │ mqtt-test │ 2026-03-14 05:29:02 │    43 │",
				"└────────┴───────────┴─────────────────────┴───────┘",
				"2 rows selected.",
			},
		},
		{
			name: "mqtt_pub_clean",
			args: append(shellArgs, "sql", "DELETE FROM example WHERE name='mqtt-test'"),
			expect: []string{
				"2 rows deleted.",
			},
		},
		{
			name: "subscriber_stop",
			args: append(shellArgs, "subscriber", "stop", "sub-mqtt"),
			expect: []string{
				"Subscriber 'sub-mqtt' stopped successfully.",
			},
		},
		{
			name: "subscriber_list_after_stop",
			args: append(shellArgs, "subscriber", "list"),
			expect: []string{
				"┌────────┬──────────┬─────────┬────────────┬──────────────────┬───────────┬───────┐",
				"│ ROWNUM │ NAME     │ BRIDGE  │ TOPIC      │ DESTINATION      │ AUTOSTART │ STATE │",
				"├────────┼──────────┼─────────┼────────────┼──────────────────┼───────────┼───────┤",
				"│      1 │ SUB-MQTT │ br-mqtt │ test/topic │ db/write/example │ YES       │ STOP  │",
				"└────────┴──────────┴─────────┴────────────┴──────────────────┴───────────┴───────┘",
			},
		},
		{
			name: "subscriber_del",
			args: append(shellArgs, "subscriber", "del", "sub-mqtt"),
			expect: []string{
				"Subscriber 'sub-mqtt' deleted successfully.",
			},
		},
		{
			name: "bridge_del_mqtt",
			args: append(shellArgs, "bridge", "del", "br-mqtt"),
			expect: []string{
				"Deleted.",
			},
		},
		{
			name: "bridge_list_after_del",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func shellBridgeNatsTest(t *testing.T, natsHostPort string) {
	tests := []ShellTestCase{
		{
			name: "bridge_list",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
		{
			name: "bridge_add_nats",
			args: append(shellArgs, "bridge", "add", "br-nats", "--type", "nats", fmt.Sprintf("server=%s name=nasts-client", natsHostPort)),
			expect: []string{
				"Adding bridge... br-nats type: nats path: " + fmt.Sprintf("server=%s name=nasts-client", natsHostPort),
			},
		},
		{
			name: "bridge_list_after_add",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬─────────┬──────┬──────────────────────────────────────────┐",
				"│ ROWNUM │ NAME    │ TYPE │ CONNECTION                               │",
				"├────────┼─────────┼──────┼──────────────────────────────────────────┤",
				"│      1 │ br-nats │ nats │ server=" + natsHostPort + " name=nasts-client │",
				"└────────┴─────────┴──────┴──────────────────────────────────────────┘",
			},
		},
		{
			name: "subscriber_add",
			args: append(shellArgs, "subscriber", "add", "--autostart", "sub-nats", "br-nats", "iot.sensor", "db/write/example"),
			expect: []string{
				"Subscriber 'sub-nats' added successfully.",
			},
		},
		{
			name:   "wait_for_nats_subscribe", // wait for subscriber to start and subscribe before publishing
			args:   append(shellArgs, "sleep", "3"),
			expect: []string{},
		},
		{
			name: "subscriber_list_after_add",
			args: append(shellArgs, "subscriber", "list"),
			expect: []string{
				"┌────────┬──────────┬─────────┬────────────┬──────────────────┬───────────┬─────────┐",
				"│ ROWNUM │ NAME     │ BRIDGE  │ TOPIC      │ DESTINATION      │ AUTOSTART │ STATE   │",
				"├────────┼──────────┼─────────┼────────────┼──────────────────┼───────────┼─────────┤",
				"│      1 │ SUB-NATS │ br-nats │ iot.sensor │ db/write/example │ YES       │ RUNNING │",
				"└────────┴──────────┴─────────┴────────────┴──────────────────┴───────────┴─────────┘",
			},
		},
		{
			name: "nats_pub",
			args: append(shellArgs, "nats_pub",
				"--broker", natsHostPort,
				"--topic", "iot.sensor",
				"--message", `[["nats-test",1773466141000000000,42],["nats-test",1773466142000000000,43]]`),
			expect: []string{},
		},
		{
			name:   "wait_for_nats_publish",
			args:   append(shellArgs, "sleep", "3"), // wait for data to arrive and be processed
			expect: []string{},
		},
		{
			name: "nats_pub_result",
			args: append(shellArgs, "sql", "--tz", "GMT", "SELECT * FROM example WHERE name='nats-test' ORDER BY time"),
			expect: []string{
				"┌────────┬───────────┬─────────────────────┬───────┐",
				"│ ROWNUM │ NAME      │ TIME                │ VALUE │",
				"├────────┼───────────┼─────────────────────┼───────┤",
				"│      1 │ nats-test │ 2026-03-14 05:29:01 │    42 │",
				"│      2 │ nats-test │ 2026-03-14 05:29:02 │    43 │",
				"└────────┴───────────┴─────────────────────┴───────┘",
				"2 rows selected.",
			},
		},
		{
			name: "nats_pub_clean",
			args: append(shellArgs, "sql", "DELETE FROM example WHERE name='nats-test'"),
			expect: []string{
				"2 rows deleted.",
			},
		},
		{
			name: "subscriber_stop",
			args: append(shellArgs, "subscriber", "stop", "sub-nats"),
			expect: []string{
				"Subscriber 'sub-nats' stopped successfully.",
			},
		},
		{
			name: "subscriber_list_after_stop",
			args: append(shellArgs, "subscriber", "list"),
			expect: []string{
				"┌────────┬──────────┬─────────┬────────────┬──────────────────┬───────────┬───────┐",
				"│ ROWNUM │ NAME     │ BRIDGE  │ TOPIC      │ DESTINATION      │ AUTOSTART │ STATE │",
				"├────────┼──────────┼─────────┼────────────┼──────────────────┼───────────┼───────┤",
				"│      1 │ SUB-NATS │ br-nats │ iot.sensor │ db/write/example │ YES       │ STOP  │",
				"└────────┴──────────┴─────────┴────────────┴──────────────────┴───────────┴───────┘",
			},
		},
		{
			name: "subscriber_del",
			args: append(shellArgs, "subscriber", "del", "sub-nats"),
			expect: []string{
				"Subscriber 'sub-nats' deleted successfully.",
			},
		},
		{
			name: "bridge_del_nats",
			args: append(shellArgs, "bridge", "del", "br-nats"),
			expect: []string{
				"Deleted.",
			},
		},
		{
			name: "bridge_list_after_del",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func TestShellTimer(t *testing.T) {
	tests := []ShellTestCase{
		{
			name: "timer_list",
			args: append(shellArgs, "timer", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬─────┬───────────┬───────┐",
				"│ ROWNUM │ NAME │ SPEC │ TQL │ AUTOSTART │ STATE │",
				"├────────┼──────┼──────┼─────┼───────────┼───────┤",
				"└────────┴──────┴──────┴─────┴───────────┴───────┘",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func TestShellKey(t *testing.T) {
	tests := []ShellTestCase{
		{
			name: "key_list",
			args: append(shellArgs, "key", "list"),
			expect: []string{
				"┌────────┬────┬──────────────────┬─────────────────┐",
				"│ ROWNUM │ ID │ NOT VALID BEFORE │ NOT VALID AFTER │",
				"├────────┼────┼──────────────────┼─────────────────┤",
				"└────────┴────┴──────────────────┴─────────────────┘",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func TestShellRun(t *testing.T) {
	tests := []ShellTestCase{
		{
			name:      "run_invalid_command",
			args:      append(shellArgs, "run", "invalid-command"),
			expectErr: "ENOENT: no such file or directory, open '/work/invalid-command'",
		},
		{
			name: "shell_run.txt",
			args: append(shellArgs, "run", "shell_run.txt"),
			expect: []string{
				"SHOW TABLES",
				"┌────────┬───────────────┬───────────┬────────────┬──────────┬────────────┬────────────┐",
				"│ ROWNUM │ DATABASE_NAME │ USER_NAME │ TABLE_NAME │ TABLE_ID │ TABLE_TYPE │ TABLE_FLAG │",
				"├────────┼───────────────┼───────────┼────────────┼──────────┼────────────┼────────────┤",
				"│      1 │ MACHBASEDB    │ SYS       │ EXAMPLE    │       19 │ Tag        │            │",
				"│      2 │ MACHBASEDB    │ SYS       │ LOG_DATA   │       13 │ Log        │            │",
				"│      3 │ MACHBASEDB    │ SYS       │ TAG_DATA   │        6 │ Tag        │            │",
				"│      4 │ MACHBASEDB    │ SYS       │ TAG_SIMPLE │       12 │ Tag        │            │",
				"└────────┴───────────────┴───────────┴────────────┴──────────┴────────────┴────────────┘",
				"",
				"INSERT INTO EXAMPLE VALUES('shell_run', 1773722371000000000, 1.234)",
				"a row inserted.",
				"",
				"exec table_flush(example)",
				"rollup executed.",
				"",
				"sql --timeformat kitchen --tz Asia/Seoul -Z --no-pause",
				"SELECT",
				"    *",
				"FROM",
				"    EXAMPLE",
				"WHERE",
				"    NAME = 'shell_run'",
				"┌────────┬───────────┬──────────────────┬───────┐",
				"│ ROWNUM │ NAME      │ TIME(ASIA/SEOUL) │ VALUE │",
				"├────────┼───────────┼──────────────────┼───────┤",
				"│      1 │ shell_run │ 1:39PM           │ 1.234 │",
				"└────────┴───────────┴──────────────────┴───────┘",
				"a row selected.",
				"",
				"DELETE FROM EXAMPLE WHERE NAME = 'shell_run'",
				"a row deleted.",
				"",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}
