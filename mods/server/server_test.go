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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/test"
	"github.com/ory/dockertest/v4"
	"github.com/stretchr/testify/require"
)

var projRootDir string
var testTimeTick = time.Unix(1705291859, 0)

var machServerAddress = ""

var mqttServer *mqttd
var mqttServerAddress = ""

var httpServer *httpd
var httpServerAddress = ""

var shellPort = 15622

var shellArgs = []string{}

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_DO_RESTORE_HELPER") == "1" {
		os.Exit(m.Run())
	}

	// get project root based current test case file path
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Failed to get current file path")
	}
	projRootDir = filepath.Join(filepath.Dir(filename), "../../")

	prefDir := filepath.Join(projRootDir, "tmp", "test", "pref")
	fileDir := filepath.Join(projRootDir, "mods", "server", "test")
	dataDir := filepath.Join(projRootDir, "tmp", "test", "machbase_home")
	binPath := filepath.Join(projRootDir, "tmp", "machbase-neo")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	// cleanup pref and files directories before test
	os.RemoveAll(prefDir)
	os.RemoveAll(dataDir)

	machPort := 15656
	grpcPort := 15655
	httpPort := 15654
	mqttPort := 15653
	machServerAddress = fmt.Sprintf("tcp://127.0.0.1:%d", machPort)
	httpServerAddress = fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	mqttServerAddress = fmt.Sprintf("127.0.0.1:%d", mqttPort)

	var server *Server
	go func() {
		Main([]string{binPath,
			"serve",
			"--data", dataDir,
			"--file", fileDir,
			"--pref", prefDir,
			"--mach-port", strconv.Itoa(machPort),
			"--grpc-port", strconv.Itoa(grpcPort),
			"--http-port", strconv.Itoa(httpPort),
			"--mqtt-port", strconv.Itoa(mqttPort),
			"--shell-port", strconv.Itoa(shellPort),
			"--jwt-secret", "__secr3t__",
			"--log-filename", "-",
			"--log-level", "INFO",
		})
	}()
	<-serverAfterStartC
	if b := booter.GetInstance("machbase.com/neo-server"); b != nil {
		server = b.(*Server)
	} else {
		panic("failed to get server instance from booter")
	}

	server.binExecutable = binPath
	httpServer = server.httpd
	mqttServer = server.mqttd

	// build shell binary for shell tests
	func() {
		buildShellCmd := []string{
			"go", "build", "-o", binPath, filepath.Join(projRootDir, "cmd", "machbase-neo"),
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
			"-v", fmt.Sprintf("/work=%s", fileDir),
		}
		server.models.ShellProvider().SetDefaultShellCommand(
			fmt.Sprintf("%q shell --server %s --user sys --password manager -v %q", binPath, httpServerAddress, fmt.Sprintf("/work=%s", fileDir)),
		)
		server.models.ShellProvider().SetDefaultJshCommand(
			fmt.Sprintf("%q jsh -v %q", binPath, fmt.Sprintf("/work=%s", fileDir)),
		)
	}()

	func(db api.Database) {
		ctx := context.TODO()
		conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		result := conn.Exec(ctx, `CREATE TAG TABLE TAG_DATA(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double summarized,
			short_value     short,
			ushort_value    ushort,
			int_value       integer,
			uint_value 	    uinteger,
			long_value      long,
			ulong_value 	ulong,
			str_value       varchar(400),
			json_value      json,
			ipv4_value      ipv4,
			ipv6_value      ipv6,
			bin_value		binary
		) TAG_PARTITION_COUNT=1`)
		if result.Err() != nil {
			panic(result.Err())
		}

		result = conn.Exec(ctx, `CREATE TABLE LOG_DATA(
			time datetime,
			short_value short,
			ushort_value ushort,
			int_value integer,
			uint_value uinteger,
			long_value long,
			ulong_value ulong,
			double_value double,
			float_value float,
			str_value varchar(400),
			json_value json,
			ipv4_value ipv4,
			ipv6_value ipv6,
			text_value text,
			bin_value binary
		)`)
		if result.Err() != nil {
			panic(result.Err())
		}

		result = conn.Exec(ctx, `CREATE TAG TABLE example (
			name VARCHAR(40) PRIMARY KEY,
			time DATETIME BASETIME,
			value DOUBLE SUMMARIZED
		) TAG_PARTITION_COUNT=1, TAG_DUPLICATE_CHECK_DURATION=1`)
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
	}(api.Default())

	// run tests
	m.Run()

	// cleanup
	booter.NotifySignal()
	<-serverBeforeStopC
}

