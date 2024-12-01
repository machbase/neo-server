package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

var testTimeTick = time.Unix(1705291859, 0)

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

	// database
	testServer := testsuite.NewServer("./testsuite_tmp")
	testServer.StartServer(m)
	testServer.CreateTestTables()
	database := testServer.DatabaseSVR()
	initTestData(database)

	// tql
	fileDirs := []string{"/=./test"}
	serverFs, _ := ssfs.NewServerSideFileSystem(fileDirs)
	ssfs.SetDefault(serverFs)
	tqlLoader := tql.NewLoader()

	// http server
	httpOpts := []HttpOption{
		WithHttpListenAddress("tcp://127.0.0.1:0"),
		WithHttpTqlLoader(tqlLoader),
		WithHttpServerInfoFunc(func() (*mgmt.ServerInfoResponse, error) {
			return &mgmt.ServerInfoResponse{
				Success: true,
				Reason:  "success",
				Elapse:  "0ms",
				Version: &mgmt.Version{},
				Runtime: &mgmt.Runtime{},
			}, nil
		}),
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
		WithMqttTqlLoader(tqlLoader),
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
	testServer.DropTestTables()
	testServer.StopServer(m)
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
	)`)
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

func Example_representativePort() {
	fmt.Println(representativePort("tcp://127.0.0.1:1234"))
	fmt.Println(representativePort("http://192.168.1.100:1234"))
	fmt.Println(representativePort("unix:///var/run/neo-server.sock"))
	// Output:
	//   > Local:   http://127.0.0.1:1234
	//   > Network: http://192.168.1.100:1234
	//   > Unix:    /var/run/neo-server.sock
}
