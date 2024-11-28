package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

var brokerAddr = ""
var testTimeTick = time.Unix(1705291859, 0)
var mqttServer *mqttd

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
	database := testServer.DatabaseSVR()
	initTestData(database)

	// tql
	fileDirs := []string{"/=./test"}
	serverFs, _ := ssfs.NewServerSideFileSystem(fileDirs)
	ssfs.SetDefault(serverFs)
	tqlLoader := tql.NewLoader()

	// mqtt broker
	opts := []MqttOption{
		WithMqttTcpListener("127.0.0.1:0", nil),
		WithMqttTqlLoader(tqlLoader),
	}
	if svr, err := NewMqtt(database, opts...); err != nil {
		panic(err)
	} else {
		mqttServer = svr
	}
	mqttServer.db = database

	if err := mqttServer.Start(); err != nil {
		panic(err)
	}

	if addr, ok := mqttServer.broker.Listeners.Get("mqtt-tcp-0"); !ok {
		panic("Listener not found")
	} else {
		brokerAddr = strings.TrimPrefix(addr.Address(), "tcp://")
	}

	// run tests
	m.Run()

	// cleanup
	mqttServer.Stop()
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
	for _, row := range rows {
		result = conn.Exec(ctx, `INSERT INTO example VALUES (?, ?, ?)`, row[0], row[1], row[2])
		if result.Err() != nil {
			panic(result.Err())
		}
	}
	conn.Exec(ctx, `EXEC table_flush(example)`)
}