func TestDoRestore(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestDoRestoreHelper$")
	cmd.Env = append(os.Environ(), "GO_WANT_DO_RESTORE_HELPER=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	if len(output) > 0 {
		t.Log(string(output))
	}
}

func TestDoRestoreHelper(t *testing.T) {
	if os.Getenv("GO_WANT_DO_RESTORE_HELPER") != "1" {
		t.Skip("helper test")
	}

	restoreCmd := &RestoreCmd{
		DataDir:   "/definitely/not/real",
		BackupDir: "/definitely/not/real/backup",
	}

	require.Equal(t, -1, doRestore(restoreCmd))
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

func TestWithHttpAuthServerSharesRpcController(t *testing.T) {
	authSvc := &Server{
		rpcController: &service.Controller{},
	}
	authSvc.registerJsonRpcHandlers()

	httpSvc, err := NewHttp(nil, WithHttpAuthServer(authSvc, false))
	require.NoError(t, err)
	require.Same(t, authSvc.rpcController, httpSvc.rpcController)

	result, rpcErr := httpSvc.rpcController.CallJsonRpc("markdown.render", []any{"# Hello", false}, nil)
	require.Nil(t, rpcErr)
	require.Contains(t, result.(string), "Hello")
}

func TestGetBestMachPortPrefersRemoteAddress(t *testing.T) {
	svr := &Server{
		servicePorts: map[string][]*model.ServicePort{
			"mach": {
				{Service: "mach", Address: "tcp://127.0.0.1:5656"},
				{Service: "mach", Address: "tcp://192.168.0.10:5656"},
			},
		},
	}

	host, port, err := svr.getBestMachPort()
	require.NoError(t, err)
	require.Equal(t, "192.168.0.10", host)
	require.Equal(t, 5656, port)
}

func TestGetBestMachPortFallsBackToLoopback(t *testing.T) {
	svr := &Server{
		servicePorts: map[string][]*model.ServicePort{
			"mach": {
				{Service: "mach", Address: "tcp://127.0.0.1:5656"},
			},
		},
	}

	host, port, err := svr.getBestMachPort()
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", host)
	require.Equal(t, 5656, port)
}

func TestGetBestMachPortReturnsErrorWhenUnavailable(t *testing.T) {
	svr := &Server{
		servicePorts: map[string][]*model.ServicePort{},
	}

	_, _, err := svr.getBestMachPort()
	require.Error(t, err)
}

func TestGetBestMachPortSkipsInvalidEntries(t *testing.T) {
	svr := &Server{
		servicePorts: map[string][]*model.ServicePort{
			"mach": {
				{Service: "mach", Address: "tcp://bad-host"},
				{Service: "mach", Address: "unix:///tmp/mach.sock"},
				{Service: "mach", Address: "tcp://127.0.0.1:5656"},
			},
		},
	}

	host, port, err := svr.getBestMachPort()
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", host)
	require.Equal(t, 5656, port)
}

func TestScoreMachServiceAddressAndHost(t *testing.T) {
	ifAddrs := []*util.InterfaceAddr{
		{IP: net.ParseIP("10.0.0.7"), Flags: net.FlagUp},
		{IP: net.ParseIP("2001:db8::5"), Flags: net.FlagUp},
		{IP: net.ParseIP("169.254.1.2"), Flags: net.FlagUp},
	}

	host, port, score, ok := scoreMachServiceAddress("tcp://10.0.0.7:5656", ifAddrs)
	require.True(t, ok)
	require.Equal(t, "10.0.0.7", host)
	require.Equal(t, 5656, port)
	require.Equal(t, 5, score)

	_, _, _, ok = scoreMachServiceAddress("tcp://bad-host", ifAddrs)
	require.False(t, ok)

	_, _, _, ok = scoreMachServiceAddress("unix:///tmp/mach.sock", ifAddrs)
	require.False(t, ok)

	require.Equal(t, 100, scoreMachHost("", ifAddrs))
	require.Equal(t, 90, scoreMachHost("localhost", ifAddrs))
	require.Equal(t, 30, scoreMachHost("db.internal", ifAddrs))
	require.Equal(t, 90, scoreMachHost("127.0.0.1", ifAddrs))
	require.Equal(t, 95, scoreMachHost("0.0.0.0", ifAddrs))
	require.Equal(t, 70, scoreMachHost("169.254.9.9", ifAddrs))
	require.Equal(t, 5, scoreMachHost("10.0.0.7", ifAddrs))
	require.Equal(t, 10, scoreMachHost("2001:db8::5", ifAddrs))
}

func TestWriteSharedInfo(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "services"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "shared-backend"), 0o755))

	ctl, err := service.NewController(&service.ControllerConfig{
		ConfigDir: "/work/services",
		SharedFS: service.ControllerSharedFSConfig{
			BackendDir: "/work/shared-backend",
		},
		Mounts: []engine.FSTab{
			{MountPoint: "/work", FS: os.DirFS(tmpDir)},
		},
	})
	require.NoError(t, err)

	svr := &Server{serviceController: ctl}

	require.NoError(t, svr.writeSharedInfo("/share/message.txt", "hello"))
	require.NoError(t, svr.writeSharedInfo("/share/config.json", map[string]any{
		"user": "sys",
		"port": 5656,
	}))

	message, err := os.ReadFile(filepath.Join(tmpDir, "shared-backend", "share", "message.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(message))

	configBody, err := os.ReadFile(filepath.Join(tmpDir, "shared-backend", "share", "config.json"))
	require.NoError(t, err)
	require.Contains(t, string(configBody), "\"user\": \"sys\"")
	require.Contains(t, string(configBody), "\"port\": 5656")
}

func TestWriteSharedInfoWithoutBackend(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "services"), 0o755))

	ctl, err := service.NewController(&service.ControllerConfig{
		ConfigDir: "/work/services",
		Mounts: []engine.FSTab{
			{MountPoint: "/work", FS: os.DirFS(tmpDir)},
		},
	})
	require.NoError(t, err)
	require.NoError(t, ctl.Start(nil))
	defer ctl.Stop(nil)

	svr := &Server{serviceController: ctl}
	require.NoError(t, svr.writeSharedInfo("/share/db.json", map[string]any{
		"host": "127.0.0.1",
		"port": 5656,
		"user": "sys",
	}))

	cfs, err := engine.NewControllerFS(ctl.Address())
	require.NoError(t, err)
	defer cfs.Close()

	body, err := cfs.ReadFile("/share/db.json")
	require.NoError(t, err)
	require.Contains(t, string(body), "\"host\": \"127.0.0.1\"")
	require.Contains(t, string(body), "\"port\": 5656")
	require.Contains(t, string(body), "\"user\": \"sys\"")
}

