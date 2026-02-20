package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

var testTimeTick = time.Unix(1705291859, 0)

var machServerAddress = ""

var mqttServer *mqttd
var mqttServerAddress = ""

var httpServer *httpd
var httpServerAddress = ""

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
	initTestData(database)

	machServerAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testServer.MachPort())

	// metric
	api.StartMetrics()
	// append worker
	api.StartAppendWorkers()

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

	// run tests
	m.Run()

	// cleanup
	mqttServer.Stop()
	httpServer.Stop()
	api.StopMetrics()
	testServer.DropTestTables()
	testServer.StopServer()
}

func initTestData(db api.Database) {
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

func TestShell(t *testing.T) {
	var projRoot = filepath.FromSlash("../../")
	var binPath = filepath.Join(projRoot, "tmp", "neo-shell")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	buildShellCmd := []string{
		"go", "build", "-o", binPath, filepath.Join(projRoot, "shell"),
	}
	err := exec.Command(buildShellCmd[0], buildShellCmd[1:]...).Run()
	require.NoError(t, err, "Failed to build shell binary")

	var binArgs = []string{
		binPath,
		"--server", httpServerAddress,
		"--user", "sys",
		"--password", "manager",
	}
	tests := []ShellTestCase{
		{
			name: "bridge_list",
			args: append(binArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
		{
			name: "bridge_add",
			args: append(binArgs, "bridge", "add", "test-bridge", "--type", "sqlite", "file::memory:?cache=shared"),
			expect: []string{
				"Adding bridge... test-bridge type: sqlite path: file::memory:?cache=shared",
			},
		},
		{
			name: "bridge_list_after_add",
			args: append(binArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬─────────────┬────────┬────────────────────────────┐",
				"│ ROWNUM │ NAME        │ TYPE   │ CONNECTION                 │",
				"├────────┼─────────────┼────────┼────────────────────────────┤",
				"│      1 │ test-bridge │ sqlite │ file::memory:?cache=shared │",
				"└────────┴─────────────┴────────┴────────────────────────────┘",
			},
		},
		{
			name: "bridge_test",
			args: append(binArgs, "bridge", "test", "test-bridge"),
			expect: []string{
				"Testing bridge... test-bridge",
				"OK.",
			},
		},
		{
			name: "bridge_del",
			args: append(binArgs, "bridge", "del", "test-bridge"),
			expect: []string{
				"Deleted.",
			},
		},
		{
			name: "bridge_list_after_del",
			args: append(binArgs, "bridge", "list"),
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},

		{
			name: "key_list",
			args: append(binArgs, "key", "list"),
			expect: []string{
				"┌────────┬────┬──────────────────┬─────────────────┐",
				"│ ROWNUM │ ID │ NOT VALID BEFORE │ NOT VALID AFTER │",
				"├────────┼────┼──────────────────┼─────────────────┤",
				"└────────┴────┴──────────────────┴─────────────────┘",
			},
		},
		{
			name: "show_license",
			args: append(binArgs, "show", "license", "--box-style", "simple"),
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
			args: append(binArgs, "show", "tables", "--format", "csv"),
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
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(tt.args[0], tt.args[1:]...)
			output, err := cmd.CombinedOutput()
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
					require.Equal(t, expect, outputLine)
				}
			}
		})
	}
}

type ShellTestCase struct {
	name   string
	args   []string
	expect []string
}