type ShellTestCase struct {
	name       string
	args       []string
	expect     []string
	expectErr  string
	expectFunc func(t *testing.T, output string) error
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
		if tt.expectFunc != nil {
			require.NoError(t, tt.expectFunc(t, string(output)), "Output did not satisfy expectation function")
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

func TestSharedInfo(t *testing.T) {
	tests := []ShellTestCase{
		{
			name: "share_boot_json",
			args: append(shellArgs, "/sbin/cat", "/proc/share/boot.json"),
			expectFunc: func(t *testing.T, output string) error {
				require.Contains(t, output, `"http": {`)
				require.Contains(t, output, `"mqtt": {`)
				require.Contains(t, output, `"process": {`)
				require.Contains(t, output, `"machbase": {`)
				return nil
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
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
				"1,MACHBASEDB,SYS,EXAMPLE,15,Tag,",
				"2,MACHBASEDB,SYS,LOG_DATA,8,Log,",
				"3,MACHBASEDB,SYS,TAG_DATA,7,Tag,",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}

func TestShellBridge(t *testing.T) {
	if !test.SupportDockerTest() {
		t.Skip("dockertest does not work in this environment")
	}
	// dockertest pool
	pool := dockertest.NewPoolT(t, "")
	//
	// start postgreSQL
	//
	postgresRepository, postgresTag := test.PostgresDockerImage.Resolve()
	postgres := pool.RunT(t, postgresRepository,
		dockertest.WithTag(postgresTag),
		dockertest.WithEnv([]string{
			"POSTGRES_USER=dbuser",
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_DB=db",
		}),
	)
	//
	// start MSSQL
	//
	mssqlImage, mssqlTag := test.MSSQLDockerImage.Resolve()
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
	mysqlRepository, mysqlTag := test.MySQLDockerImage.Resolve()
	mysql := pool.RunT(t, mysqlRepository,
		dockertest.WithTag(mysqlTag),
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

	mosquittoRepository, mosquittoTag := test.MosquittoDockerImage.Resolve()
	mosquitto := pool.RunT(t, mosquittoRepository,
		dockertest.WithTag(mosquittoTag),
		dockertest.WithMounts([]string{filepath.Join(testDir, "mosquitto.conf") + ":/mosquitto/config/mosquitto.conf:ro"}),
	)
	//
	// start NATS server
	//
	natsRepository, natsTag := test.NATSDockerImage.Resolve()
	nats := pool.RunT(t, natsRepository,
		dockertest.WithTag(natsTag),
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
			name: "bridge_exec_sqlite_create_supported_table",
			args: append(shellArgs, "bridge", "exec", "br-sqlite", "CREATE TABLE IF NOT EXISTS typed_ids(id INTEGER NOT NULL PRIMARY KEY, event_bool BOOLEAN, event_integer INTEGER, event_real REAL, event_text TEXT, event_blob BLOB, event_datetime DATETIME)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_sqlite_insert_supported_row",
			args: append(shellArgs, "bridge", "exec", "br-sqlite", "INSERT INTO typed_ids(id, event_bool, event_integer, event_real, event_text, event_blob, event_datetime) VALUES(1, TRUE, 42, 3.25, 'sqlite-text', X'0A0B0C', '2026-03-14 05:29:01')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_sqlite_query_supported_types",
			args: append(shellArgs, "bridge", "query", "br-sqlite", "SELECT id, event_bool, event_integer, event_real, event_text, HEX(event_blob) AS event_blob_hex, strftime('%Y-%m-%d %H:%M:%S', event_datetime) AS event_datetime FROM typed_ids ORDER BY id"),
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ EVENT_BOOL │ EVENT_INTEGER │ EVENT_REAL │ EVENT_TEXT  │ EVENT_BLOB_HEX │ EVENT_DATETIME\\s*│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │ (true|1)\\s+│\\s+42\\s+│\\s+3\\.25\\s+│ sqlite-text │ 0A0B0C\\s+│ 2026-03-14 05:29:01 │$",
				"/r/^└.*┘$",
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
			name: "bridge_exec_postgres_create_supported_table",
			args: append(shellArgs, "bridge", "exec", "br-postgres", "CREATE TABLE IF NOT EXISTS typed_ids(id SERIAL PRIMARY KEY, event_bool BOOLEAN, event_int INTEGER, event_bigint BIGINT, event_real REAL, event_text TEXT, event_uuid UUID, event_date DATE, event_timestamp TIMESTAMP, event_timestamptz TIMESTAMPTZ)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_insert_supported_row",
			args: append(shellArgs, "bridge", "exec", "br-postgres", "INSERT INTO typed_ids(event_bool, event_int, event_bigint, event_real, event_text, event_uuid, event_date, event_timestamp, event_timestamptz) VALUES(TRUE, 42, 4200000000, 3.25, 'pg-text', '550e8400-e29b-41d4-a716-446655440000', DATE '2026-03-14', TIMESTAMP '2026-03-14 05:29:01', TIMESTAMPTZ '2026-03-14 05:29:01+00')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_query_supported_types",
			args: append(shellArgs, "bridge", "query", "br-postgres", "SELECT id, event_bool, event_int, event_bigint, event_real, event_text, event_uuid::text AS event_uuid, TO_CHAR(event_date, 'YYYY-MM-DD') AS event_date, TO_CHAR(event_timestamp, 'YYYY-MM-DD HH24:MI:SS') AS event_timestamp, TO_CHAR(event_timestamptz AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS') AS event_timestamptz FROM typed_ids ORDER BY id"),
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ EVENT_BOOL │ EVENT_INT │ EVENT_BIGINT │ EVENT_REAL │ EVENT_TEXT │ EVENT_UUID\\s+│ EVENT_DATE │ EVENT_TIMESTAMP\\s+│ EVENT_TIMESTAMPTZ\\s+│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │ true\\s+│\\s+42\\s+│\\s+(4200000000|4\\.2e\\+09)\\s+│\\s+3\\.25\\s+│ pg-text\\s+│ 550e8400-e29b-41d4-a716-446655440000 │ 2026-03-14 │ 2026-03-14 05:29:01 │ 2026-03-14 05:29:01 │$",
				"/r/^└.*┘$",
			},
		},
		{
			name: "bridge_exec_postgres_query_timestamp_string",
			args: append(shellArgs, "bridge", "query", "br-postgres", "SELECT id, memo, TO_CHAR(TIMESTAMP '2026-03-14 05:29:01', 'YYYY-MM-DD HH24:MI:SS') AS ts FROM ids WHERE id = 1 ORDER BY id"),
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ MEMO │ TS\\s*│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │ pg-1 │ 2026-03-14 05:29:01 │$",
				"/r/^└.*┘$",
			},
		},
		{
			name: "bridge_exec_postgres_query_null_timestamp",
			args: append(shellArgs, "bridge", "query", "br-postgres", "SELECT id, memo, CAST(NULL AS TIMESTAMP) AS ts FROM ids WHERE id = 1 ORDER BY id"),
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ MEMO │ TS\\s*│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │ pg-1 │ NULL\\s*│$",
				"/r/^└.*┘$",
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
			name: "bridge_exec_mssql_create_supported_table",
			args: append(shellArgs, "bridge", "exec", "br-ms", "CREATE TABLE typed_ids(id INT NOT NULL PRIMARY KEY, event_smallint SMALLINT NULL, event_decimal DECIMAL(10,2) NULL, event_real REAL NULL, event_varchar VARCHAR(100) NULL, event_text TEXT NULL, event_datetime DATETIME NULL)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mssql_insert_supported_row",
			args: append(shellArgs, "bridge", "exec", "br-ms", "INSERT INTO typed_ids(id, event_smallint, event_decimal, event_real, event_varchar, event_text, event_datetime) VALUES(1, 7, 123.45, 9.5, 'ms-varchar', 'ms-text', '2026-03-14 05:29:01')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mssql_query_supported_types",
			args: append(shellArgs, "bridge", "query", "br-ms", "SELECT id, event_smallint, event_decimal, event_real, event_varchar, event_text, CONVERT(VARCHAR(19), event_datetime, 120) AS event_datetime FROM typed_ids ORDER BY id"),
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ EVENT_SMALLINT │ EVENT_DECIMAL │ EVENT_REAL │ EVENT_VARCHAR │ EVENT_TEXT │ EVENT_DATETIME\\s*│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │\\s+7\\s+│\\s+123\\.45\\s+│\\s+9\\.5\\s+│ ms-varchar\\s+│ ms-text\\s+│ 2026-03-14 05:29:01 │$",
				"/r/^└.*┘$",
			},
		},
		{
			name: "bridge_exec_mssql_query_null_datetime",
			args: append(shellArgs, "bridge", "query", "br-ms", "SELECT id, memo, CAST(NULL AS DATETIME) AS dt FROM ids WHERE id = 1 ORDER BY id"),
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ MEMO │ DT\\s*│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │ ms-1 │ NULL\\s*│$",
				"/r/^└.*┘$",
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
			args: append(shellArgs, "bridge", "exec", "br-my", "CREATE TABLE IF NOT EXISTS ids(id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, memo TEXT, event_date DATE, event_datetime DATETIME, event_timestamp TIMESTAMP NULL)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mysql_insert_1",
			args: append(shellArgs, "bridge", "exec", "br-my", "INSERT INTO ids(memo, event_date, event_datetime, event_timestamp) VALUES('my-1', '2026-03-14', '2026-03-14 05:29:01', '2026-03-14 05:29:01')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mysql_insert_2",
			args: append(shellArgs, "bridge", "exec", "br-my", "INSERT INTO ids(memo, event_date, event_datetime, event_timestamp) VALUES('my-2', '2026-03-15', '2026-03-15 06:30:02', '2026-03-15 06:30:02')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mysql_insert_3_null_timestamp",
			args: append(shellArgs, "bridge", "exec", "br-my", "INSERT INTO ids(memo, event_date, event_datetime, event_timestamp) VALUES('my-3', '2026-03-16', '2026-03-16 07:31:03', NULL)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mysql_query",
			args: append(shellArgs, "bridge", "query", "br-my", "SELECT id, memo, DATE_FORMAT(event_date, '%Y-%m-%d') AS dt, DATE_FORMAT(event_datetime, '%Y-%m-%d %H:%i:%s') AS dttm, DATE_FORMAT(event_timestamp, '%Y-%m-%d %H:%i:%s') AS ts FROM ids ORDER BY id"),
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ MEMO │ DT\\s+│ DTTM\\s+│ TS\\s+│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │ my-1 │ 2026-03-14 │ 2026-03-14 05:29:01 │ 2026-03-14 05:29:01 │$",
				"/r/^│\\s+2 │\\s+2 │ my-2 │ 2026-03-15 │ 2026-03-15 06:30:02 │ 2026-03-15 06:30:02 │$",
				"/r/^│\\s+3 │\\s+3 │ my-3 │ 2026-03-16 │ 2026-03-16 07:31:03 │ NULL\\s*│$",
				"/r/^└.*┘$",
			},
		},
		{
			name: "bridge_exec_mysql_create_supported_table",
			args: append(shellArgs, "bridge", "exec", "br-my", "CREATE TABLE IF NOT EXISTS typed_ids(id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, event_bigint BIGINT, event_int INT, event_smallint SMALLINT, event_double DOUBLE, event_varchar VARCHAR(64), event_char CHAR(4), event_text TEXT, event_blob BLOB, event_date DATE, event_datetime DATETIME, event_timestamp TIMESTAMP NULL)"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mysql_insert_supported_row",
			args: append(shellArgs, "bridge", "exec", "br-my", "INSERT INTO typed_ids(event_bigint, event_int, event_smallint, event_double, event_varchar, event_char, event_text, event_blob, event_date, event_datetime, event_timestamp) VALUES(4200000000, 123456, 12, 3.5, 'my-varchar', 'ABCD', 'my-text', X'010203', '2026-03-14', '2026-03-14 05:29:01', '2026-03-14 05:29:01')"),
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_mysql_query_supported_types",
			args: append(shellArgs, "bridge", "query", "br-my", "SELECT id, event_bigint, event_int, event_smallint, event_double, event_varchar, event_char, event_text, TO_BASE64(event_blob) AS event_blob_b64, DATE_FORMAT(event_date, '%Y-%m-%d') AS event_date, DATE_FORMAT(event_datetime, '%Y-%m-%d %H:%i:%s') AS event_datetime, DATE_FORMAT(event_timestamp, '%Y-%m-%d %H:%i:%s') AS event_timestamp FROM typed_ids ORDER BY id"),
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ EVENT_BIGINT │ EVENT_INT │ EVENT_SMALLINT │ EVENT_DOUBLE │ EVENT_VARCHAR │ EVENT_CHAR │ EVENT_TEXT │ EVENT_BLOB_B64 │ EVENT_DATE │ EVENT_DATETIME\\s+│ EVENT_TIMESTAMP\\s+│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │\\s+(4200000000|4\\.2e\\+09)\\s+│\\s+123456\\s+│\\s+12\\s+│\\s+3\\.5\\s+│ my-varchar\\s+│ ABCD\\s+│ my-text\\s+│ QVFJRA==\\s+│ 2026-03-14 │ 2026-03-14 05:29:01 │ 2026-03-14 05:29:01 │$",
				"/r/^└.*┘$",
			},
		},
		{
			name: "bridge_exec_mysql_query_no_rows",
			args: append(shellArgs, "bridge", "query", "br-my", "SELECT id, memo, DATE_FORMAT(event_date, '%Y-%m-%d') AS dt, DATE_FORMAT(event_datetime, '%Y-%m-%d %H:%i:%s') AS dttm, DATE_FORMAT(event_timestamp, '%Y-%m-%d %H:%i:%s') AS ts FROM ids WHERE id < 0 ORDER BY id"),
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ MEMO │ DT\\s*│ DTTM\\s*│ TS\\s*│$",
				"/r/^├.*┤$",
				"/r/^└.*┘$",
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
			args: append(shellArgs, "bridge", "add", "br-nats", "--type", "nats", fmt.Sprintf("server=%s name=nats-client", natsHostPort)),
			expect: []string{
				"Adding bridge... br-nats type: nats path: " + fmt.Sprintf("server=%s name=nats-client", natsHostPort),
			},
		},
		{
			name: "bridge_list_after_add",
			args: append(shellArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬─────────┬──────┬─────────────────────────────────────────┐",
				"│ ROWNUM │ NAME    │ TYPE │ CONNECTION                              │",
				"├────────┼─────────┼──────┼─────────────────────────────────────────┤",
				"│      1 │ br-nats │ nats │ server=" + natsHostPort + " name=nats-client │",
				"└────────┴─────────┴──────┴─────────────────────────────────────────┘",
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
				"│      1 │ MACHBASEDB    │ SYS       │ EXAMPLE    │       15 │ Tag        │            │",
				"│      2 │ MACHBASEDB    │ SYS       │ LOG_DATA   │        8 │ Log        │            │",
				"│      3 │ MACHBASEDB    │ SYS       │ TAG_DATA   │        7 │ Tag        │            │",
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

func TestShellSql(t *testing.T) {
	tests := []ShellTestCase{
		{
			name: "sql_invalid_query",
			args: append(shellArgs, "sql", "SELECT * FROM non_existent_table"),
			expect: []string{
				"Error:  MACHCLI-ERR-2025, Table NON_EXISTENT_TABLE does not exist.",
			},
		},
		{
			name: "sql_valid_query",
			args: append(shellArgs, "sql", "SELECT 1 AS COL1, 'test' AS COL2"),
			expect: []string{
				"┌────────┬──────┬──────┐",
				"│ ROWNUM │ COL1 │ COL2 │",
				"├────────┼──────┼──────┤",
				"│      1 │    1 │ test │",
				"└────────┴──────┴──────┘",
				"a row selected.",
			},
		},
		{
			name: "sql_crud_insert",
			args: append(shellArgs, "sql", "INSERT INTO example VALUES('my-crd', to_date('2023-08-03'), 1.2345)"),
			expect: []string{
				"a row inserted.",
			},
		},
		{
			name: "sql_crud_flush",
			args: append(shellArgs, "sql", "EXEC table_flush(example)"),
			expect: []string{
				"rollup executed.", // TODO: check this message, it should be "executed." instead of "rollup executed." since it's called from SQL, not CLI
			},
		},
		{
			name: "sql_crud_select",
			args: append(shellArgs, "sql", "SELECT * FROM example WHERE name='my-crd'"),
			expect: []string{
				"┌────────┬────────┬─────────────────────┬────────┐",
				"│ ROWNUM │ NAME   │ TIME                │  VALUE │",
				"├────────┼────────┼─────────────────────┼────────┤",
				"│      1 │ my-crd │ 2023-08-03 00:00:00 │ 1.2345 │",
				"└────────┴────────┴─────────────────────┴────────┘",
				"a row selected.",
			},
		},
		{
			name: "sql_crud_select",
			args: append(shellArgs, "SELECT time, value FROM example WHERE name='my-crd'"),
			expect: []string{
				"┌────────┬─────────────────────┬────────┐",
				"│ ROWNUM │ TIME                │  VALUE │",
				"├────────┼─────────────────────┼────────┤",
				"│      1 │ 2023-08-03 00:00:00 │ 1.2345 │",
				"└────────┴─────────────────────┴────────┘",
				"a row selected.",
			},
		},
		{
			name: "sql_crud_delete",
			args: append(shellArgs, "sql", "DELETE FROM example WHERE name='my-crd'"),
			expect: []string{
				"a row deleted.",
				"",
			},
		},
		{
			name: "sql_crud_flush_after_delete",
			args: append(shellArgs, "EXEC table_flush(example)"),
			expect: []string{
				// TODO: check this message, it should be "executed."
				// instead of "rollup executed." since it's called from SQL, not CLI
				// this message is built in neo-client/machgo/machgo.go
				"rollup executed.",
			},
		},
		{
			name: "sql_crud_select_after_delete",
			args: append(shellArgs, "SELECT count(*) FROM example WHERE name='my-crd'"),
			expect: []string{
				"┌────────┬──────────┐",
				"│ ROWNUM │ COUNT(*) │",
				"├────────┼──────────┤",
				"│      1 │        0 │",
				"└────────┴──────────┘",
				"a row selected.",
			},
		},
	}
	for _, tt := range tests {
		runShellTestCase(t, tt)
	}
}
